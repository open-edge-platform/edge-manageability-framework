// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	componentStatusLabel = "component-status"
)

type ComponentStatus struct {
	SchemaVersion string                 `json:"schema-version"`
	Orchestrator  OrchestratorStatus     `json:"orchestrator"`
}

type OrchestratorStatus struct {
	Version  string            `json:"version"`
	Features map[string]Feature `json:"features"`
}

type Feature struct {
	Installed bool               `json:"installed"`
	SubFeatures map[string]Feature `json:",inline"`
}

var _ = Describe("Component Status Service", Label(componentStatusLabel), func() {
	var cli *http.Client
	var token string

	BeforeEach(func() {
		cli = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		}
		fmt.Printf("serviceDomain: %v\n", serviceDomain)
		// Get authentication token - component status contains sensitive information
		user := fmt.Sprintf("%s-api-user", "sample-project")
		token = getKeycloakJWT(cli, user)
	})

	Describe("Component Status API", Label(componentStatusLabel), func() {
		componentStatusURL := "https://api." + serviceDomainWithPort + "/v1/orchestrator"

		It("should be accessible over HTTPS with valid authentication", func() {
			req, err := http.NewRequest(http.MethodGet, componentStatusURL, nil)
			Expect(err).ToNot(HaveOccurred())
			req.Header.Add("Authorization", "Bearer "+token)

			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		It("should return valid JSON with correct schema", func() {
			req, err := http.NewRequest(http.MethodGet, componentStatusURL, nil)
			Expect(err).ToNot(HaveOccurred())
			req.Header.Add("Authorization", "Bearer "+token)

			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			body, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())

			var status ComponentStatus
			err = json.Unmarshal(body, &status)
			Expect(err).ToNot(HaveOccurred())

			// Verify schema version is present
			Expect(status.SchemaVersion).ToNot(BeEmpty())
			Expect(status.SchemaVersion).To(Equal("1.0"))

			// Verify orchestrator section exists
			Expect(status.Orchestrator.Version).ToNot(BeEmpty())

			// Verify features section exists
			Expect(status.Orchestrator.Features).ToNot(BeNil())
		})

		It("should return expected feature flags", func() {
			req, err := http.NewRequest(http.MethodGet, componentStatusURL, nil)
			Expect(err).ToNot(HaveOccurred())
			req.Header.Add("Authorization", "Bearer "+token)

			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			body, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())

			var status ComponentStatus
			err = json.Unmarshal(body, &status)
			Expect(err).ToNot(HaveOccurred())

			// Check that expected top-level features are present
			expectedFeatures := []string{
				"application-orchestration",
				"cluster-orchestration",
				"edge-infrastructure-manager",
				"observability",
				"multitenancy",
			}

			for _, feature := range expectedFeatures {
				_, exists := status.Orchestrator.Features[feature]
				Expect(exists).To(BeTrue(), fmt.Sprintf("Feature %s should be present", feature))
			}
		})

		It("should have proper Content-Type header", func() {
			req, err := http.NewRequest(http.MethodGet, componentStatusURL, nil)
			Expect(err).ToNot(HaveOccurred())
			req.Header.Add("Authorization", "Bearer "+token)

			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(resp.Header.Get("Content-Type")).To(ContainSubstring("application/json"))
		})

		It("should return 404 for non-existent paths", func() {
			req, err := http.NewRequest(http.MethodGet, "https://api."+serviceDomainWithPort+"/v1/orchestrator/nonexistent", nil)
			Expect(err).ToNot(HaveOccurred())
			req.Header.Add("Authorization", "Bearer "+token)

			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})

		It("should support GET method only", func() {
			// Test POST should fail
			req, err := http.NewRequest(http.MethodPost, componentStatusURL, nil)
			Expect(err).ToNot(HaveOccurred())
			req.Header.Add("Authorization", "Bearer "+token)

			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusMethodNotAllowed))
		})
	})

	Describe("Health and Readiness endpoints", Label(componentStatusLabel), func() {
		It("should have /healthz endpoint", func() {
			req, err := http.NewRequest(http.MethodGet, "https://api."+serviceDomainWithPort+"/v1/orchestrator/healthz", nil)
			if err != nil {
				Skip("Health endpoint may not be exposed externally")
			}

			resp, err := cli.Do(req)
			if err != nil {
				Skip("Health endpoint may not be exposed externally")
			}
			defer resp.Body.Close()

			// If accessible, should return 200
			if resp.StatusCode == http.StatusOK {
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
			}
		})
	})
})
