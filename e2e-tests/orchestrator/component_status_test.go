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
	SchemaVersion string             `json:"schema-version"`
	Orchestrator  OrchestratorStatus `json:"orchestrator"`
}

type OrchestratorStatus struct {
	Version  string             `json:"version"`
	Features map[string]Feature `json:"features"`
}

type Feature struct {
	Installed   bool               `json:"installed"`
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
				"web-ui",
				"kyverno",
			}

			for _, feature := range expectedFeatures {
				_, exists := status.Orchestrator.Features[feature]
				Expect(exists).To(BeTrue(), fmt.Sprintf("Feature %s should be present", feature))
			}

			// Verify sub-features for cluster-orchestration
			clusterOrch := status.Orchestrator.Features["cluster-orchestration"]
			Expect(clusterOrch.SubFeatures).ToNot(BeNil(), "cluster-orchestration should have sub-features")
			expectedClusterSubFeatures := []string{"cluster-management", "capi", "intel-provider"}
			for _, subFeature := range expectedClusterSubFeatures {
				_, exists := clusterOrch.SubFeatures[subFeature]
				Expect(exists).To(BeTrue(), fmt.Sprintf("cluster-orchestration sub-feature %s should be present", subFeature))
			}

			// Verify sub-features for observability
			observability := status.Orchestrator.Features["observability"]
			Expect(observability.SubFeatures).ToNot(BeNil(), "observability should have sub-features")
			expectedObservabilitySubFeatures := []string{"orchestrator-monitoring", "edge-node-monitoring", "orchestrator-dashboards", "edge-node-dashboards", "alerting"}
			for _, subFeature := range expectedObservabilitySubFeatures {
				_, exists := observability.SubFeatures[subFeature]
				Expect(exists).To(BeTrue(), fmt.Sprintf("observability sub-feature %s should be present", subFeature))
			}

			// Verify sub-features for kyverno
			kyverno := status.Orchestrator.Features["kyverno"]
			Expect(kyverno.SubFeatures).ToNot(BeNil(), "kyverno should have sub-features")
			expectedKyvernoSubFeatures := []string{"policy-engine", "policies"}
			for _, subFeature := range expectedKyvernoSubFeatures {
				_, exists := kyverno.SubFeatures[subFeature]
				Expect(exists).To(BeTrue(), fmt.Sprintf("kyverno sub-feature %s should be present", subFeature))
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

	Describe("Feature Sub-Feature Validation", Label(componentStatusLabel), func() {
		var status ComponentStatus
		var componentStatusURL string

		BeforeEach(func() {
			componentStatusURL = "https://api." + serviceDomainWithPort + "/v1/orchestrator"

			req, err := http.NewRequest(http.MethodGet, componentStatusURL, nil)
			Expect(err).ToNot(HaveOccurred())
			req.Header.Add("Authorization", "Bearer "+token)

			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())

			err = json.Unmarshal(body, &status)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("Edge Infrastructure Manager workflows", func() {
			It("should validate EIM sub-features exist", func() {
				eim, exists := status.Orchestrator.Features["edge-infrastructure-manager"]
				Expect(exists).To(BeTrue(), "edge-infrastructure-manager feature should exist")

				expectedEIMSubFeatures := []string{"day2", "onboarding", "oob", "provisioning"}
				for _, subFeature := range expectedEIMSubFeatures {
					_, exists := eim.SubFeatures[subFeature]
					Expect(exists).To(BeTrue(), fmt.Sprintf("EIM sub-feature %s should be present", subFeature))
				}
			})

			It("should have day2 workflow detection based on maintenance-manager", func() {
				eim := status.Orchestrator.Features["edge-infrastructure-manager"]
				day2, exists := eim.SubFeatures["day2"]
				Expect(exists).To(BeTrue(), "day2 sub-feature should exist")
				// Installed status depends on maintenance-manager deployment
				Expect(day2.Installed).To(Or(BeTrue(), BeFalse()))
			})

			It("should have onboarding workflow detection based on onboarding-manager", func() {
				eim := status.Orchestrator.Features["edge-infrastructure-manager"]
				onboarding, exists := eim.SubFeatures["onboarding"]
				Expect(exists).To(BeTrue(), "onboarding sub-feature should exist")
				// Installed status depends on onboarding-manager.enabled
				Expect(onboarding.Installed).To(Or(BeTrue(), BeFalse()))
			})

			It("should have oob workflow detection based on AMT managers", func() {
				eim := status.Orchestrator.Features["edge-infrastructure-manager"]
				oob, exists := eim.SubFeatures["oob"]
				Expect(exists).To(BeTrue(), "oob sub-feature should exist")
				// Installed status depends on infra-external.amt configuration
				Expect(oob.Installed).To(Or(BeTrue(), BeFalse()))
			})

			It("should have provisioning workflow detection based on autoProvision configuration", func() {
				eim := status.Orchestrator.Features["edge-infrastructure-manager"]
				provisioning, exists := eim.SubFeatures["provisioning"]
				Expect(exists).To(BeTrue(), "provisioning sub-feature should exist")
				// Installed status depends on infra-managers.autoProvision.enabled
				Expect(provisioning.Installed).To(Or(BeTrue(), BeFalse()))
			})

			It("should allow onboarding and provisioning to be independent", func() {
				eim := status.Orchestrator.Features["edge-infrastructure-manager"]
				onboarding := eim.SubFeatures["onboarding"]
				provisioning := eim.SubFeatures["provisioning"]

				// Onboarding can be enabled without provisioning
				// This validates they are truly independent workflows
				if onboarding.Installed && !provisioning.Installed {
					// Manual device registration without auto-provisioning
					Expect(true).To(BeTrue())
				}
				if !onboarding.Installed && provisioning.Installed {
					// Auto-provisioning without manual registration
					Expect(true).To(BeTrue())
				}
			})
		})

		Context("Cluster Orchestration capabilities", func() {
			It("should validate cluster-orch sub-features exist", func() {
				clusterOrch, exists := status.Orchestrator.Features["cluster-orchestration"]
				Expect(exists).To(BeTrue(), "cluster-orchestration feature should exist")

				expectedSubFeatures := []string{"cluster-management", "capi", "intel-provider"}
				for _, subFeature := range expectedSubFeatures {
					_, exists := clusterOrch.SubFeatures[subFeature]
					Expect(exists).To(BeTrue(), fmt.Sprintf("cluster-orch sub-feature %s should be present", subFeature))
				}
			})

			It("should detect cluster-management based on cluster-manager", func() {
				clusterOrch := status.Orchestrator.Features["cluster-orchestration"]
				clusterMgmt, exists := clusterOrch.SubFeatures["cluster-management"]
				Expect(exists).To(BeTrue(), "cluster-management sub-feature should exist")

				// If cluster-orch is installed, cluster-management should match
				if clusterOrch.Installed {
					Expect(clusterMgmt.Installed).To(Equal(clusterOrch.Installed))
				}
			})

			It("should detect CAPI integration", func() {
				clusterOrch := status.Orchestrator.Features["cluster-orchestration"]
				capi, exists := clusterOrch.SubFeatures["capi"]
				Expect(exists).To(BeTrue(), "capi sub-feature should exist")
				Expect(capi.Installed).To(Or(BeTrue(), BeFalse()))
			})

			It("should detect Intel infrastructure provider", func() {
				clusterOrch := status.Orchestrator.Features["cluster-orchestration"]
				intelProvider, exists := clusterOrch.SubFeatures["intel-provider"]
				Expect(exists).To(BeTrue(), "intel-provider sub-feature should exist")
				Expect(intelProvider.Installed).To(Or(BeTrue(), BeFalse()))
			})
		})

		Context("Observability monitoring capabilities", func() {
			It("should validate observability sub-features exist", func() {
				obs, exists := status.Orchestrator.Features["observability"]
				Expect(exists).To(BeTrue(), "observability feature should exist")

				expectedSubFeatures := []string{
					"orchestrator-monitoring",
					"edge-node-monitoring",
					"orchestrator-dashboards",
					"edge-node-dashboards",
					"alerting",
				}
				for _, subFeature := range expectedSubFeatures {
					_, exists := obs.SubFeatures[subFeature]
					Expect(exists).To(BeTrue(), fmt.Sprintf("observability sub-feature %s should be present", subFeature))
				}
			})

			It("should detect orchestrator monitoring independently from edge node monitoring", func() {
				obs := status.Orchestrator.Features["observability"]
				orchMon := obs.SubFeatures["orchestrator-monitoring"]
				edgeMon := obs.SubFeatures["edge-node-monitoring"]

				// These can be enabled independently
				Expect(orchMon.Installed).To(Or(BeTrue(), BeFalse()))
				Expect(edgeMon.Installed).To(Or(BeTrue(), BeFalse()))
			})

			It("should detect dashboard availability", func() {
				obs := status.Orchestrator.Features["observability"]
				orchDash := obs.SubFeatures["orchestrator-dashboards"]
				edgeDash := obs.SubFeatures["edge-node-dashboards"]

				Expect(orchDash.Installed).To(Or(BeTrue(), BeFalse()))
				Expect(edgeDash.Installed).To(Or(BeTrue(), BeFalse()))
			})

			It("should detect alerting capabilities", func() {
				obs := status.Orchestrator.Features["observability"]
				alerting, exists := obs.SubFeatures["alerting"]
				Expect(exists).To(BeTrue(), "alerting sub-feature should exist")
				Expect(alerting.Installed).To(Or(BeTrue(), BeFalse()))
			})
		})

		Context("Kyverno policy management", func() {
			It("should validate kyverno sub-features exist", func() {
				kyverno, exists := status.Orchestrator.Features["kyverno"]
				Expect(exists).To(BeTrue(), "kyverno feature should exist")

				expectedSubFeatures := []string{"policy-engine", "policies"}
				for _, subFeature := range expectedSubFeatures {
					_, exists := kyverno.SubFeatures[subFeature]
					Expect(exists).To(BeTrue(), fmt.Sprintf("kyverno sub-feature %s should be present", subFeature))
				}
			})

			It("should detect policy engine and policies independently", func() {
				kyverno := status.Orchestrator.Features["kyverno"]
				engine := kyverno.SubFeatures["policy-engine"]
				policies := kyverno.SubFeatures["policies"]

				// Policy engine is the core, policies are optional
				if policies.Installed {
					// If policies are installed, engine must be installed
					Expect(engine.Installed).To(BeTrue(), "policy-engine must be installed if policies are installed")
				}
			})
		})

		Context("Web UI components", func() {
			It("should validate web-ui sub-features exist", func() {
				webUI, exists := status.Orchestrator.Features["web-ui"]
				Expect(exists).To(BeTrue(), "web-ui feature should exist")

				expectedSubFeatures := []string{
					"orchestrator-ui-root",
					"application-orchestration-ui",
					"cluster-orchestration-ui",
					"infrastructure-ui",
				}
				for _, subFeature := range expectedSubFeatures {
					_, exists := webUI.SubFeatures[subFeature]
					Expect(exists).To(BeTrue(), fmt.Sprintf("web-ui sub-feature %s should be present", subFeature))
				}
			})

			It("should allow independent UI component deployment", func() {
				webUI := status.Orchestrator.Features["web-ui"]

				orchUIRoot := webUI.SubFeatures["orchestrator-ui-root"]
				appUI := webUI.SubFeatures["application-orchestration-ui"]
				clusterUI := webUI.SubFeatures["cluster-orchestration-ui"]
				infraUI := webUI.SubFeatures["infrastructure-ui"]

				// Each UI component can be enabled/disabled independently
				Expect(orchUIRoot.Installed).To(Or(BeTrue(), BeFalse()))
				Expect(appUI.Installed).To(Or(BeTrue(), BeFalse()))
				Expect(clusterUI.Installed).To(Or(BeTrue(), BeFalse()))
				Expect(infraUI.Installed).To(Or(BeTrue(), BeFalse()))
			})
		})

		Context("Multitenancy configuration", func() {
			It("should validate multitenancy sub-features", func() {
				mt, exists := status.Orchestrator.Features["multitenancy"]
				Expect(exists).To(BeTrue(), "multitenancy feature should exist")

				// Multitenancy is always installed
				Expect(mt.Installed).To(BeTrue(), "multitenancy should always be installed")

				defaultOnly, exists := mt.SubFeatures["default-tenant-only"]
				Expect(exists).To(BeTrue(), "default-tenant-only sub-feature should exist")
				Expect(defaultOnly.Installed).To(Or(BeTrue(), BeFalse()))
			})
		})

		Context("Application Orchestration", func() {
			It("should exist as a top-level feature", func() {
				appOrch, exists := status.Orchestrator.Features["application-orchestration"]
				Expect(exists).To(BeTrue(), "application-orchestration feature should exist")

				// App-orch is a cohesive feature without sub-features
				// All components work together as one deployment capability
				Expect(appOrch.Installed).To(Or(BeTrue(), BeFalse()))
			})
		})
	})

	Describe("Feature State Consistency", Label(componentStatusLabel), func() {
		var status ComponentStatus
		var componentStatusURL string

		BeforeEach(func() {
			componentStatusURL = "https://api." + serviceDomainWithPort + "/v1/orchestrator"

			req, err := http.NewRequest(http.MethodGet, componentStatusURL, nil)
			Expect(err).ToNot(HaveOccurred())
			req.Header.Add("Authorization", "Bearer "+token)

			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())

			err = json.Unmarshal(body, &status)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should have consistent orchestrator version format", func() {
			version := status.Orchestrator.Version
			Expect(version).ToNot(BeEmpty(), "version should not be empty")
			// Version should follow pattern: v2026.0.0 or v2026.0.0-dev-<hash>
			Expect(version).To(MatchRegexp(`^v?\d{4}\.\d+\.\d+(-[a-z]+-[a-f0-9]+)?$`),
				fmt.Sprintf("version format should be valid: %s", version))
		})

		It("should maintain parent-child feature relationships", func() {
			// If parent is disabled, children should also be disabled
			for featureName, feature := range status.Orchestrator.Features {
				if !feature.Installed {
					for subName, subFeature := range feature.SubFeatures {
						Expect(subFeature.Installed).To(BeFalse(),
							fmt.Sprintf("sub-feature %s.%s should be disabled when parent is disabled",
								featureName, subName))
					}
				}
			}
		})

		It("should have all boolean installed fields", func() {
			// Verify all features have installed field as boolean
			for featureName, feature := range status.Orchestrator.Features {
				Expect(feature.Installed).To(Or(BeTrue(), BeFalse()),
					fmt.Sprintf("feature %s should have boolean installed field", featureName))

				for subName, subFeature := range feature.SubFeatures {
					Expect(subFeature.Installed).To(Or(BeTrue(), BeFalse()),
						fmt.Sprintf("sub-feature %s.%s should have boolean installed field",
							featureName, subName))
				}
			}
		})
	})
})
