// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package onboarding_test

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/open-edge-platform/edge-manageability-framework/mage"
)

// This test suites contain tests that validate the ability to onboard an Edge Node to Orchestrator. It requires a
// running k8s instance of Orchestrator to execute tests against. It implicitly assumes domain name resolution is setup
// on the machine executing these tests. All tests in this suite should be black-box tests. In other words, the tests
// should never manipulate systems under tests in a way that an end user cannot. For example, a test that remote
// connects over SSH to change a parameter in order to make assertions against the system fails this condition.
// Alternatively, a public API should be exposed to manipulate this parameter in order to write tests that make
// assertions on actual system behavior.
func TestOnboarding(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Edge Node Onboarding Suite")
}

var (
	// defaultServiceDomain is the root domain name of the Orchestrator service. It can be overridden by setting the
	// E2E_SVC_DOMAIN environment variable.
	defaultServiceDomain = "cluster.onprem"
	// servicePort is the port number of the Orchestrator service. It can be overridden by setting the
	// E2E_SVC_PORT environment variable.
	servicePort = "443"
)

var serviceDomain = func() string {
	sd := os.Getenv("E2E_SVC_DOMAIN")
	// retrieve svcdomain from configmap
	// if it does not exist there, then use the defaultservicedomain
	if sd == "" {
		domain, err := mage.LookupOrchestratorDomain()
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

var _ = SynchronizedBeforeSuite(func() []byte {
	if os.Getenv("KUBECONFIG") == "" {
		Fail("KUBECONFIG environment variable must be set to run e2e tests")
	}

	if os.Getenv("E2E_SVC_PORT") != "" {
		servicePort = os.Getenv("E2E_SVC_PORT")
	}

	GinkgoWriter.Printf("Using service domain: %s\n", serviceDomain)

	By("Waiting for the DKAM service to be ready")
	Eventually(func() error {
		req, err := http.NewRequest(
			http.MethodGet,
			"https://"+serviceDomain+":"+servicePort+"/tink-stack/keys/Full_server.crt",
			nil,
		)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		GinkgoWriter.Printf("Checking if DKAM returns the Orchestrator TLS certificate at %s\n", req.URL.String())

		httpCli := &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}

		resp, err := httpCli.Do(req)
		if err != nil {
			return fmt.Errorf("failed to do request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("expected status code 200, got %d, will retry", resp.StatusCode)
		}

		return nil
	}, 15*time.Second, 10*time.Minute).Should(Succeed(), "DKAM service did not become ready in time")

	return nil
}, func(data []byte) {})

var _ = SynchronizedAfterSuite(func() {}, func(ctx SpecContext) {
	By("Cleaning up the deployed Edge Nodes")
	serialNumber := "EN123456789"
	if err := (mage.Undeploy{}).VEN(ctx, serialNumber); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to undeploy Edge Nodes: %v\n", err)
	}
})
