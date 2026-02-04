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
	componentStatusLabel        = "component-status"
	orchestratorIntegrationLabel = "orchestrator-integration"
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
	Installed bool `json:"installed"`
	// SubFeatures is populated by custom UnmarshalJSON - captures all fields except "installed"
	SubFeatures map[string]Feature `json:"-"`
}

// UnmarshalJSON custom unmarshaler to handle inline sub-features
func (f *Feature) UnmarshalJSON(data []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if installed, ok := raw["installed"].(bool); ok {
		f.Installed = installed
	}

	f.SubFeatures = make(map[string]Feature)

	for key, value := range raw {
		if key == "installed" {
			continue
		}

		// Marshal the sub-feature back to JSON and unmarshal into Feature struct
		subFeatureJSON, err := json.Marshal(value)
		if err != nil {
			continue
		}

		var subFeature Feature
		if err := json.Unmarshal(subFeatureJSON, &subFeature); err != nil {
			continue
		}

		f.SubFeatures[key] = subFeature
	}

	return nil
}

var _ = Describe("Component Status Service", Label(componentStatusLabel, orchestratorIntegrationLabel), func() {
	var cli *http.Client
	var token string

	BeforeEach(func() {
		cli = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		}
		fmt.Printf("serviceDomain: %v\n", serviceDomain)
		// Get authentication token - component status endpoint requires valid JWT
		// Use all-groups-example-user which is created in both KIND and OnPrem deployments
		token = getKeycloakJWT(cli, "all-groups-example-user")
	})

	Describe("Component Status API", Label(componentStatusLabel), func() {
		componentStatusURL := "https://api." + serviceDomainWithPort + "/v1/orchestrator"

		It("should be accessible over HTTPS with valid authentication", func() {
			req, err := http.NewRequest(http.MethodGet, componentStatusURL, nil)
			Expect(err).ToNot(HaveOccurred())
			req.Header.Add("Authorization", "Bearer "+token)

			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		It("should return valid JSON with correct schema", func() {
			req, err := http.NewRequest(http.MethodGet, componentStatusURL, nil)
			Expect(err).ToNot(HaveOccurred())
			req.Header.Add("Authorization", "Bearer "+token)

			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
		defer func() { _ = resp.Body.Close() }()

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

		It("should have 'installed' boolean field for all features and sub-features", func() {
			req, err := http.NewRequest(http.MethodGet, componentStatusURL, nil)
			Expect(err).ToNot(HaveOccurred())
			req.Header.Add("Authorization", "Bearer "+token)

			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()

			body, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())

			// Parse as raw map to verify structure
			var rawStatus map[string]interface{}
			err = json.Unmarshal(body, &rawStatus)
			Expect(err).ToNot(HaveOccurred())

			orchestrator := rawStatus["orchestrator"].(map[string]interface{})
			features := orchestrator["features"].(map[string]interface{})

			// Verify each top-level feature has 'installed' field
			for featureName, featureValue := range features {
				feature := featureValue.(map[string]interface{})
				installed, exists := feature["installed"]
				Expect(exists).To(BeTrue(), fmt.Sprintf("Feature %s should have 'installed' field", featureName))
				_, isBool := installed.(bool)
				Expect(isBool).To(BeTrue(), fmt.Sprintf("Feature %s 'installed' should be boolean", featureName))

				// Check sub-features
				for subName, subValue := range feature {
					if subName == "installed" {
						continue
					}
					subFeature := subValue.(map[string]interface{})
					subInstalled, subExists := subFeature["installed"]
					Expect(subExists).To(BeTrue(), fmt.Sprintf("Sub-feature %s.%s should have 'installed' field", featureName, subName))
					_, subIsBool := subInstalled.(bool)
					Expect(subIsBool).To(BeTrue(), fmt.Sprintf("Sub-feature %s.%s 'installed' should be boolean", featureName, subName))
				}
			}
		})

		It("should return expected feature flags", func() {
			req, err := http.NewRequest(http.MethodGet, componentStatusURL, nil)
			Expect(err).ToNot(HaveOccurred())
			req.Header.Add("Authorization", "Bearer "+token)

			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()

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
				"orchestrator-observability",
				"edgenode-observability",
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

			// Verify sub-features for orchestrator-observability
			orchestratorObs := status.Orchestrator.Features["orchestrator-observability"]
			Expect(orchestratorObs.SubFeatures).ToNot(BeNil(), "orchestrator-observability should have sub-features")
			expectedOrchestratorObsSubFeatures := []string{"monitoring", "dashboards", "alerting"}
			for _, subFeature := range expectedOrchestratorObsSubFeatures {
				_, exists := orchestratorObs.SubFeatures[subFeature]
				Expect(exists).To(BeTrue(), fmt.Sprintf("orchestrator-observability sub-feature %s should be present", subFeature))
			}

			// Verify sub-features for edgenode-observability
			edgeNodeObs := status.Orchestrator.Features["edgenode-observability"]
			Expect(edgeNodeObs.SubFeatures).ToNot(BeNil(), "edgenode-observability should have sub-features")
			expectedEdgeNodeObsSubFeatures := []string{"monitoring", "dashboards"}
			for _, subFeature := range expectedEdgeNodeObsSubFeatures {
				_, exists := edgeNodeObs.SubFeatures[subFeature]
				Expect(exists).To(BeTrue(), fmt.Sprintf("edgenode-observability sub-feature %s should be present", subFeature))
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
			defer func() { _ = resp.Body.Close() }()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(resp.Header.Get("Content-Type")).To(ContainSubstring("application/json"))
		})

		It("should return 404 for non-existent paths", func() {
			req, err := http.NewRequest(http.MethodGet, "https://api."+serviceDomainWithPort+"/v1/orchestrator/nonexistent", nil)
			Expect(err).ToNot(HaveOccurred())
			req.Header.Add("Authorization", "Bearer "+token)

			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()

			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})

		It("should support GET method only", func() {
			// Test POST should fail
			req, err := http.NewRequest(http.MethodPost, componentStatusURL, nil)
			Expect(err).ToNot(HaveOccurred())
			req.Header.Add("Authorization", "Bearer "+token)

			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()

			Expect(resp.StatusCode).To(Equal(http.StatusMethodNotAllowed))
		})

		It("should reject PUT requests", func() {
			req, err := http.NewRequest(http.MethodPut, componentStatusURL, nil)
			Expect(err).ToNot(HaveOccurred())
			req.Header.Add("Authorization", "Bearer "+token)

			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()

			Expect(resp.StatusCode).To(Equal(http.StatusMethodNotAllowed))
		})

		It("should reject DELETE requests", func() {
			req, err := http.NewRequest(http.MethodDelete, componentStatusURL, nil)
			Expect(err).ToNot(HaveOccurred())
			req.Header.Add("Authorization", "Bearer "+token)

			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()

			Expect(resp.StatusCode).To(Equal(http.StatusMethodNotAllowed))
		})
	})

	Describe("Authentication and Security", Label(componentStatusLabel), func() {
		componentStatusURL := "https://api." + serviceDomainWithPort + "/v1/orchestrator"

		It("should reject requests without authentication token", func() {
			req, err := http.NewRequest(http.MethodGet, componentStatusURL, nil)
			Expect(err).ToNot(HaveOccurred())
			// No Authorization header

			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()

			// Expect 403 Forbidden (JWT validation middleware returns 403 for missing/invalid token)
			Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
		})

		It("should reject requests with invalid authentication token", func() {
			req, err := http.NewRequest(http.MethodGet, componentStatusURL, nil)
			Expect(err).ToNot(HaveOccurred())
			req.Header.Add("Authorization", "Bearer invalid-token-12345")

			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()

			// Expect 403 Forbidden (JWT validation middleware returns 403 for invalid token)
			Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
		})

		It("should return identical responses for multiple requests (idempotency)", func() {
			// Make first request
			req1, err := http.NewRequest(http.MethodGet, componentStatusURL, nil)
			Expect(err).ToNot(HaveOccurred())
			req1.Header.Add("Authorization", "Bearer "+token)

			resp1, err := cli.Do(req1)
			Expect(err).ToNot(HaveOccurred())
			defer func() { _ = resp1.Body.Close() }()

			body1, err := io.ReadAll(resp1.Body)
			Expect(err).ToNot(HaveOccurred())

			// Make second request
			req2, err := http.NewRequest(http.MethodGet, componentStatusURL, nil)
			Expect(err).ToNot(HaveOccurred())
			req2.Header.Add("Authorization", "Bearer "+token)

			resp2, err := cli.Do(req2)
			Expect(err).ToNot(HaveOccurred())
			defer func() { _ = resp2.Body.Close() }()

			body2, err := io.ReadAll(resp2.Body)
			Expect(err).ToNot(HaveOccurred())

			// Responses should be identical (service returns static config)
			Expect(body1).To(Equal(body2), "component status should return identical responses")
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
			defer func() { _ = resp.Body.Close() }()

			body, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())

			err = json.Unmarshal(body, &status)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("Edge Infrastructure Manager workflows", func() {
			It("should validate EIM sub-features exist", func() {
				eim, exists := status.Orchestrator.Features["edge-infrastructure-manager"]
				Expect(exists).To(BeTrue(), "edge-infrastructure-manager feature should exist")

				expectedEIMSubFeatures := []string{"oxm-profile", "day2", "onboarding", "oob", "provisioning"}
				for _, subFeature := range expectedEIMSubFeatures {
					_, exists := eim.SubFeatures[subFeature]
					Expect(exists).To(BeTrue(), fmt.Sprintf("EIM sub-feature %s should be present", subFeature))
				}
			})


			It("should have oxm-profile detection for microvisor-based deployments", func() {
				eim := status.Orchestrator.Features["edge-infrastructure-manager"]
				oxmProfile, exists := eim.SubFeatures["oxm-profile"]
				Expect(exists).To(BeTrue(), "oxm-profile sub-feature should exist")
				// OXM profile only on on-prem (metallb enabled) with pxe-server enabled
				// When OXM is enabled, OOB should be disabled (mutually exclusive)
				// For non-onprem deployments (AWS, Kind), OXM will always be false
				if oxmProfile.Installed {
					oob := eim.SubFeatures["oob"]
					Expect(oob.Installed).To(BeFalse(), "OXM and OOB are mutually exclusive - OXM uses microvisor, not vPRO/AMT")
				}
			})

			It("should validate EIM workflow sub-features have correct structure", func() {
				eim := status.Orchestrator.Features["edge-infrastructure-manager"]
				
				// Verify all workflow sub-features exist
				day2, day2Exists := eim.SubFeatures["day2"]
				Expect(day2Exists).To(BeTrue(), "day2 sub-feature should exist")
				
				onboarding, onboardingExists := eim.SubFeatures["onboarding"]
				Expect(onboardingExists).To(BeTrue(), "onboarding sub-feature should exist")
				
				oob, oobExists := eim.SubFeatures["oob"]
				Expect(oobExists).To(BeTrue(), "oob sub-feature should exist")
				
				provisioning, provExists := eim.SubFeatures["provisioning"]
				Expect(provExists).To(BeTrue(), "provisioning sub-feature should exist")
				
				// If EIM is installed, at least one workflow should be enabled
				if eim.Installed {
					Expect(day2.Installed || onboarding.Installed || oob.Installed || provisioning.Installed).To(BeTrue(),
						"if EIM is installed, at least one workflow should be enabled")
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

				// Parent should be enabled if ANY child is enabled
				capi := clusterOrch.SubFeatures["capi"]
				intelProvider := clusterOrch.SubFeatures["intel-provider"]
				if clusterMgmt.Installed || capi.Installed || intelProvider.Installed {
					Expect(clusterOrch.Installed).To(BeTrue(), "parent should be enabled if any child is enabled")
				}
			})

			It("should have all cluster-orchestration sub-features with proper structure", func() {
				clusterOrch := status.Orchestrator.Features["cluster-orchestration"]
				
				// Verify all expected sub-features exist
				capi, capiExists := clusterOrch.SubFeatures["capi"]
				Expect(capiExists).To(BeTrue(), "capi sub-feature should exist")
				
				intelProvider, intelExists := clusterOrch.SubFeatures["intel-provider"]
				Expect(intelExists).To(BeTrue(), "intel-provider sub-feature should exist")
				
				clusterMgmt := clusterOrch.SubFeatures["cluster-management"]
				
				// If ANY sub-feature is enabled, at least one should be true
				if clusterOrch.Installed {
					Expect(capi.Installed || intelProvider.Installed || clusterMgmt.Installed).To(BeTrue(),
						"if cluster-orchestration is installed, at least one sub-feature should be installed")
				}
			})
		})

		Context("Observability monitoring capabilities", func() {
			It("should validate orchestrator-observability exists as separate top-level feature", func() {
				orchObs, exists := status.Orchestrator.Features["orchestrator-observability"]
				Expect(exists).To(BeTrue(), "orchestrator-observability feature should exist")

				expectedSubFeatures := []string{
					"monitoring",
					"dashboards",
					"alerting",
				}
				for _, subFeature := range expectedSubFeatures {
					_, exists := orchObs.SubFeatures[subFeature]
					Expect(exists).To(BeTrue(), fmt.Sprintf("orchestrator-observability sub-feature %s should be present", subFeature))
				}
			})

			It("should validate edgenode-observability exists as separate top-level feature", func() {
				edgeObs, exists := status.Orchestrator.Features["edgenode-observability"]
				Expect(exists).To(BeTrue(), "edgenode-observability feature should exist")

				expectedSubFeatures := []string{
					"monitoring",
					"dashboards",
				}
				for _, subFeature := range expectedSubFeatures {
					_, exists := edgeObs.SubFeatures[subFeature]
					Expect(exists).To(BeTrue(), fmt.Sprintf("edgenode-observability sub-feature %s should be present", subFeature))
				}
			})

			It("should maintain proper observability feature independence", func() {
				orchObs, orchExists := status.Orchestrator.Features["orchestrator-observability"]
				edgeObs, edgeExists := status.Orchestrator.Features["edgenode-observability"]

				Expect(orchExists).To(BeTrue(), "orchestrator-observability should exist")
				Expect(edgeExists).To(BeTrue(), "edgenode-observability should exist")

				// These are independent - can be enabled separately
				// Verify that edgenode doesn't have alerting (only orchestrator has it)
				_, edgeAlertingExists := edgeObs.SubFeatures["alerting"]
				Expect(edgeAlertingExists).To(BeFalse(), "edgenode-observability should not have alerting sub-feature")
				
				// Verify orchestrator has alerting sub-feature
				_, orchAlertingExists := orchObs.SubFeatures["alerting"]
				Expect(orchAlertingExists).To(BeTrue(), "orchestrator-observability should have alerting sub-feature")
			})

			It("should enable orchestrator-observability parent if ANY sub-component is enabled", func() {
				orchObs := status.Orchestrator.Features["orchestrator-observability"]
				monitoring := orchObs.SubFeatures["monitoring"]
				dashboards := orchObs.SubFeatures["dashboards"]
				alerting := orchObs.SubFeatures["alerting"]

				// Parent should be enabled if ANY child is enabled
				if monitoring.Installed || dashboards.Installed || alerting.Installed {
					Expect(orchObs.Installed).To(BeTrue(), "parent should be enabled if any child is enabled")
				}
			})

			It("should enable edgenode-observability parent if ANY sub-component is enabled", func() {
				edgeObs := status.Orchestrator.Features["edgenode-observability"]
				monitoring := edgeObs.SubFeatures["monitoring"]
				dashboards := edgeObs.SubFeatures["dashboards"]

				// Parent should be enabled if ANY child is enabled
				if monitoring.Installed || dashboards.Installed {
					Expect(edgeObs.Installed).To(BeTrue(), "parent should be enabled if any child is enabled")
				}
			})
		})

		Context("Kyverno policy enforcement", func() {
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
                                        "admin-ui",
                                        "alerts-ui",
                                }
                                for _, subFeature := range expectedSubFeatures {
                                        _, exists := webUI.SubFeatures[subFeature]
                                        Expect(exists).To(BeTrue(), fmt.Sprintf("web-ui sub-feature %s should be present", subFeature))
                                }
                        })

                        It("should disable UI when backend is disabled", func() {
                                webUI := status.Orchestrator.Features["web-ui"]
                                appOrch := status.Orchestrator.Features["application-orchestration"]
                                clusterOrch := status.Orchestrator.Features["cluster-orchestration"]
                                infra := status.Orchestrator.Features["edge-infrastructure-manager"]
                                orchObs, orchObsExists := status.Orchestrator.Features["orchestrator-observability"]

                                appUI := webUI.SubFeatures["application-orchestration-ui"]
                                clusterUI := webUI.SubFeatures["cluster-orchestration-ui"]
                                infraUI := webUI.SubFeatures["infrastructure-ui"]
                                alertsUI, alertsUIExists := webUI.SubFeatures["alerts-ui"]

                                // If backend is disabled, UI must also be disabled
                                if !appOrch.Installed {
                                        Expect(appUI.Installed).To(BeFalse(), "app-orch-ui requires app-orch backend")
                                }
                                if !clusterOrch.Installed {
                                        Expect(clusterUI.Installed).To(BeFalse(), "cluster-orch-ui requires cluster-orch backend")
                                }
                                if !infra.Installed {
                                        Expect(infraUI.Installed).To(BeFalse(), "infra-ui requires infra backend")
                                }
                                if alertsUIExists && orchObsExists && !orchObs.Installed {
                                        Expect(alertsUI.Installed).To(BeFalse(), "alerts-ui requires orchestrator-observability backend")
                                }
                        })

                        It("should enable UI when backend is enabled", func() {
                                webUI := status.Orchestrator.Features["web-ui"]
                                appOrch := status.Orchestrator.Features["application-orchestration"]
                                clusterOrch := status.Orchestrator.Features["cluster-orchestration"]
                                infra := status.Orchestrator.Features["edge-infrastructure-manager"]
                                orchObs := status.Orchestrator.Features["orchestrator-observability"]

                                appUI := webUI.SubFeatures["application-orchestration-ui"]
                                clusterUI := webUI.SubFeatures["cluster-orchestration-ui"]
                                infraUI := webUI.SubFeatures["infrastructure-ui"]
                                alertsUI, alertsUIExists := webUI.SubFeatures["alerts-ui"]

                                // If UI is enabled, backend must be enabled (reverse dependency)
                                if appUI.Installed {
                                        Expect(appOrch.Installed).To(BeTrue(), "app-orch backend must be enabled if UI is enabled")
                                }
                                if clusterUI.Installed {
                                        Expect(clusterOrch.Installed).To(BeTrue(), "cluster-orch backend must be enabled if UI is enabled")
                                }
                                if infraUI.Installed {
                                        Expect(infra.Installed).To(BeTrue(), "infra backend must be enabled if UI is enabled")
                                }
                                if alertsUIExists && alertsUI.Installed {
                                        Expect(orchObs.Installed).To(BeTrue(), "orchestrator-observability must be enabled if alerts-ui is enabled")
                                }
                        })
                })

                Context("Multitenancy configuration", func() {
                        It("should validate multitenancy sub-features", func() {
                                mt, exists := status.Orchestrator.Features["multitenancy"]
                                Expect(exists).To(BeTrue(), "multitenancy feature should exist")

                                // Multitenancy is always installed
				defaultOnly, exists := mt.SubFeatures["default-tenant-only"]
				Expect(exists).To(BeTrue(), "default-tenant-only sub-feature should exist")
				Expect(defaultOnly.Installed).To(Or(BeTrue(), BeFalse()))
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
			defer func() { _ = resp.Body.Close() }()

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
        })
})
