// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bitfield/script"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
	"google.golang.org/grpc/status"

	onboarding_manager "github.com/open-edge-platform/edge-manageability-framework/e2e-tests/orchestrator/onboarding_manager"
	"github.com/open-edge-platform/edge-manageability-framework/internal/retry"
	util "github.com/open-edge-platform/edge-manageability-framework/mage"
	pb_am "github.com/open-edge-platform/infra-managers/attestationstatus/pkg/api/attestmgr/v1"
	pb_hm "github.com/open-edge-platform/infra-managers/host/pkg/api/hostmgr/proto"
	pb_mm "github.com/open-edge-platform/infra-managers/maintenance/pkg/api/maintmgr/v1"
	pb_tm "github.com/open-edge-platform/infra-managers/telemetry/pkg/api/telemetrymgr/v1"
	pb_om "github.com/open-edge-platform/infra-onboarding/onboarding-manager/pkg/api/onboardingmgr/v1"
)

var (
	testEnUser     = util.TestUser + "-en-svc-account"
	testOnbUser    = util.TestUser + "-onboarding-user"
	testApiUser    = util.TestUser + "-api-user"
	baseProjAPIUrl = fmt.Sprintf(apiBaseURLTemplate, serviceDomain, util.TestProject)
)

var _ = Describe("Edge Infrastructure Manager integration test", Label("orchestrator-integration"), func() {
	var cli *http.Client

	testUserPassword := func() string {
		pass, err := util.GetDefaultOrchPassword()
		if err != nil {
			log.Fatal(err)
		}
		return pass
	}()

	BeforeEach(func() {
		cli = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		}

		fmt.Printf("serviceDomain: %v\n", serviceDomain)
	})

	Describe("Onboarding Manager service", Label(infraManagement), func() {
		It("should be able to trigger workflow over HTTPS when using valid JWT token", func(ctx SpecContext) {
			var (
				hostGrpcUrl = fmt.Sprintf("onboarding-node.%s:%d", serviceDomain, servicePort)
				hostUrl     = baseProjAPIUrl + "/compute/hosts"
				instanceUrl = baseProjAPIUrl + "/compute/instances"
				err         error
				mac         = onboarding_manager.Mac()
				hostUuid    = uuid.NewString()
			)

			apiToken, err := getTestInfraApiToken(cli, testUserPassword)
			Expect(err).ToNot(HaveOccurred())

			onbToken, err := getTestInfraOnbToken(cli, testUserPassword)
			Expect(err).ToNot(HaveOccurred())

			// Create Host
			err = onboarding_manager.GrpcInfraOnboardNewNode(hostGrpcUrl, *onbToken, mac, hostUuid)
			Expect(err).ToNot(HaveOccurred(), "cannot create host")

			// Get Host and Instance
			_, _, err = onboarding_manager.HttpInfraOnboardGetHostAndInstance(ctx, hostUrl, *apiToken, cli, hostUuid)
			Expect(err).ToNot(HaveOccurred(), "cannot get host and instance")

			// Check if Workflow is created
			k8scli, err := onboarding_manager.NewK8SClient()
			Expect(err).ToNot(HaveOccurred(), "cannot create k8s client")
			ns := "orch-infra"
			err = onboarding_manager.CheckWorkflowCreationInfraOnboard(ns, hostUuid, k8scli)
			Expect(err).ToNot(HaveOccurred(), "cannot check workflow creation")

			Expect(cleanupHost(context.Background(), hostUrl, instanceUrl, *apiToken, cli, hostUuid)).To(Succeed())
		})
	})

	Describe("Onboarding Manager service using NIO stream", Ordered, Label(infraManagement), func() {
		It("should be able to onboard an EN when using onboarding-stream without JWT Token", func(ctx SpecContext) {
			var (
				hostGrpcUrl = fmt.Sprintf("onboarding-stream.%s:%d", serviceDomain, servicePort)
				hostRegUrl  = baseProjAPIUrl + "/compute/hosts/register"
				hostUrl     = baseProjAPIUrl + "/compute/hosts"
				instanceUrl = baseProjAPIUrl + "/compute/instances"
				err         error
				mac         = onboarding_manager.Mac()
				hostUuid    = uuid.New()
			)

			apiToken, err := getTestInfraApiToken(cli, testUserPassword)
			Expect(err).ToNot(HaveOccurred())

			// Register Host
			registeredHost, err := onboarding_manager.HttpInfraOnboardNewRegisterHost(hostRegUrl, *apiToken, cli, hostUuid)
			Expect(err).ToNot(HaveOccurred())
			fmt.Printf("Registered Host: %+v\n", registeredHost)

			// Verify registration
			nodeState, err := onboarding_manager.GrpcInfraOnboardStreamNode(hostGrpcUrl, mac, hostUuid.String(), "")
			Expect(err).ToNot(HaveOccurred())
			Expect(nodeState).To(Equal(pb_om.OnboardNodeStreamResponse_NODE_STATE_REGISTERED))

			// Call the /onboard endpoint to set the state to onboarded
			onboardUrl := baseProjAPIUrl + fmt.Sprintf("/compute/hosts/%s/onboard", *registeredHost.ResourceId)
			req, err := http.NewRequestWithContext(context.Background(), http.MethodPatch, onboardUrl, nil)
			Expect(err).ToNot(HaveOccurred())
			req.Header.Set("Authorization", "Bearer "+*apiToken)
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			// Confirm Host
			nodeState, err = onboarding_manager.GrpcInfraOnboardStreamNode(hostGrpcUrl, mac, hostUuid.String(), "")
			Expect(err).ToNot(HaveOccurred())
			Expect(nodeState).To(Equal(pb_om.OnboardNodeStreamResponse_NODE_STATE_ONBOARDED))

			// Wait for host and instance reconciliation
			reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			err = retry.UntilItSucceeds(
				reqCtx,
				func() error {
					hostID, instanceID, err := onboarding_manager.HttpInfraOnboardGetHostAndInstance(reqCtx, hostUrl, *apiToken, cli, hostUuid.String())
					if err != nil {
						if strings.Contains(err.Error(), "empty host result for uuid") || strings.Contains(err.Error(), "instance not yet created for uuid") {
							// Continue retrying if the host result is empty or instance is not yet created
							return fmt.Errorf("waiting for instance creation")
						}
						// Stop retrying if there is another error
						return err
					}
					if hostID == "" || instanceID == "" {
						// Continue retrying if the host or instance does not exist
						return fmt.Errorf("waiting for instance creation")
					}
					// Stop retrying if the host and instance exist
					return nil
				},
				5*time.Second,
			)
			Expect(err).ToNot(HaveOccurred())

			// Cleanup after test
			Expect(cleanupHost(context.Background(), hostUrl, instanceUrl, *apiToken, cli, hostUuid.String())).To(Succeed())
		})

		It("should fail to onboard an EN when using onboarding-stream with invalid UUID", func(ctx SpecContext) {
			var (
				hostGrpcUrl = fmt.Sprintf("onboarding-stream.%s:%d", serviceDomain, servicePort)
				hostRegUrl  = baseProjAPIUrl + "/compute/hosts/register"
				hostUrl     = baseProjAPIUrl + "/compute/hosts"
				err         error
				mac         = onboarding_manager.Mac()
				hostUuid    = uuid.New()
				hostID      string
			)

			apiToken, err := getTestInfraApiToken(cli, testUserPassword)
			Expect(err).ToNot(HaveOccurred())

			// Register Host
			registeredHost, err := onboarding_manager.HttpInfraOnboardNewRegisterHost(hostRegUrl, *apiToken, cli, hostUuid)
			Expect(err).ToNot(HaveOccurred())
			fmt.Printf("Registered Host: %+v\n", registeredHost)

			// Retrieve the host ID
			hostID, err = onboarding_manager.HttpInfraOnboardGetNode(ctx, hostUrl, *apiToken, cli, hostUuid.String())
			Expect(err).ToNot(HaveOccurred())

			// Verify registration with invalid UUID
			invalidHostUuid := "invalid-host-id"
			nodeState, err := onboarding_manager.GrpcInfraOnboardStreamNode(hostGrpcUrl, mac, invalidHostUuid, "")
			Expect(err).To(HaveOccurred(), "Expected error when using invalid UUID")
			Expect(nodeState).ToNot(Equal(pb_om.OnboardNodeStreamResponse_NODE_STATE_REGISTERED), "Expected node state to not be REGISTERED with invalid UUID")

			// Cleanup after test
			Expect(onboarding_manager.HttpInfraOnboardDelResource(context.Background(), hostUrl, *apiToken, cli, hostID)).To(Succeed())
		})
	})

	Describe("Edge Infra services", Ordered, Label(infraManagement), func() {
		testUrl := baseProjAPIUrl + "/compute"
		It("Edge Infrastructure services should NOT be accessible over HTTPS when using valid but expired token", func() { //nolint: dupl, lll
			Expect(saveTokenUser(cli, testApiUser, testUserPassword)).To(Succeed())

			jwt, err := script.File(outputFile).String()
			Expect(err).ToNot(HaveOccurred())
			Expect(jwt).ToNot(BeEmpty())

			isUnexpired, err := isTokenUnexpired(jwt)
			Expect(err).ToNot(HaveOccurred())
			if isUnexpired {
				Skip("Skipping this test because JWT Token is NOT expired")
			}

			request, err := http.NewRequest("GET", testUrl, nil)
			Expect(err).ToNot(HaveOccurred())

			// adding JWT to the Authorization header
			request.Header.Add("Authorization", "Bearer "+jwt)

			response, err := cli.Do(request)
			Expect(err).ToNot(HaveOccurred())
			defer response.Body.Close()

			Expect(response.StatusCode).To(Equal(http.StatusForbidden))
		})
		It("should be accessible over HTTPS when using valid token", func() {
			req, err := http.NewRequest("GET", testUrl, nil)
			Expect(err).ToNot(HaveOccurred())
			apiToken, err := getTestInfraApiToken(cli, testUserPassword)
			Expect(err).ToNot(HaveOccurred())
			req.Header.Add("Authorization", "Bearer "+*apiToken)
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			_, err = io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
		})
		It("should NOT be accessible over HTTPS when using no token", func() {
			req, err := http.NewRequest("GET", testUrl, nil)
			Expect(err).ToNot(HaveOccurred())
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
		})
		It("should NOT be accessible over HTTPS when using invalid token", func() {
			req, err := http.NewRequest("GET", testUrl, nil)
			Expect(err).ToNot(HaveOccurred())
			const invalid = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c" //nolint: lll
			req.Header.Add("Authorization", "Bearer "+invalid)
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
		})
	})

	Describe("Host Manager gRPC service using jwt", Ordered, Label(infraManagement), func() { //nolint: dupl
		onbSBIUrl := "onboarding-node." + serviceDomain
		hrmSBIUrl := "infra-node." + serviceDomain
		hostUuid := uuid.New().String()

		It("should be accessible over gRPC when uses a valid keycloak token", func(ctx SpecContext) {
			enToken, err := getTestENToken(cli, testUserPassword)
			Expect(err).ToNot(HaveOccurred())
			apiToken, err := getTestInfraApiToken(cli, testUserPassword)
			Expect(err).ToNot(HaveOccurred())
			hostUrl := baseProjAPIUrl + "/compute/hosts"
			instanceUrl := baseProjAPIUrl + "/compute/instances"

			reqCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()

			// Invoke API using jwt token
			Expect(grpcInfraOnboardNodeJWT(reqCtx, onbSBIUrl, servicePort, *enToken, hostUuid)).To(Succeed())
			// Invoke API using jwt token
			Expect(grpcInfraHostMgrJWT(reqCtx, hrmSBIUrl, servicePort, *enToken, hostUuid)).To(Succeed())

			// Housekeeping
			Expect(cleanupHost(ctx, hostUrl, instanceUrl, *apiToken, cli, hostUuid)).To(Succeed())
		})
		It("should NOT be accessible over gRPC when using non-EN token", func(ctx SpecContext) {
			sbiWithAPITokenExpectError(ctx, cli, grpcInfraHostMgrJWT, hrmSBIUrl, servicePort, testUserPassword)
		})
		It("should NOT be accessible over gRPC when uses no keycloak token", func(ctx SpecContext) {
			sbiWithNoTokenExpectError(ctx, grpcInfraHostMgrJWT, hrmSBIUrl, servicePort)
		})
		It("should NOT be accessible over gRPC when uses invalid token", func(ctx SpecContext) {
			sbiWithInvalidTokenExpectError(ctx, grpcInfraHostMgrJWT, hrmSBIUrl, servicePort)
		})
		It("should NOT be accessible over gRPC when using valid but expired token", func(ctx SpecContext) {
			sbiWithExpiredTokenExpectError(ctx, cli, grpcInfraHostMgrJWT, hrmSBIUrl, servicePort)
		})
	})

	Describe("Maintenance Manager gRPC service using jwt", Ordered, Label(infraManagement), func() {
		mmSBIUrl := "update-node." + serviceDomain

		It("should be accessible over gRPC when uses a valid keycloak token", func(ctx SpecContext) {
			enToken, err := getTestENToken(cli, testUserPassword)
			Expect(err).ToNot(HaveOccurred())

			reqCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()

			// Invoke API using jwt token
			Expect(grpcInfraMainMgrJWT(reqCtx, mmSBIUrl, servicePort, *enToken)).Should(
				MatchError(ContainSubstring(
					"could not call grpc endpoint for server update-node.%s:%d: rpc error: code = NotFound",
					serviceDomain, servicePort)),
			)
		})
		It("should NOT be accessible over gRPC when using non-EN token", func(ctx SpecContext) {
			sbiWithAPITokenExpectError(ctx, cli, grpcInfraMainMgrJWT, mmSBIUrl, servicePort, testUserPassword)
		})
		It("should NOT be accessible over gRPC when uses no keycloak token", func(ctx SpecContext) {
			sbiWithNoTokenExpectError(ctx, grpcInfraMainMgrJWT, mmSBIUrl, servicePort)
		})
		It("should NOT be accessible over gRPC when uses invalid token", func(ctx SpecContext) {
			sbiWithInvalidTokenExpectError(ctx, grpcInfraMainMgrJWT, mmSBIUrl, servicePort)
		})
		It("should NOT be accessible over gRPC when using valid but expired token", func(ctx SpecContext) {
			sbiWithExpiredTokenExpectError(ctx, cli, grpcInfraMainMgrJWT, mmSBIUrl, servicePort)
		})
	})

	Describe("Telemetry Manager service", Ordered, Label(infraManagement), func() {
		tmSBIUrl := "telemetry-node." + serviceDomain

		It("should be accessible over HTTPS when using valid token", func(ctx SpecContext) {
			enToken, err := getTestENToken(cli, testUserPassword)
			Expect(err).ToNot(HaveOccurred())

			reqCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()

			err = grpcInfraTelemetryMgrJWT(
				reqCtx,
				tmSBIUrl,
				servicePort,
				*enToken,
			)
			Expect(status.Code(err)).To(Equal(codes.NotFound)) // Getting a gRPC response back is good enough
		})

		It("should NOT be accessible over gRPC when using non-EN token", func(ctx SpecContext) {
			sbiWithAPITokenExpectError(ctx, cli, grpcInfraTelemetryMgrJWT, tmSBIUrl, servicePort, testUserPassword)
		})

		It("should NOT be accessible over HTTPS when using no token", func(ctx SpecContext) {
			sbiWithNoTokenExpectError(ctx, grpcInfraTelemetryMgrJWT, tmSBIUrl, servicePort)
		})

		It("should NOT be accessible over HTTPS when using invalid token", func(ctx SpecContext) {
			sbiWithInvalidTokenExpectError(ctx, grpcInfraTelemetryMgrJWT, tmSBIUrl, servicePort)
		})

		It("should NOT be accessible over HTTPS when using valid but expired token", func(ctx SpecContext) { //nolint: dupl
			sbiWithExpiredTokenExpectError(ctx, cli, grpcInfraTelemetryMgrJWT, tmSBIUrl, servicePort)
		})
	})

	Describe("Attestation Status Manager service", Ordered, Label(infraManagement), func() {
		amSBIUrl := "attest-node." + serviceDomain

		It("should be accessible over HTTPS when using valid token", func(ctx SpecContext) {
			enToken, err := getTestENToken(cli, testUserPassword)
			Expect(err).ToNot(HaveOccurred())

			reqCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()

			err = grpcAttestStatusMgrJWT(
				reqCtx,
				amSBIUrl,
				servicePort,
				*enToken,
			)
			Expect(status.Code(err)).To(Equal(codes.NotFound)) // Getting a gRPC response back is good enough
		})

		It("should NOT be accessible over gRPC when using non-EN token", func(ctx SpecContext) {
			sbiWithAPITokenExpectError(ctx, cli, grpcAttestStatusMgrJWT, amSBIUrl, servicePort, testUserPassword)
		})

		It("should NOT be accessible over HTTPS when using no token", func(ctx SpecContext) {
			sbiWithNoTokenExpectError(ctx, grpcAttestStatusMgrJWT, amSBIUrl, servicePort)
		})

		It("should NOT be accessible over HTTPS when using invalid token", func(ctx SpecContext) {
			sbiWithInvalidTokenExpectError(ctx, grpcAttestStatusMgrJWT, amSBIUrl, servicePort)
		})

		It("should NOT be accessible over HTTPS when using valid but expired token", func(ctx SpecContext) { //nolint: dupl
			sbiWithExpiredTokenExpectError(ctx, cli, grpcAttestStatusMgrJWT, amSBIUrl, servicePort)
		})
	})

	Describe("Tinkerbell CDN-NGINX service", Ordered, Label(infraManagement), func() {
		var cdnURL string
		var bootsCli *http.Client
		var bootsTLSConfig *tls.Config
		var caCert *x509.Certificate
		var cafilepath string
		BeforeEach(func(ctx SpecContext) {
			By("fetching the CA certificate")
			Eventually(
				func() error {
					cdnURL = "https://tinkerbell-nginx." + serviceDomain + "/tink-stack/boot.ipxe"

					req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+serviceDomain+"/boots/ca.crt", nil) //nolint: lll
					if err != nil {
						return err
					}

					resp, err := cli.Do(req)
					if err != nil {
						return err
					}
					defer resp.Body.Close()

					caPEM, err := io.ReadAll(resp.Body)
					if err != nil {
						return err
					}

					caBlock, _ := pem.Decode(caPEM)
					if caBlock == nil {
						return fmt.Errorf("no CA certificate")
					}

					if caCert, err = x509.ParseCertificate(caBlock.Bytes); err != nil {
						return err
					}
					pool := x509.NewCertPool()
					pool.AddCert(caCert)

					bootsTLSConfig = &tls.Config{ //nolint: gosec
						RootCAs:    pool,
						MinVersion: tls.VersionTLS12,
						MaxVersion: tls.VersionTLS12,
					}
					bootsCli = &http.Client{
						Transport: &http.Transport{
							TLSClientConfig: bootsTLSConfig,
						},
					}

					return nil
				},
				time.Minute,
				5*time.Second,
			).Should(Succeed())
			// cdnURL = "https://tinkerbell-nginx." + serviceDomain
			cafilepath = "/tmp/cluster_ca.crt"
			cacert := "curl https://" + serviceDomain + "/boots/ca.crt -o " + cafilepath
			_, err := script.NewPipe().Exec(cacert).String()
			Expect(err).ToNot(HaveOccurred())
		})

		It("should be accessible over HTTPS with cipher TLS_DHE_RSA_WITH_AES_256_GCM_SHA384", func() {
			// curl is needed for cipher DHE-RSA-AES256-GCM-SHA384 as Go wont supported the required cipher
			cmd := "curl --cacert " + cafilepath + " -s --noproxy \"*\" --tlsv1.2 --tls-max 1.2 --ciphers DHE-RSA-AES256-GCM-SHA384 " +
				"-o /dev/null -w \"%{http_code}\" " + strconv.Quote(cdnURL)
			req, err := script.NewPipe().Exec(cmd).String()
			Expect(err).ToNot(HaveOccurred())
			Expect(req).To(Equal("200"))
		})

		It("should NOT be accessible over HTTPS with cipher ECDHE-RSA-AES128-GCM-SHA256", func() {
			// curl is needed for cipher DHE-RSA-AES256-GCM-SHA384 as Go wont supported the required cipher
			cmd := "curl --cacert " + cafilepath + " -s --noproxy \"*\" --tlsv1.2 --tls-max 1.2 --ciphers ECDHE-RSA-AES128-GCM-SHA256" +
				" -o /dev/null -v " + strconv.Quote(cdnURL)
			req, _ := script.NewPipe().Exec(cmd).String()
			Expect(req).To(ContainSubstring("TLS alert, handshake failure"))
		})

		It("should serve the CA certificate of the Tinkerbell Boots TLS server certificate", func(ctx SpecContext) {
			By("verifying that the CA certificate issued the Tinkerbell Boots TLS server certificate")
			req, err := http.NewRequestWithContext(
				ctx,
				http.MethodGet,
				"https://tinkerbell-nginx."+serviceDomain,
				nil,
			)
			Expect(err).ToNot(HaveOccurred())

			resp, err := bootsCli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			// Checking equivalency to check that the certificate is synced across the namespaces
			Expect(resp.TLS.PeerCertificates[0].Equal(caCert)).To(BeTrue())
		})

		It("should be accessible over HTTPS", func(ctx SpecContext) {
			req, err := http.NewRequestWithContext(
				ctx,
				http.MethodGet,
				cdnURL,
				nil,
			)
			Expect(err).ToNot(HaveOccurred())

			resp, err := bootsCli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			content, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("ipxe"))
		})

		It("should have a body that contains ipxe when using cipher TLS_DHE_RSA_WITH_AES_256_GCM_SHA384", func() {
			// curl is needed for cipher DHE-RSA-AES256-GCM-SHA384 as Go wont supported the required cipher
			cmd := "curl --cacert " + cafilepath + " -s --noproxy \"*\" --tlsv1.2 --tls-max 1.2 --ciphers DHE-RSA-AES256-GCM-SHA384 " + cdnURL
			fmt.Println(cmd)
			req, err := script.NewPipe().Exec(cmd).String()
			Expect(err).ToNot(HaveOccurred())
			Expect(req).To(ContainSubstring("ipxe"))
		})

		It("should be accessible over HTTPS with one of approved cipher suites", func(ctx SpecContext) {
			bootsTLSConfig.CipherSuites = []uint16{
				tls.TLS_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			}
			req, err := http.NewRequestWithContext(
				ctx,
				http.MethodGet,
				cdnURL,
				nil,
			)
			Expect(err).ToNot(HaveOccurred())

			resp, err := bootsCli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			content, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("ipxe"))
		})

		It("should NOT be accessible over HTTPS with cipher TLS_RSA_WITH_AES_128_GCM_SHA256", func(ctx SpecContext) {
			bootsTLSConfig.CipherSuites = []uint16{
				tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
			}
			req, err := http.NewRequestWithContext(
				ctx,
				http.MethodGet,
				cdnURL,
				nil,
			)
			Expect(err).ToNot(HaveOccurred())

			_, err = bootsCli.Do(req)
			Expect(err.Error()).To(ContainSubstring("tls: handshake failure"))
		})
	})

	Describe("Tinkerbell GRPC service using jwt", Label(infraManagement), func() {
		serverURL := "tinkerbell-server." + serviceDomainWithPort

		It("should be accessible over gRPC with a valid keycloak token", func(ctx SpecContext) {
			enToken, err := getTestENToken(cli, testUserPassword)
			Expect(err).ToNot(HaveOccurred())

			reqCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()

			conn, err := grpc.DialContext(
				reqCtx,
				serverURL,
				grpc.WithBlock(),
				grpc.WithTransportCredentials(
					credentials.NewClientTLSFromCert(nil, ""), // Use host's root CA set to establish trust
				),
				grpc.WithPerRPCCredentials(
					oauth.TokenSource{
						TokenSource: oauth2.StaticTokenSource(
							&oauth2.Token{AccessToken: *enToken}, // Send the access token as part of the HTTP Authorization header
						),
					},
				),
			)
			Expect(err).ToNot(HaveOccurred())
			defer conn.Close()
		})

		It("should NOT be accessible over gRPC when uses no keycloak token", func(ctx SpecContext) {
			reqCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()

			// Invoke API without providing jwt token
			conn, err := grpc.DialContext(
				reqCtx,
				serverURL,
				grpc.WithBlock(),
				grpc.WithTransportCredentials(
					credentials.NewClientTLSFromCert(nil, ""), // Use host's root CA set to establish trust
				),
				grpc.WithPerRPCCredentials(
					oauth.TokenSource{
						TokenSource: oauth2.StaticTokenSource(
							&oauth2.Token{AccessToken: ""}, // Send the access token as part of the HTTP Authorization header
						),
					},
				),
			)
			Expect(err).ToNot(HaveOccurred())
			defer conn.Close()
		})

		It("should NOT be accessible over gRPC with an invalid keycloak token", func(ctx SpecContext) {
			const invalidToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c" //nolint: lll

			reqCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()

			conn, err := grpc.DialContext(
				reqCtx,
				serverURL,
				grpc.WithBlock(),
				grpc.WithTransportCredentials(
					credentials.NewClientTLSFromCert(nil, ""), // Use host's root CA set to establish trust
				),
				grpc.WithPerRPCCredentials(
					oauth.TokenSource{
						TokenSource: oauth2.StaticTokenSource(
							&oauth2.Token{AccessToken: invalidToken}, // Send the access token as part of the HTTP Authorization header
						),
					},
				),
			)
			Expect(err).ToNot(HaveOccurred())
			defer conn.Close()
		})

		It("should NOT be accessible over gRPC when using valid but expired token", func(ctx SpecContext) {
			Expect(saveTokenUser(cli, testEnUser, testUserPassword)).To(Succeed())
			token, err := script.File(outputFile).String()
			Expect(err).ToNot(HaveOccurred())
			Expect(token).ToNot(BeEmpty())

			isUnexpired, err := isTokenUnexpired(token)
			Expect(err).ToNot(HaveOccurred())
			if isUnexpired {
				Skip("Skipping this test because JWT Token is NOT expired")
			}

			reqCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()

			conn, err := grpc.DialContext(
				reqCtx,
				serverURL,
				grpc.WithBlock(),
				grpc.WithTransportCredentials(
					credentials.NewClientTLSFromCert(nil, ""), // Use host's root CA set to establish trust
				),
				grpc.WithPerRPCCredentials(
					oauth.TokenSource{
						TokenSource: oauth2.StaticTokenSource(
							&oauth2.Token{AccessToken: token}, // Send the access token as part of the HTTP Authorization header
						),
					},
				),
			)
			Expect(err).ToNot(HaveOccurred())
			defer conn.Close()
		})
	})

	Describe("Onboarding Manager gRPC service using jwt", Label(infraManagement), func() { //nolint: dupl
		omSBIUrl := "onboarding-node." + serviceDomain
		It("should be accessible over gRPC when uses a valid keycloak token", func(ctx SpecContext) {
			onbToken, err := getTestInfraOnbToken(cli, testUserPassword)
			Expect(err).ToNot(HaveOccurred())
			apiToken, err := getTestInfraApiToken(cli, testUserPassword)
			Expect(err).ToNot(HaveOccurred())

			hostUrl := baseProjAPIUrl + "/compute/hosts"
			instanceUrl := baseProjAPIUrl + "/compute/instances"
			hostUuid := uuid.New().String()

			reqCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()

			// Invoke API using jwt token
			Expect(grpcInfraOnboardNodeJWT(reqCtx, omSBIUrl, servicePort, *onbToken, hostUuid)).To(Succeed())

			// Housekeeping
			Expect(cleanupHost(ctx, hostUrl, instanceUrl, *apiToken, cli, hostUuid)).To(Succeed())
		})

		It("should NOT be accessible over gRPC when uses no keycloak token", func(ctx SpecContext) {
			sbiWithNoTokenExpectError(ctx, grpcInfraOnboardNodeJWT, omSBIUrl, servicePort)
		})

		It("should NOT be accessible over gRPC when uses invalid token", func(ctx SpecContext) {
			sbiWithInvalidTokenExpectError(ctx, grpcInfraOnboardNodeJWT, omSBIUrl, servicePort)
		})

		It("should NOT be accessible over gRPC when using valid but expired token", func(ctx SpecContext) {
			sbiWithExpiredTokenExpectError(ctx, cli, grpcInfraOnboardNodeJWT, omSBIUrl, servicePort)
		})
	})
})

func getTestInfraApiToken(client *http.Client, testUserPassword string) (*string, error) {
	return util.GetApiToken(client, testApiUser, testUserPassword)
}

func getTestInfraOnbToken(client *http.Client, testUserPassword string) (*string, error) {
	return util.GetApiToken(client, testOnbUser, testUserPassword)
}

func getTestENToken(client *http.Client, testUserPassword string) (*string, error) {
	return util.GetApiToken(client, testEnUser, testUserPassword)
}

// last param is needed to keep the signature of the function same as other grpcInfra* functions
func grpcInfraTelemetryMgrJWT(ctx context.Context, address string, port int, token string, _ ...string) error {
	target := fmt.Sprintf("%s:%d", address, port)

	conn, err := grpc.DialContext(
		ctx,
		target,
		grpc.WithBlock(),
		grpc.WithTransportCredentials(
			credentials.NewClientTLSFromCert(nil, ""), // Use host's root CA set to establish trust
		),
		grpc.WithPerRPCCredentials(
			oauth.TokenSource{
				TokenSource: oauth2.StaticTokenSource(
					&oauth2.Token{AccessToken: token}, // Send the access token as part of the HTTP Authorization header
				),
			},
		),
	)
	if err != nil {
		return fmt.Errorf("could not dial server %s: %w", target, err)
	}
	defer conn.Close()

	cli := pb_tm.NewTelemetryMgrClient(conn)

	if _, err := cli.GetTelemetryConfigByGUID(
		ctx,
		&pb_tm.GetTelemetryConfigByGuidRequest{
			Guid: uuid.New().String(),
		},
	); err != nil {
		return fmt.Errorf("could not call grpc endpoint for server %s: %w", target, err)
	}

	return nil
}

func grpcInfraOnboardNodeJWT(ctx context.Context, address string, port int, token string, hostUuid ...string) error {
	hUuid := uuid.NewString()
	if len(hostUuid) > 0 {
		hUuid = hostUuid[0]
	}
	target := fmt.Sprintf("%s:%d", address, port)
	conn, err := grpc.DialContext(
		ctx,
		target,
		grpc.WithBlock(),
		grpc.WithTransportCredentials(
			credentials.NewClientTLSFromCert(nil, ""), // Use host's root CA set to establish trust
		),
		grpc.WithPerRPCCredentials(
			oauth.TokenSource{
				TokenSource: oauth2.StaticTokenSource(
					&oauth2.Token{AccessToken: token}, // Send the access token as part of the HTTP Authorization header
				),
			},
		),
	)
	if err != nil {
		return fmt.Errorf("could not dial server %s: %w", target, err)
	}
	defer conn.Close()

	cli := pb_om.NewInteractiveOnboardingServiceClient(conn)
	// Create a NodeData object
	nodeData := &pb_om.NodeData{
		Hwdata: []*pb_om.HwData{
			{
				MacId:     "ab:cd:ef:12:34:56",
				SutIp:     "192.168.1.1",
				Uuid:      hUuid,
				Serialnum: "98330",
			},
		},
	}
	// Create a NodeRequest object and set the Payload field
	nodeRequest := &pb_om.CreateNodesRequest{
		Payload: []*pb_om.NodeData{nodeData},
	}
	// Call the gRPC endpoint with the NodeRequest
	if _, err := cli.CreateNodes(ctx, nodeRequest); err != nil {
		return fmt.Errorf("could not call gRPC endpoint for server %s: %w", target, err)
	}
	return nil
}

func grpcInfraHostMgrJWT(ctx context.Context, address string, port int, token string, hostUuid ...string) error {
	hUuid := uuid.NewString()
	if len(hostUuid) > 0 {
		hUuid = hostUuid[0]
	}

	target := fmt.Sprintf("%s:%d", address, port)

	conn, err := grpc.DialContext(
		ctx,
		target,
		grpc.WithBlock(),
		grpc.WithTransportCredentials(
			credentials.NewClientTLSFromCert(nil, ""), // Use host's root CA set to establish trust
		),
		grpc.WithPerRPCCredentials(
			oauth.TokenSource{
				TokenSource: oauth2.StaticTokenSource(
					&oauth2.Token{AccessToken: token}, // Send the access token as part of the HTTP Authorization header
				),
			},
		),
	)
	if err != nil {
		return fmt.Errorf("could not dial server %s: %w", target, err)
	}
	defer conn.Close()

	cli := pb_hm.NewHostmgrClient(conn)
	if _, err := cli.UpdateHostStatusByHostGuid(
		ctx,
		&pb_hm.UpdateHostStatusByHostGuidRequest{
			HostGuid: hUuid,
			HostStatus: &pb_hm.HostStatus{
				HostStatus: pb_hm.HostStatus_UNSPECIFIED,
			},
		},
	); err != nil {
		return fmt.Errorf("could not call grpc endpoint for server %s: %w", target, err)
	}
	return nil
}

// last param is needed to keep the signature of the function same as other grpcInfra* functions
func grpcInfraMainMgrJWT(ctx context.Context, address string, port int, token string, _ ...string) error {
	target := fmt.Sprintf("%s:%d", address, port)

	conn, err := grpc.DialContext(
		ctx,
		target,
		grpc.WithBlock(),
		grpc.WithTransportCredentials(
			credentials.NewClientTLSFromCert(nil, ""), // Use host's root CA set to establish trust
		),
		grpc.WithPerRPCCredentials(
			oauth.TokenSource{
				TokenSource: oauth2.StaticTokenSource(
					&oauth2.Token{AccessToken: token}, // Send the access token as part of the HTTP Authorization header
				),
			},
		),
	)
	if err != nil {
		return fmt.Errorf("could not dial server %s: %w", target, err)
	}
	defer conn.Close()

	cli := pb_mm.NewMaintmgrServiceClient(conn)
	if _, err := cli.PlatformUpdateStatus(
		ctx,
		&pb_mm.PlatformUpdateStatusRequest{
			HostGuid: uuid.New().String(),
			UpdateStatus: &pb_mm.UpdateStatus{
				StatusType: pb_mm.UpdateStatus_STATUS_TYPE_UP_TO_DATE,
			},
		},
	); err != nil {
		return fmt.Errorf("could not call grpc endpoint for server %s: %w", target, err)
	}
	return nil
}

// last param is needed to keep the signature of the function same as other grpcInfra* functions
func grpcAttestStatusMgrJWT(ctx context.Context, address string, port int, token string, _ ...string) error {
	target := fmt.Sprintf("%s:%d", address, port)

	conn, err := grpc.DialContext(
		ctx,
		target,
		grpc.WithBlock(),
		grpc.WithTransportCredentials(
			credentials.NewClientTLSFromCert(nil, ""), // Use host's root CA set to establish trust
		),
		grpc.WithPerRPCCredentials(
			oauth.TokenSource{
				TokenSource: oauth2.StaticTokenSource(
					&oauth2.Token{AccessToken: token}, // Send the access token as part of the HTTP Authorization header
				),
			},
		),
	)
	if err != nil {
		return fmt.Errorf("could not dial server %s: %w", target, err)
	}
	defer conn.Close()

	cli := pb_am.NewAttestationStatusMgrServiceClient(conn)
	if _, err := cli.UpdateInstanceAttestationStatusByHostGuid(
		ctx,
		&pb_am.UpdateInstanceAttestationStatusByHostGuidRequest{
			HostGuid:          uuid.New().String(),
			AttestationStatus: pb_am.AttestationStatus_ATTESTATION_STATUS_UNSPECIFIED,
		},
	); err != nil {
		return fmt.Errorf("could not call grpc endpoint for server %s: %w", target, err)
	}
	return nil
}

func sbiWithAPITokenExpectError(
	ctx context.Context, cli *http.Client,
	sbiCall func(context.Context, string, int, string, ...string) error,
	url string, port int, testUserPassword string,
) {
	apiToken, err := getTestInfraApiToken(cli, testUserPassword) //nolint:contextcheck // false positive
	Expect(err).ToNot(HaveOccurred())
	sbiCallExpectErrorMsg(ctx, sbiCall, url, port, *apiToken, "Rejected because missing projectID in JWT roles")
}

func sbiWithInvalidTokenExpectError(
	ctx context.Context,
	sbiCall func(context.Context, string, int, string, ...string) error,
	url string, port int,
) {
	const invalid = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c" //nolint: lll
	sbiCallExpectErrorMsg(ctx, sbiCall, url, port, invalid, "unexpected HTTP status code received from server: 403")
}

func sbiWithExpiredTokenExpectError(
	ctx context.Context, cli *http.Client,
	sbiCall func(context.Context, string, int, string, ...string) error,
	url string, port int,
) {
	//nolint:contextcheck
	Expect(saveToken(cli)).To(Succeed())
	jwt, err := script.File(outputFile).String()
	Expect(err).ToNot(HaveOccurred())
	Expect(jwt).ToNot(BeEmpty())

	isUnexpired, err := isTokenUnexpired(jwt)
	Expect(err).ToNot(HaveOccurred())
	if isUnexpired {
		Skip("Skipping this test because JWT Token is NOT expired")
	}
	sbiCallExpectErrorMsg(ctx, sbiCall, url, port, jwt, "unexpected HTTP status code received from server: 403")
}

func sbiWithNoTokenExpectError(
	ctx context.Context,
	sbiCall func(context.Context, string, int, string, ...string) error,
	url string, port int,
) {
	sbiCallExpectErrorMsg(ctx, sbiCall, url, port, "", "unexpected HTTP status code received from server: 403")
}

func sbiCallExpectErrorMsg(
	ctx context.Context,
	sbiCall func(context.Context, string, int, string, ...string) error,
	url string, port int, token, expErrMsg string,
) {
	reqCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	err := sbiCall(reqCtx, url, port, token)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring(expErrMsg))
}

// cleanupHost is used to remove hosts that are created during the test.
func cleanupHost(ctx context.Context, hostUrl, instanceUrl, apiToken string, cli *http.Client, hostUUID string) error {
	hostID, instanceID, err := onboarding_manager.HttpInfraOnboardGetHostAndInstance(ctx, hostUrl, apiToken, cli, hostUUID)
	if err != nil {
		return err
	}
	err = onboarding_manager.HttpInfraOnboardDelResource(ctx, instanceUrl, apiToken, cli, instanceID)
	if err != nil {
		return err
	}
	err = onboarding_manager.HttpInfraOnboardDelResource(ctx, hostUrl, apiToken, cli, hostID)
	if err != nil {
		return err
	}

	reqCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	err = retry.UntilItSucceeds(
		reqCtx,
		func() error {
			hostID, err := onboarding_manager.HttpInfraOnboardGetNode(reqCtx, hostUrl, apiToken, cli, hostUUID)
			if hostID != "" {
				err = fmt.Errorf("not removed yet")
			}
			if strings.Contains(err.Error(), "empty host result for uuid") {
				return nil
			}
			return err
		},
		1*time.Second,
	)
	if err != nil {
		return err
	}
	return nil
}
