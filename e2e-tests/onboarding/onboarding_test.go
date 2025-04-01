// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package onboarding_test

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/open-edge-platform/edge-manageability-framework/mage"
)

var _ = Describe("Node Onboarding test (Non-Interactive flow)", func() {
	It("should onboard a node successfully", func(ctx SpecContext) {
		By("Deploying a new Edge Node")
		serialNumber, err := mage.Deploy{}.VENWithFlow(ctx, "nio")
		Expect(err).NotTo(HaveOccurred())

		By("Creating an Orchestrator client")
		httpCli := &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}

		By("Getting an Orchestrator API token")
		projectAPIPass, err := mage.GetDefaultOrchPassword()
		Expect(err).NotTo(HaveOccurred())

		token, err := mage.GetApiToken(httpCli, "sample-project-api-user", projectAPIPass)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("Using token: %s\n", *token)

		By("Waiting for the Edge Node to onboard to Orchestrator")
		Eventually(func() error {
			return checkNodeOnboarding(ctx, httpCli, *token, serialNumber)
		}, 10*time.Minute, 15*time.Second).Should(Succeed(), "Edge Node did not onboard in time")
	})
})

// The IO test should be left as pending so that it can continue to be executed manually.
var _ = PDescribe("Node Onboarding test (Interactive Onboarding flow)", func() {
	It("should onboard a node successfully", func(ctx SpecContext) {
		By("Deploying a new Edge Node")
		serialNumber, err := mage.Deploy{}.VENWithFlow(ctx, "io")
		Expect(err).NotTo(HaveOccurred())

		By("Creating an Orchestrator client")
		httpCli := &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}

		By("Getting an Orchestrator API token")
		projectAPIPass, err := mage.GetDefaultOrchPassword()
		Expect(err).NotTo(HaveOccurred())

		token, err := mage.GetApiToken(httpCli, "sample-project-api-user", projectAPIPass)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("Using token: %s\n", *token)

		By("Waiting for the Edge Node to onboard to Orchestrator")
		Eventually(func() error {
			return checkNodeOnboarding(ctx, httpCli, *token, serialNumber)
		}, 10*time.Minute, 15*time.Second).Should(Succeed(), "Edge Node did not onboard in time")
	})
})

func checkNodeOnboarding(ctx SpecContext, httpCli *http.Client, token string, serialNumber string) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("https://api.%s/v1/projects/%s/compute/hosts", serviceDomain, "sample-project"),
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpCli.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		GinkgoWriter.Printf("Attempt failed: expected status code 200, got %d, will retry\n", resp.StatusCode)
		return fmt.Errorf("expected status code 200, got %d, will retry", resp.StatusCode)
	}

	var response struct {
		Hosts []struct {
			SerialNumber     string `json:"serialNumber"`
			OnboardingStatus string `json:"onboardingStatus"`
		} `json:"hosts"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	for _, host := range response.Hosts {
		if host.SerialNumber == serialNumber && host.OnboardingStatus == "Onboarded" {
			GinkgoWriter.Printf("Edge Node with serial number %s onboarded successfully\n", serialNumber)
			return nil
		}
	}

	GinkgoWriter.Printf("Edge Node with serial number %s not yet onboarded, will retry\n", serialNumber)
	return fmt.Errorf("Edge Node with serial number %s not yet onboarded, will retry", serialNumber)
}
