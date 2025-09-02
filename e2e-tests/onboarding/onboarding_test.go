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
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/open-edge-platform/edge-manageability-framework/mage"
)

var _ = Describe("Node Onboarding test (Non-Interactive flow)", func() {
	It("should onboard a node successfully", func(ctx SpecContext) {
		By("Deploying a new Edge Node")

		// Copy the current working directory to restore it later
		// This is necessary because the mage.Deploy{}.VENWithFlow function
		// changes the current working directory to the directory where
		// the Edge Node deployment files are located.
		initialDir, err := os.Getwd()
		Expect(err).NotTo(HaveOccurred())

		serialNumber := "EN123456789"
		err = mage.Deploy{}.VENWithFlow(ctx, "nio", serialNumber)
		Expect(err).NotTo(HaveOccurred())

		// Restore the initial working directory
		err = os.Chdir(initialDir)
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

		if os.Getenv("EN_PROFILE") == "microvisor-standalone" {
			By("Waiting for the Edge Node to reach 'Provisioned' status")
			Eventually(func() error {
				return checkNodeProvisioning(ctx, httpCli, *token, serialNumber)
			}, 20*time.Minute, 15*time.Second).Should(Succeed(), "Edge Node did not provision in time")
		} else {
			By("Waiting for the Edge Node to reach 'Running' status")
			Eventually(func() error {
				return checkHostStatus(ctx, httpCli, *token, serialNumber)
			}, 20*time.Minute, 15*time.Second).Should(Succeed(), "Edge Node did not reach 'Running' status in time")
		}
	})
})

// The IO test should be left as pending so that it can continue to be executed manually.
var _ = PDescribe("Node Onboarding test (Interactive Onboarding flow)", func() {
	It("should onboard a node successfully", func(ctx SpecContext) {
		By("Deploying a new Edge Node")

		// Copy the current working directory to restore it later
		// This is necessary because the mage.Deploy{}.VENWithFlow function
		// changes the current working directory to the directory where
		// the Edge Node deployment files are located.
		initialDir, err := os.Getwd()
		Expect(err).NotTo(HaveOccurred())

		serialNumber := "EN123456789"
		err = mage.Deploy{}.VENWithFlow(ctx, "io", serialNumber)
		Expect(err).NotTo(HaveOccurred())

		// Restore the initial working directory
		err = os.Chdir(initialDir)
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

		if os.Getenv("EN_PROFILE") == "microvisor-standalone" {
			By("Waiting for the Edge Node to reach 'Provisioned' status")
			Eventually(func() error {
				return checkNodeProvisioning(ctx, httpCli, *token, serialNumber)
			}, 20*time.Minute, 15*time.Second).Should(Succeed(), "Edge Node did not provision in time")
		} else {
			By("Waiting for the Edge Node to reach 'Running' status")
			Eventually(func() error {
				return checkHostStatus(ctx, httpCli, *token, serialNumber)
			}, 20*time.Minute, 15*time.Second).Should(Succeed(), "Edge Node did not reach 'Running' status in time")
		}
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

func checkNodeProvisioning(ctx SpecContext, httpCli *http.Client, token string, serialNumber string) error {
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

	type Instance struct {
		ProvisioningStatus string `json:"provisioningStatus"`
	}

	var response struct {
		Hosts []struct {
			SerialNumber     string   `json:"serialNumber"`
			OnboardingStatus string   `json:"onboardingStatus"`
			Instance         Instance `json:"instance"`
		} `json:"hosts"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	for _, host := range response.Hosts {
		if host.SerialNumber == serialNumber && host.Instance.ProvisioningStatus == "Provisioned" {
			GinkgoWriter.Printf("Edge Node with serial number %s provisioned successfully\n", serialNumber)
			return nil
		}
	}

	GinkgoWriter.Printf("Edge Node with serial number %s not yet provisioned, will retry\n", serialNumber)
	return fmt.Errorf("Edge Node with serial number %s not yet provisioned, will retry", serialNumber)
}

func checkHostStatus(ctx SpecContext, httpCli *http.Client, token string, serialNumber string) error {
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
			SerialNumber string `json:"serialNumber"`
			HostStatus   string `json:"hostStatus"`
		} `json:"hosts"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	for _, host := range response.Hosts {
		if host.SerialNumber == serialNumber && host.HostStatus == "Running" {
			GinkgoWriter.Printf("Edge Node with serial number %s reached 'Running' status\n", serialNumber)
			return nil
		}
	}

	GinkgoWriter.Printf("Edge Node with serial number %s not yet reached 'Running' status, will retry\n", serialNumber)
	return fmt.Errorf("Edge Node with serial number %s not yet reached 'Running' status, will retry\n", serialNumber)
}
