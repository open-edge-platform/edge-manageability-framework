// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/bitfield/script"
	"github.com/golang-jwt/jwt/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog/log"

	"github.com/open-edge-platform/edge-manageability-framework/internal/retry"
	util "github.com/open-edge-platform/edge-manageability-framework/mage"
	invapi "github.com/open-edge-platform/infra-core/apiv2/v2/pkg/api/v2"
	baseorginfrahostcomv1 "github.com/open-edge-platform/orch-utils/tenancy-datamodel/build/apis/org.edge-orchestrator.intel.com/v1"
	baseprojectinfrahostcomv1 "github.com/open-edge-platform/orch-utils/tenancy-datamodel/build/apis/project.edge-orchestrator.intel.com/v1"
)

const outputFile = "../../jwt.txt"

const (
	defaultServiceDomain = "kind.internal"
	defaultServicePort   = 443
)

const (
	platform         = "platform"
	appOrch          = "app-orch"
	clusterOrch      = "cluster-orch"
	clusterOrchSmoke = "cluster-orch-smoke-test"
	infraManagement  = "infra-management"
	ui               = "ui"
	metadataBroker   = "metadata-broker"
)

// revert once CO tests are re-enabled
// const (
// 	timeout  = time.Second * 120
// 	interval = time.Second * 1
// )

type Role struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type Orgs struct {
	Name   string                                `json:"name,omitempty" yaml:"name,omitempty"`
	Spec   *baseorginfrahostcomv1.OrgSpec        `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status *baseorginfrahostcomv1.OrgNexusStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

type Projects struct {
	Name   string                                        `json:"name,omitempty" yaml:"name,omitempty"`
	Spec   *baseprojectinfrahostcomv1.ProjectSpec        `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status *baseprojectinfrahostcomv1.ProjectNexusStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

// serviceDomain is a package-level variable initialized during startup.
var serviceDomain = func() string {
	sd := os.Getenv("E2E_SVC_DOMAIN")
	// retrieve svcdomain from configmap
	// if it does not exist there, then use the defaultservicedomain
	if sd == "" {
		domain, err := util.LookupOrchestratorDomain()
		if err != nil {
			fmt.Printf("error retrieving the orchestrator domain from configmap. Using defaults: %s \n", defaultServiceDomain)
			return defaultServiceDomain
		}

		if len(domain) > 0 {
			sd = domain
		} else {
			sd = defaultServiceDomain
		}
	}

	return sd
}()

// servicePort is a package-level variable initialized during startup.
var servicePort = func() int {
	spStr := os.Getenv("E2E_SVC_PORT")
	if spStr == "" {
		return defaultServicePort
	}

	sp, err := strconv.Atoi(spStr)
	if err != nil {
		fmt.Printf("error converting %s to integer port number. Using defaults\n", spStr)
		return defaultServicePort
	}
	return sp
}()

var serviceDomainWithPort = fmt.Sprintf("%s:%d", serviceDomain, servicePort)

func generateRandomDigits(length int) string {
	rand.Seed(time.Now().UnixNano()) // Seed the random number generator for better randomness

	digits := []byte{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9'}
	result := make([]byte, length)

	for i := 0; i < length; i++ {
		result[i] = digits[rand.Intn(10)]
	}

	return string(result)
}

var _ = Describe("Orchestrator integration test", Label("orchestrator-integration"), func() {
	var cli *http.Client

	BeforeEach(func() {
		cli = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		}

		fmt.Printf("serviceDomain: %v\n", serviceDomain)
	})

	Describe("Harbor OCI service", Label(appOrch), func() {
		It("should be accessible over HTTPS", func() {
			resp, err := cli.Get("https://registry-oci." + serviceDomainWithPort)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			content, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("<title>Harbor</title>"))
		})
	})

	Describe("Self-signed wildcard certificate works with new service", Label(platform), func() {
		serviceName := "test-newservice"
		namespace := "orch-gateway"

		BeforeEach(func() {
			if err := tearDownNewService(serviceName, namespace); err != nil {
				fmt.Println(fmt.Errorf("unable to teardown service before test: %w", err))
			}
			if err := setupNewService("8280", serviceName, namespace, "ClusterIP"); err != nil {
				fmt.Println(fmt.Errorf("unable to setup service: %w", err))
			}
			fmt.Println("finished setting up new service")
		})

		AfterEach(func() {
			if err := tearDownNewService(serviceName, namespace); err != nil {
				fmt.Println(fmt.Errorf("unable to teardown service after test: %w", err))
			}
		})

		It("should be accessible over HTTPS", func() {
			ip, err := util.LookupOrchestratorIP()
			if err != nil {
				Expect(err).ToNot(HaveOccurred())
			}

			var hcli *http.Client

			dialer := &net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 5 * time.Second,
			}
			hcli = &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: tlsConfig,
					DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
						if addr == "test-service."+serviceDomainWithPort {
							addr = net.JoinHostPort(ip, "443")
						}
						return dialer.DialContext(ctx, network, addr)
					},
				},
			}

			Eventually(
				func() error {
					resp, err := hcli.Get("https://test-service." + serviceDomainWithPort)
					if err != nil {
						return fmt.Errorf("failed to get response from service: %w", err)
					}

					if resp.StatusCode != http.StatusForbidden {
						return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
					}

					defer resp.Body.Close()
					return err
				},
				20*time.Second,
				5*time.Second,
			).Should(Succeed())
		})
	})

	Describe("Keycloak service", Label(platform), func() {
		It("should be accessible over HTTPS", func() {
			resp, err := cli.Get("https://keycloak." + serviceDomainWithPort + "/admin/master/console/")
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			content, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("<title>Keycloak Administration Console</title>"))
		})
		It("should show error when redirect_uri is invalid", func() {
			url := ("https://keycloak." + serviceDomainWithPort +
				"/realms/master/protocol/openid-connect/auth?" +
				"client_id=webui-client&" +
				"redirect_uri=https://invalid&" +
				"response_type=code&scope=openid")
			fmt.Println(url)
			resp, err := cli.Get(url)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("UI service", Label(ui), func() {
		It("should be accessible over HTTPS", func() {
			resp, err := cli.Get("https://web-ui." + serviceDomainWithPort)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			content, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("<title>Edge Orchestrator</title>"))
		})

		It("should verify UI response headers", func() {
			resp, err := cli.Get("https://web-ui." + serviceDomainWithPort)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			for k, v := range secureHeadersAdd() {
				Expect(k).To(BeKeyOf(resp.Header))
				Expect(resp.Header.Values(k)).To(ContainElements(v))
			}
			for _, k := range secureHeadersRemove() {
				Expect(k).ToNot(BeKeyOf(resp.Header))
			}
		})

		It("should verify API response COOP & COEP headers", func() {
			req, err := http.NewRequest("GET", "https://api."+serviceDomainWithPort+"/v1/projects/sample-project/compute/os?filter='profile_name=\"tiberos-nonrt\"", nil)
			Expect(err).ToNot(HaveOccurred())
			user := fmt.Sprintf("%s-edge-op", util.TestUser)
			token := getKeycloakJWT(cli, user)
			req.Header.Add("Authorization", "Bearer "+token)
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			for k, v := range coopCoepHeaders() {
				Expect(k).To(BeKeyOf(resp.Header))
				Expect(resp.Header.Values(k)).To(ContainElements(v))
			}
		})

		It("should verify API response CORS headers", func() {
			req, err := http.NewRequest("OPTIONS", "https://api."+serviceDomainWithPort+"/v1/projects/sample-project/compute/os?filter='profile_name=\"tiberos-nonrt\"", nil)
			Expect(err).ToNot(HaveOccurred())
			user := fmt.Sprintf("%s-edge-op", util.TestUser)
			token := getKeycloakJWT(cli, user)
			req.Header.Add("Authorization", "Bearer "+token)
			req.Header.Set("Origin", fmt.Sprintf("https://web-ui.%s", serviceDomain))
			req.Header.Set("Access-Control-Request-Method", "GET")
			req.Header.Set("Access-Control-Request-Headers", "Content-Type")
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			for k, v := range corsHeader() {
				Expect(k).To(BeKeyOf(resp.Header))
				Expect(resp.Header.Values(k)).To(ContainElements(v))
			}
		})

		Describe("Harbor service", Label(appOrch), func() {
			It("should verify Harbor response headers", func() {
				resp, err := cli.Get("https://registry-oci." + serviceDomainWithPort + "/api/v2.0/ping")
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				for k, v := range secureHeadersAdd() {
					Expect(k).To(BeKeyOf(resp.Header))
					Expect(resp.Header.Values(k)).To(ContainElements(v))
				}
				for _, k := range secureHeadersRemove() {
					Expect(k).ToNot(BeKeyOf(resp.Header))
				}
			})
		})

		Describe("App Service Proxy service", Label(appOrch), func() {
			It("should verify ASP response headers", func() {
				resp, err := cli.Get("https://app-service-proxy." + serviceDomainWithPort + "/app-service-proxy-test")
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				for k, v := range secureHeadersAddAppOrch() {
					Expect(k).To(BeKeyOf(resp.Header))
					Expect(resp.Header.Values(k)).To(ContainElements(v))
				}
				for _, k := range secureHeadersRemove() {
					Expect(k).ToNot(BeKeyOf(resp.Header))
				}
			})
		})

		Describe("VNC service", Label(appOrch), func() {
			It("should verify VNC response headers", func() {
				resp, err := cli.Get("https://vnc." + serviceDomainWithPort + "/?project=p1&app=a1&cluster=c1&vm=v1")
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				for k, v := range secureHeadersAddAppOrch() {
					Expect(k).To(BeKeyOf(resp.Header))
					Expect(resp.Header.Values(k)).To(ContainElements(v))
				}
				for _, k := range secureHeadersRemove() {
					Expect(k).ToNot(BeKeyOf(resp.Header))
				}
			})
		})

		// FIXME: Test is needs to be improved to use other source of truth for version
		PIt("should have the version set in the configuration", func() {
			resp, err := cli.Get("https://web-ui." + serviceDomainWithPort + "/runtime-config.js")
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			v, err := os.ReadFile("../../VERSION")
			Expect(err).ToNot(HaveOccurred())
			version := strings.TrimSpace(string(v))
			var orchVersion string
			if strings.Contains(version, "-dev") {
				c, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
				Expect(err).ToNot(HaveOccurred())
				commit := strings.TrimSpace(string(c))
				orchVersion = fmt.Sprintf("v%s-%s", version, commit)
			} else {
				orchVersion = fmt.Sprintf("v%s", version)
			}

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			content, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(ContainSubstring(fmt.Sprintf("orchestrator: \"%s\",", orchVersion)))
		})

		It("should respond to OPTIONS on 403 without server disclosure", Label(ui), func() {
			// Create OPTIONS request to a non-existent URL
			req, err := http.NewRequest("OPTIONS", "https://web-ui."+serviceDomainWithPort+"/mfe/infrastructure/679.d844fa89e1647e1784b6.js", nil)
			Expect(err).ToNot(HaveOccurred())

			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			// Check status code (should be 403)
			Expect(resp.StatusCode).To(Equal(http.StatusForbidden))

			// Verify response doesn't contain nginx server information
			content, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).ToNot(ContainSubstring("nginx"))
			Expect(string(content)).To(ContainSubstring("Error 40x"))
			Expect(string(content)).To(ContainSubstring("<p>"))
			Expect(string(content)).To(ContainSubstring("Oops! The page you are looking for cannot be found"))
			Expect(string(content)).To(ContainSubstring("permission to access it."))
			Expect(string(content)).To(ContainSubstring("</p>"))

			// Verify server header is not present
			Expect("Server").ToNot(BeKeyOf(resp.Header))
		})
	})

	Describe("Metadata Broker service ", Label(metadataBroker, platform), func() {
		metadataUrl := fmt.Sprintf("https://api.%s/v1/projects/%s/metadata", serviceDomainWithPort, util.TestProject)
		It("should be accessible over HTTPS when using valid JWT token", func() {
			req, err := http.NewRequest("GET", metadataUrl, nil)
			Expect(err).ToNot(HaveOccurred())
			user := fmt.Sprintf("%s-edge-op", util.TestUser)
			token := getKeycloakJWT(cli, user)
			req.Header.Add("Authorization", "Bearer "+token)
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			content, err := io.ReadAll(resp.Body)
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK), "API Response code is not 200, body: %s", content)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(ContainSubstring(`{"metadata":[`))
		})
	})

	Describe("UI service with root domain", Label(ui), func() {
		It("should be accessible over HTTPS", func() {
			resp, err := cli.Get("https://" + serviceDomainWithPort)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			content, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("<title>Edge Orchestrator</title>"))
		})
	})

	Describe("Vault service", Label(platform), func() {
		It("should be accessible over HTTPS", func() {
			resp, err := cli.Get("https://vault." + serviceDomainWithPort)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			content, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("<title>Vault</title>"))
		})
	})

	Describe("Orchestrator Observability UI service", Label(platform), func() {
		It("should be accessible over HTTPS", func() {
			resp, err := cli.Get("https://observability-admin." + serviceDomainWithPort)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			_, err = io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("App Deployment Manager service", Label(appOrch), func() {
		admDeploymentsURL := fmt.Sprintf("https://api.%s/v1/projects/%s/appdeployment/deployments", serviceDomainWithPort, util.TestProject)

		It("should NOT be accessible over HTTPS when using valid but expired token", func() {
			Expect(saveToken(cli)).To(Succeed())

			jwt, err := script.File(outputFile).String()
			Expect(err).ToNot(HaveOccurred())
			Expect(jwt).ToNot(BeEmpty())

			isUnexpired, err := isTokenUnexpired(jwt)
			Expect(err).ToNot(HaveOccurred())
			if isUnexpired {
				Skip("Skipping this test because JWT Token is NOT expired")
			}

			request, err := http.NewRequest("GET", admDeploymentsURL, nil)
			Expect(err).ToNot(HaveOccurred())

			// adding JWT to the Authorization header
			request.Header.Add("Authorization", "Bearer "+jwt)

			response, err := cli.Do(request)
			Expect(err).ToNot(HaveOccurred())
			defer response.Body.Close()

			Expect(response.StatusCode).To(Equal(http.StatusForbidden))
		})
		It("should be accessible over HTTPS when using valid token", func() {
			req, err := http.NewRequest("GET", admDeploymentsURL, nil)
			Expect(err).ToNot(HaveOccurred())
			user := fmt.Sprintf("%s-edge-op", util.TestUser)
			token := getKeycloakJWT(cli, user)
			req.Header.Add("Authorization", "Bearer "+token)
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			content, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(ContainSubstring(""))
		})
		It("should NOT be accessible over HTTPS when using no token", func() {
			req, err := http.NewRequest("GET", admDeploymentsURL, nil)
			Expect(err).ToNot(HaveOccurred())
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
		})
		It("should NOT be accessible over HTTPS when using invalid token", func() {
			req, err := http.NewRequest("GET", admDeploymentsURL, nil)
			Expect(err).ToNot(HaveOccurred())
			const invalid = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c" //nolint: lll
			req.Header.Add("Authorization", "Bearer "+invalid)
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
		})
	})

	Describe("Release Service Token endpoint", Label(platform), func() {
		releaseTokenURL := "https://release." + serviceDomainWithPort + "/token"
		It("should NOT be accessible over HTTPS when using valid but expired token", func() {
			Expect(saveToken(cli)).To(Succeed())

			jwt, err := script.File(outputFile).String()
			Expect(err).ToNot(HaveOccurred())
			Expect(jwt).ToNot(BeEmpty())

			isUnexpired, err := isTokenUnexpired(jwt)
			Expect(err).ToNot(HaveOccurred())
			if isUnexpired {
				Skip("Skipping this test because JWT Token is NOT expired")
			}

			request, err := http.NewRequest("GET", releaseTokenURL, nil)
			Expect(err).ToNot(HaveOccurred())

			// adding JWT to the Authorization header
			request.Header.Add("Authorization", "Bearer "+jwt)

			response, err := cli.Do(request)
			Expect(err).ToNot(HaveOccurred())
			defer response.Body.Close()

			Expect(response.StatusCode).To(Equal(http.StatusForbidden))
		})
		It("should be accessible over HTTPS when using valid token", func() {
			req, err := http.NewRequest("GET", releaseTokenURL, nil)
			Expect(err).ToNot(HaveOccurred())
			token := getKeycloakJWT(cli, "all-groups-example-user")
			req.Header.Add("Authorization", "Bearer "+token)
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			content, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).ToNot(BeEmpty())
		})
		It("should NOT be accessible over HTTPS when using valid token without the correct role claim", func() {
			req, err := http.NewRequest("GET", releaseTokenURL, nil)
			Expect(err).ToNot(HaveOccurred())
			token := getKeycloakJWT(cli, "no-groups-example-user")
			req.Header.Add("Authorization", "Bearer "+token)
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			content, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			if resp.StatusCode == http.StatusForbidden {
				Expect(string(content)).To(ContainSubstring("Invalid claims"))
			} else if resp.StatusCode == http.StatusOK {
				Expect(string(content)).To(Equal("anonymous"))
			} else {
				Fail(fmt.Sprintf("Unexpected response code: %d", resp.StatusCode))
			}
		})
		It("should NOT be accessible over HTTPS when using no token", func() {
			req, err := http.NewRequest("GET", releaseTokenURL, nil)
			Expect(err).ToNot(HaveOccurred())
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
		})
		It("should NOT be accessible over HTTPS when using invalid token", func() {
			req, err := http.NewRequest("GET", releaseTokenURL, nil)
			Expect(err).ToNot(HaveOccurred())
			const invalid = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c" //nolint: lll
			req.Header.Add("Authorization", "Bearer "+invalid)
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
		})
	})

	Describe("Cluster Manager service - Templates", Label(clusterOrch), func() {
		templatesUrl := fmt.Sprintf("https://api.%s/v2/projects/%s/templates", serviceDomainWithPort, util.TestProject)
		coUser := fmt.Sprintf("%s-edge-op", util.TestUser)

		It("should be accessible over HTTPS when using valid token", func() {
			Eventually(func() bool {
				req, err := http.NewRequest("GET", templatesUrl, nil)
				Expect(err).ToNot(HaveOccurred())
				token := getKeycloakJWT(cli, coUser)
				req.Header.Add("Authorization", "Bearer "+token)
				resp, err := cli.Do(req)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				return resp.StatusCode == http.StatusOK
			}, 20*time.Second, 5*time.Second).Should(BeTrue())
		})

		nonCoUser := fmt.Sprintf("%s-api-user", util.TestUser)
		It("should NOT be accessible over HTTPS when using valid token with invalid roles", func() {
			req, err := http.NewRequest("GET", templatesUrl, nil)
			Expect(err).ToNot(HaveOccurred())
			token := getKeycloakJWT(cli, nonCoUser)
			req.Header.Add("Authorization", "Bearer "+token)
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		})

		It("should NOT be accessible over HTTPS when using invalid token", func() {
			req, err := http.NewRequest("GET", templatesUrl, nil)
			Expect(err).ToNot(HaveOccurred())
			const invalid = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c" //nolint: lll
			req.Header.Add("Authorization", "Bearer "+invalid)
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
		})

		It("should NOT be accessible over HTTPS when using no token", func() {
			req, err := http.NewRequest("GET", templatesUrl, nil)
			Expect(err).ToNot(HaveOccurred())
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
		})

		It("should NOT be accessible over HTTPS when using valid but expired token", func() { //nolint: dupl
			Expect(saveToken(cli)).To(Succeed())
			token, err := script.File(outputFile).String()
			Expect(err).ToNot(HaveOccurred())
			Expect(token).ToNot(BeEmpty())

			isUnexpired, err := isTokenUnexpired(token)
			Expect(err).ToNot(HaveOccurred())

			if isUnexpired {
				Skip("Skipping this test because JWT Token is NOT expired")
			}

			req, err := http.NewRequest("GET", templatesUrl, nil)
			Expect(err).ToNot(HaveOccurred())

			req.Header.Add("Authorization", "Bearer "+token)
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
		})
	})

	Describe("Cluster connect gateway", Label(clusterOrch), func() {
		ccgUrl := fmt.Sprintf("https://connect-gateway.%s/kubernetes/%s-randomid/v1/pods", serviceDomainWithPort, util.TestProject)
		It("should NOT be accessible when using invalid token", func() {
			req, err := http.NewRequest("GET", ccgUrl, nil)
			Expect(err).ToNot(HaveOccurred())
			const invalid = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c" //nolint: lll
			req.Header.Add("Authorization", "Bearer "+invalid)
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		})

		It("should NOT be accessible over HTTPS when using no token", func() {
			req, err := http.NewRequest("GET", ccgUrl, nil)
			Expect(err).ToNot(HaveOccurred())
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		})

		It("should NOT be accessible over HTTPS when using valid but expired token", func() {
			Expect(saveToken(cli)).To(Succeed())
			token, err := script.File(outputFile).String()
			Expect(err).ToNot(HaveOccurred())
			Expect(token).ToNot(BeEmpty())

			isUnexpired, err := isTokenUnexpired(token)
			Expect(err).ToNot(HaveOccurred())
			if isUnexpired {
				Skip("Skipping this test because JWT Token is NOT expired")
			}

			req, err := http.NewRequest("GET", ccgUrl, nil)
			Expect(err).ToNot(HaveOccurred())

			req.Header.Add("Authorization", "Bearer "+token)
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		})
	})

	Describe("Cluster Manager service - Clusters", Label(clusterOrch), func() {
		cmUrl := fmt.Sprintf("https://api.%s/v2/projects/%s/clusters", serviceDomainWithPort, util.TestProject)
		coUser := fmt.Sprintf("%s-edge-op", util.TestUser)
		It("should be accessible over HTTPS when using valid token", func() {
			req, err := http.NewRequest("GET", cmUrl, nil)
			Expect(err).ToNot(HaveOccurred())
			token := getKeycloakJWT(cli, coUser)
			req.Header.Add("Authorization", "Bearer "+token)
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			content, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(ContainSubstring(`{"clusters":[`))
		})

		nonCoUser := fmt.Sprintf("%s-api-user", util.TestUser)
		It("should NOT be accessible over HTTPS when using valid token with invalid roles", func() {
			req, err := http.NewRequest("GET", cmUrl, nil)
			Expect(err).ToNot(HaveOccurred())
			token := getKeycloakJWT(cli, nonCoUser)
			req.Header.Add("Authorization", "Bearer "+token)
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		})

		It("should NOT be accessible over HTTPS when using invalid token", func() {
			req, err := http.NewRequest("GET", cmUrl, nil)
			Expect(err).ToNot(HaveOccurred())
			const invalid = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c" //nolint: lll
			req.Header.Add("Authorization", "Bearer "+invalid)
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
		})

		It("should NOT be accessible over HTTPS when using no token", func() {
			req, err := http.NewRequest("GET", cmUrl, nil)
			Expect(err).ToNot(HaveOccurred())
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
		})

		It("should NOT be accessible over HTTPS when using valid but expired token", func() {
			Expect(saveToken(cli)).To(Succeed())
			token, err := script.File(outputFile).String()
			Expect(err).ToNot(HaveOccurred())
			Expect(token).ToNot(BeEmpty())

			isUnexpired, err := isTokenUnexpired(token)
			Expect(err).ToNot(HaveOccurred())
			if isUnexpired {
				Skip("Skipping this test because JWT Token is NOT expired")
			}

			req, err := http.NewRequest("GET", cmUrl, nil)
			Expect(err).ToNot(HaveOccurred())

			req.Header.Add("Authorization", "Bearer "+token)
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
		})
	})
})

// Improved logging function for consistency
func logInfo(format string, v ...interface{}) {
	log.Info().Msgf(format, v...)
}

// Utility functions for making authorized requests and parsing responses
func makeAuthorizedRequest(method, url, token string, body []byte, cli *http.Client) (*http.Response, error) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")
	return cli.Do(req)
}

func parseOrgsList(body io.ReadCloser) ([]Orgs, error) {
	var orgsList []Orgs

	data, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	if err = json.Unmarshal(data, &orgsList); err != nil {
		return nil, fmt.Errorf("failed to parse orgs list: %w", err)
	}

	return orgsList, nil
}

func parseOrg(body io.ReadCloser) (Orgs, error) {
	var org Orgs
	data, err := io.ReadAll(body)
	if err != nil {
		return org, fmt.Errorf("Failed to read body")
	}

	err = json.Unmarshal(data, &org)
	if err != nil {
		return org, fmt.Errorf("Failed to Unmarshal org")
	}

	return org, nil
}

func parseProjectsList(body io.ReadCloser) ([]Projects, error) {
	var projList []Projects

	data, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	if err := json.Unmarshal(data, &projList); err != nil {
		return nil, fmt.Errorf("failed to parse project list: %w", err)
	}

	return projList, nil
}

func parseProject(body io.ReadCloser) (Projects, error) {
	var proj Projects
	data, err := io.ReadAll(body)
	if err != nil {
		return proj, fmt.Errorf("Failed to read body")
	}
	err = json.Unmarshal(data, &proj)
	if err != nil {
		return proj, fmt.Errorf("Failed to Unmarshal proj")
	}

	return proj, nil
}

func parseRegionsList(body io.ReadCloser) (*invapi.ListRegionsResponse, error) {
	regionsList := &invapi.ListRegionsResponse{}

	data, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	if err = json.Unmarshal(data, &regionsList); err != nil {
		return nil, fmt.Errorf("failed to Unmarshal regionsList: %w", err)
	}

	return regionsList, nil
}

func getKeycloakJWT(cli *http.Client, username string) string {
	// this password is used by users that are created pre e2e tests and use this by default.
	// This is not the password for the admin user.
	keycloakCreds, err := util.GetDefaultOrchPassword()
	Expect(err).ToNot(HaveOccurred())

	if username == "admin" {
		keycloakCreds, err = util.GetKeycloakSecret()
		Expect(err).ToNot(HaveOccurred())
	}

	token, err := util.GetApiToken(cli, username, keycloakCreds)
	Expect(err).ToNot(HaveOccurred())
	return *token
}

// saveToken should persist token to a file only if one does not exist.
func saveToken(cli *http.Client) error {
	fileInfo, err := os.Stat(outputFile)
	if err != nil || fileInfo.Size() == 0 {
		jwt := getKeycloakJWT(cli, "all-groups-example-user")
		// create file if it does not exist
		_, err := script.Echo(jwt).WriteFile(outputFile)
		fileInfo, _ = os.Stat(outputFile)
		if fileInfo.Size() == 0 {
			return fmt.Errorf("Token file is empty")
		}
		return err
	}
	return nil
}

// saveToken should persist token to a file only if one does not exist.
func saveTokenUser(cli *http.Client, username, password string) error {
	fileInfo, err := os.Stat(outputFile)
	if err != nil || fileInfo.Size() == 0 {
		token, err := util.GetApiToken(cli, username, password)
		if err != nil {
			return err
		}
		// create file if it does not exist
		_, err = script.Echo(*token).WriteFile(outputFile)
		fileInfo, _ = os.Stat(outputFile)
		if fileInfo.Size() == 0 {
			return fmt.Errorf("Token file is empty")
		}
		return err
	}
	return nil
}

func isTokenUnexpired(tokenStr string) (bool, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenStr, jwt.MapClaims{})
	if err != nil {
		return false, fmt.Errorf("parsing jwt token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return false, fmt.Errorf("error converting token's claims to standard claims")
	}

	var tm time.Time
	switch iat := claims["exp"].(type) {
	case float64:
		tm = time.Unix(int64(iat), 0)
	case json.Number:
		v, err := iat.Int64()
		if err != nil {
			return false, err
		}
		tm = time.Unix(v, 0)
	}
	unexpired := time.Now().UTC().Before(tm)

	return unexpired, nil
}

func corsHeader() map[string][]string {
	return map[string][]string{
		"Access-Control-Allow-Origin":      {"https://web-ui." + serviceDomain},
		"Access-Control-Allow-Methods":     {"GET,OPTIONS,PUT,PATCH,POST,DELETE"},
		"Access-Control-Allow-Headers":     {"*"},
		"Access-Control-Allow-Credentials": {"true"},
		"Access-Control-Max-Age":           {"100"},
	}
}

func coopCoepHeaders() map[string][]string {
	return map[string][]string{
		"Cross-Origin-Embedder-Policy": {"require-corp"},
		"Cross-Origin-Opener-Policy":   {"same-origin"},
	}
}

func secureHeadersAdd() map[string][]string {
	// adapted from https://owasp.org/www-project-secure-headers/ci/headers_add.json
	return map[string][]string{
		"Referrer-Policy":                   {"no-referrer"},
		"Strict-Transport-Security":         {"max-age=31536000; includeSubDomains"},
		"X-Content-Type-Options":            {"nosniff"},
		"X-Frame-Options":                   {"DENY"}, // case sensitive
		"X-Permitted-Cross-Domain-Policies": {"none"},
		"Pragma":                            {"no-cache"},
		"Cache-Control":                     {"no-store, max-age=0"},
		// Original OWASP recommendation is modified:
		// -cache: might not be strictly required, but it makes the UI load faster
		// -cookies: and storage are used to store the OIDC token used for authentication
		// "Clear-Site-Data":                   {"\"cache\",\"cookies\",\"storage\""},
		//
		// Original OWASP recommendation is modified (temporarily);
		// there will be a follow up release to remove unsafe directives:
		// "Content-Security-Policy":
		// {"default-src 'self'; form-action 'self'; object-src 'none';
		// frame-ancestors 'none'; upgrade-insecure-requests;
		// block-all-mixed-content"}, //nolint: lll
		"Content-Security-Policy": {fmt.Sprintf("default-src 'self'; form-action 'self'; object-src 'none'; frame-ancestors 'none'; script-src 'self' 'unsafe-eval' https://app-service-proxy.%s; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self' https://keycloak.%s wss://vnc.%s https://app-service-proxy.%s https://app-orch.%s https://api.%s https://cluster-orch.%s https://metadata.%s https://alerting-monitor.%s; upgrade-insecure-requests; block-all-mixed-content", //nolint: lll
			serviceDomain, serviceDomain, serviceDomain, serviceDomain, serviceDomain, serviceDomain, serviceDomain, serviceDomain, serviceDomain)},
		"Cross-Origin-Embedder-Policy": {"require-corp"},
		"Cross-Origin-Opener-Policy":   {"same-origin"},
		"Cross-Origin-Resource-Policy": {"same-origin"},
		"Permissions-Policy": {
			"accelerometer=(),ambient-light-sensor=(),autoplay=(),battery=(),camera=(),display-capture=(),document-domain=(),encrypted-media=(),fullscreen=(),gamepad=(),geolocation=(),gyroscope=(),layout-animations=(self),legacy-image-formats=(self),magnetometer=(),microphone=(),midi=(),oversized-images=(self),payment=(),picture-in-picture=(),publickey-credentials-get=(),speaker-selection=(),sync-xhr=(self),unoptimized-images=(self),unsized-media=(self),usb=(),screen-wake-lock=(),web-share=(),xr-spatial-tracking=()", //nolint: lll
		},
	}
}

func secureHeadersAddAppOrch() map[string][]string {
	// adapted from https://owasp.org/www-project-secure-headers/ci/headers_add.json
	appOrchSecureHeaders := secureHeadersAdd()

	appOrchSecureHeaders["Content-Security-Policy"] = []string{fmt.Sprintf("default-src 'self'; form-action 'self'; object-src 'none'; frame-ancestors 'none'; script-src 'self' ; frame-src 'self' https://keycloak.%s; style-src 'self'; img-src 'self' data:; connect-src 'self' https://keycloak.%s; upgrade-insecure-requests; block-all-mixed-content", //nolint: lll
		serviceDomain, serviceDomain)}
	delete(appOrchSecureHeaders, "X-Content-Type-Options")
	delete(appOrchSecureHeaders, "X-Frame-Options")

	return appOrchSecureHeaders
}

func secureHeadersRemove() []string {
	// adapted from https://owasp.org/www-project-secure-headers/ci/headers_remove.json
	return []string{
		"$wsep",
		"Host-Header",
		"K-Proxy-Request",
		"Liferay-Portal",
		"OracleCommerceCloud-Version",
		"Pega-Host",
		"Powered-By",
		"Product",
		"Server",
		"SourceMap",
		"X-AspNet-Version",
		"X-AspNetMvc-Version",
		"X-Atmosphere-error",
		"X-Atmosphere-first-request",
		"X-Atmosphere-tracking-id",
		"X-B3-ParentSpanId",
		"X-B3-Sampled",
		"X-B3-SpanId",
		"X-B3-TraceId",
		"X-CF-Powered-By",
		"X-CMS",
		"X-Content-Encoded-By",
		"X-Envoy-Attempt-Count",
		"X-Envoy-External-Address",
		"X-Envoy-Internal",
		"X-Envoy-Original-Dst-Host",
		"X-Envoy-Upstream-Service-Time",
		"X-Framework",
		"X-Generated-By",
		"X-Generator",
		"X-LiteSpeed-Cache",
		"X-LiteSpeed-Purge",
		"X-LiteSpeed-Tag",
		"X-LiteSpeed-Vary",
		"X-Litespeed-Cache-Control",
		"X-Mod-Pagespeed",
		"X-Nextjs-Cache",
		"X-Nextjs-Matched-Path",
		"X-Nextjs-Page",
		"X-Nextjs-Redirect",
		"X-Old-Content-Length",
		"X-OneAgent-JS-Injection",
		"X-Page-Speed",
		"X-Php-Version",
		"X-Powered-By",
		"X-Powered-By-Plesk",
		"X-Powered-CMS",
		"X-Redirect-By",
		"X-Server-Powered-By",
		"X-SourceFiles",
		"X-SourceMap",
		"X-Turbo-Charged-By",
		"X-Umbraco-Version",
		"X-Varnish-Backend",
		"X-Varnish-Server",
		"X-dtAgentId",
		"X-dtHealthCheck",
		"X-dtInjectedServlet",
		"X-ruxit-JS-Agent",
	}
}

// createPod creates a new Kubernetes pod using kubectl.
func createPod(podName, imageName, namespace, command string) error {
	// Construct the kubectl command to create a new pod.
	cmd := exec.Command("kubectl", "run", podName, "--image", imageName, "-n", namespace, "--port", "8280", "--", "/bin/sh", "-c", command)

	// Run the kubectl command.
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error creating pod: %w, output: %s", err, string(output))
	}

	fmt.Printf("Pod created successfully: %s\n", string(output))
	return nil
}

// createService creates a new Kubernetes service using kubectl.
func createService(serviceName, serviceType, portMapping, namespace string) error {
	// Construct the kubectl command to create a new service.
	cmd := exec.Command("kubectl", "expose", "pod", serviceName, "--type", serviceType, "--port", portMapping, "-n", namespace)

	// Run the kubectl command.
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error creating service: %w, output: %s", err, string(output))
	}

	fmt.Printf("Service created successfully: %s\n", string(output))
	return nil
}

// IngressParams holds parameters for creating an Ingress resource.
type IngressParams struct {
	IngressName string
	Namespace   string
	Host        string
	Path        string
	ServiceName string
	ServicePort int
}

// ingressTemplate is a template for creating an Ingress resource with Traefik annotations.
const ingressTemplate = `
apiVersion: traefik.containo.us/v1alpha1
kind: IngressRoute
metadata:
  name: {{.ServiceName}}
  namespace: {{.Namespace}}
spec:
  entryPoints:
    - websecure
  routes:
      - match: Host(` + "`" + `{{.Host}}` + "`" + `)
        kind: Rule
        middlewares:
          - name: limit-request-size
          - name: validate-jwt
        services:
          - name: {{.ServiceName}}
            port: {{.ServicePort}}
            scheme: http
            namespace: {{.Namespace}}
  tls:
    secretName: tls-orch
    options:
      name: gateway-tls
      namespace: orch-gateway
`

// createIngress creates a new Ingress resource for Traefik using kubectl.
func createIngress(params IngressParams) error {
	// Parse the ingress template.
	tmpl, err := template.New("ingress").Parse(ingressTemplate)
	if err != nil {
		return fmt.Errorf("error parsing ingress template: %w", err)
	}

	// Execute the template with the provided parameters.
	var ingressManifest bytes.Buffer
	err = tmpl.Execute(&ingressManifest, params)
	if err != nil {
		return fmt.Errorf("error executing ingress template: %w", err)
	}

	// Run the kubectl command to apply the Ingress resource.
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = bytes.NewReader(ingressManifest.Bytes())

	// Run the kubectl command.
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error creating ingress: %w, output: %s", err, string(output))
	}

	fmt.Printf("Ingress created successfully: %s\n", string(output))
	return nil
}

func tearDownNewService(serviceName, namespace string) error {
	// Construct the kubectl delete service command
	deleteServiceCmd := exec.Command("kubectl", "delete", "--ignore-not-found=true", "service", serviceName, "-n", namespace)
	// Run the kubectl delete service command
	if err := deleteServiceCmd.Run(); err != nil {
		fmt.Printf("Error deleting service: %v\n", err)
		return err
	}

	// Construct the kubectl delete service command
	deleteIngressRouteCmd := exec.Command("kubectl", "delete", "--ignore-not-found=true", "ingressroute", serviceName, "-n", namespace)
	// Run the kubectl delete service command
	if err := deleteIngressRouteCmd.Run(); err != nil {
		fmt.Printf("Error deleting ingress route: %v\n", err)
		return err
	}

	// Construct the kubectl delete pod command
	deletePodCmd := exec.Command("kubectl", "delete", "pod", "--ignore-not-found=true", "grace-period=0", "--force", serviceName, "-n", namespace)
	// Run the kubectl delete service command
	if err := deletePodCmd.Run(); err != nil {
		fmt.Printf("Error deleting pod: %v\n", err)
		return err
	}
	return nil
}

func setupNewService(port, serviceName, namespace, serviceType string) error {
	// Create a simple http server pod
	if err := createPod(serviceName, "busybox", namespace, fmt.Sprintf("echo 'Hello' > /var/www/index.html && httpd -f -p %s -h /var/www/", port)); err != nil {
		return err
	}

	timeout := 120 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Wait until the pod is running
	if err := retry.UntilItSucceeds(
		ctx,
		func() error {
			podUp, err := script.NewPipe().
				Exec(fmt.Sprintf("kubectl get pod %s -n %s -o json", serviceName, namespace)).
				JQ(`.status.phase == "Running"`).
				String()
			if err != nil {
				return err
			}
			if strings.TrimSpace(podUp) == "true" {
				return nil
			}
			return fmt.Errorf("test pod not ready yet.")
		},
		5*time.Second,
	); err != nil {
		return fmt.Errorf("test failed: %w ❌", err)
	}

	// Create a service to expose the BusyBox pod.
	if err := createService(serviceName, serviceType, port, namespace); err != nil {
		fmt.Println("Error creating service:", err)
		return err
	}

	// Create the new traefik ingress route
	if err := createIngress(IngressParams{
		IngressName: "test-service-ingress",
		Namespace:   namespace,
		Host:        "test-service." + serviceDomain,
		Path:        "/",
		ServiceName: serviceName,
		ServicePort: 8280,
	}); err != nil {
		fmt.Println("Error creating ingress route:", err)
	}
	return nil
}
