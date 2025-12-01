// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	util "github.com/open-edge-platform/edge-manageability-framework/mage"
	catalogloader "github.com/open-edge-platform/orch-library/go/pkg/loader"
)

var (
	accessToken string
	_           = Describe("Application Catalog integration test", Label("orchestrator-integration"), func() {
		var c *http.Client
		var cancel context.CancelFunc
		var ctx context.Context

		testPassword := func() string {
			pass, err := util.GetDefaultOrchPassword()
			if err != nil {
				log.Fatal(err)
			}
			return pass
		}()

		BeforeEach(func() {
			c = &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: tlsConfig,
				},
			}
			ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		})

		AfterEach(func() {
			cancel()
		})

		Describe("Catalog service smoke test", Ordered, func() {
			When("Connecting to application catalog service without a token", func() {
				It("should NOT be accessible over HTTPS when token is not specified", func() {
					req, err := http.NewRequest("GET",
						ConstructCatalogURL(testProject, "registries"),
						nil)
					Expect(err).ToNot(HaveOccurred())
					resp, err := c.Do(req)
					Expect(err).ToNot(HaveOccurred())
					defer resp.Body.Close()
					Expect(resp.StatusCode).To(Equal(http.StatusForbidden), func() string {
						b, err := io.ReadAll(resp.Body)
						Expect(err).ToNot(HaveOccurred())
						return fmt.Sprintf("error on GET %s %s",
							ConstructCatalogURL(testProject, "registries"), string(b))
					})
					content, err := io.ReadAll(resp.Body)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(content)).To(ContainSubstring(`named cookie not present`))
				})
			})
			When("Using the catalog REST API", Ordered, func() {
				const regName = "testreg1"
				const artName = "testart1"
				It("should Get a token from KeyCloak", func() {
					accessToken = getAccessToken(c, testUsername, testPassword)
					Expect(accessToken).To(Not(ContainSubstring(`named cookie not present`)))
				})

				It("should ensure no registry is leftover from a previous run", func() {
					// Delete registry
					DeleteRegistry(ctx, c, accessToken, testProject, regName, ignoreRESTResponse)
					GetRegistry(ctx, c, accessToken, testProject, regName, http.StatusNotFound, checkRESTResponse)
				})

				It("should create a registry", func() {
					CreateRegistry(ctx, c, accessToken, testProject, Registry{
						Name:        regName,
						RootURL:     "http://a.b.c",
						DisplayName: "Registry 1",
						Description: "First test registry",
						Username:    "user",
						AuthToken:   "token",
						Type:        "HELM",
						Cacerts:     "CA",
					})
				})

				It("should determine that the new registry was created", func() {
					reg := GetRegistry(ctx, c, accessToken, testProject, regName, http.StatusOK, checkRESTResponse)
					Expect(reg.RootURL).To(Equal("http://a.b.c"))
					Expect(reg.DisplayName).To(Equal("Registry 1"))
					Expect(reg.Description).To(Equal("First test registry"))
					Expect(reg.Username).To(Equal("user"))
					Expect(reg.AuthToken).To(Equal("token"))
					Expect(reg.Type).To(Equal("HELM"))
					Expect(reg.Cacerts).To(Equal("CA"))
				})

				It("should ensure no artifact is leftover from a previous run", func() {
					DeleteArtifact(ctx, c, accessToken, testProject, artName, ignoreRESTResponse)
					GetArtifact(ctx, c, accessToken, testProject, artName, http.StatusNotFound, checkRESTResponse)
				})

				It("should create an artifact that has no malware and verify the artifact exists", func() {
					if serviceDomain == "offline.lab" {
						Skip("Skipping malware test for offline deployment")
					}
					resp := CreateArtifact(
						ctx,
						c,
						accessToken,
						testProject,
						artName,
						artName,
						"text/plain",
						[]byte("safe_contents"),
						http.StatusOK,
					)
					Expect(resp.StatusCode).To(Equal(http.StatusOK))
					GetArtifact(ctx, c, accessToken, testProject, artName, http.StatusOK, checkRESTResponse)
				})

				It("should delete the good artifact and make sure the artifact is gone", func() {
					DeleteArtifact(ctx, c, accessToken, testProject, artName, checkRESTResponse)
					GetArtifact(ctx, c, accessToken, testProject, artName, http.StatusNotFound, checkRESTResponse)
				})

				// superficial change

				It("should delete the registry and make sure all registries are gone", func() {
					DeleteRegistry(ctx, c, accessToken, testProject, regName, checkRESTResponse)
					GetRegistry(ctx, c, accessToken, testProject, regName, http.StatusNotFound, checkRESTResponse)
				})
			})
		})

		Describe("Catalog service YAML import test", Ordered, func() {
			When("Importing YAML files", func() {
				It("should Get a token from KeyCloak", func() {
					accessToken = getAccessToken(c, testUsername, testPassword)
					Expect(accessToken).To(Not(ContainSubstring(`named cookie not present`)))
				})
				It("should import the samples", func() {
					loader := catalogloader.NewLoader(apiBaseURL, testProject)
					loadFiles := []string{"../samples/00-common", "../samples/10-applications", "../samples/20-deployment-packages"}
					err := loader.LoadResources(context.Background(), accessToken, loadFiles)
					Expect(err).ToNot(HaveOccurred())
				})
				It("should determine that the new registry was created", func() {
					r := GetRegistry(ctx, c, accessToken, testProject, "bitnami-helm-oci", http.StatusOK, checkRESTResponse)
					Expect(r.Name).To(Equal("bitnami-helm-oci"))
					Expect(r.Description).To(Equal("Bitnami helm registry"))
					Expect(r.Type).To(Equal("HELM"))
					Expect(r.RootURL).To(Equal("oci://registry-1.docker.io/bitnamicharts"))
				})
				It("should determine that the new application was created", func() {
					wordpressApp := GetApplication(ctx, c, accessToken, testProject, "wordpress", "0.1.0", http.StatusOK, checkRESTResponse)
					Expect(wordpressApp.Name).To(Equal("wordpress"))
					Expect(wordpressApp.Description).To(Equal("Wordpress"))
					Expect(wordpressApp.ChartVersion).To(Equal("15.2.42"))
					Expect(wordpressApp.Version).To(Equal("0.1.0"))
					Expect(wordpressApp.Profiles).To(HaveLen(1))
					Expect(wordpressApp.Profiles[0].Name).To(Equal("default"))
					Expect(wordpressApp.Profiles[0].ChartValues).To(
						ContainSubstring("wordpressUsername: admin"),
					)
				})
				It("should determine that the deployment package was created", func() {
					dp := GetDeploymentPackage(ctx, c, accessToken, testProject, "wordpress", "0.1.0", http.StatusOK, checkRESTResponse)

					Expect(dp.Version).To(Equal("0.1.0"))
					Expect(dp.Name).To(Equal("wordpress"))
					Expect(dp.DisplayName).To(Equal("My Wordpress Blog"))
					Expect(dp.IsDeployed).To(BeFalse())
					Expect(dp.IsVisible).To(BeFalse())
					Expect(dp.Profiles).To(HaveLen(1))
					Expect(dp.Profiles[0].Name).To(Equal("testing"))
				})
				It("should determine that the artifacts were created", func() {
					GetArtifact(ctx, c, accessToken, testProject, "intel-icon", http.StatusOK, checkRESTResponse)
					GetArtifact(ctx, c, accessToken, testProject, "roc-icon", http.StatusOK, checkRESTResponse)
				})
				// It("should delete the project and make sure it's gone", func() {
				AfterAll(func() {
					type CatalogItem struct {
						projectID string
						version   string
						name      string
					}

					// Delete packages
					pkgs := map[string]CatalogItem{
						"wordpress-0.1.0":                {name: "wordpress", projectID: testProject, version: "0.1.0"},
						"wordpress-0.0.1":                {name: "wordpress", projectID: testProject, version: "0.1.1"},
						"nginx-app-0.1.0":                {name: "nginx-app", projectID: testProject, version: "0.1.0"},
						"nginx-ns-pt-0.1.0":              {name: "nginx-app-ns-pt", projectID: testProject, version: "0.1.0"},
						"wordpress-and-nginx-apps-0.1.0": {name: "wordpress-and-nginx-apps", projectID: testProject, version: "0.1.0"},
						"wordpress-and-nginx-apps-0.1.1": {name: "wordpress-and-nginx-apps", projectID: testProject, version: "0.1.1"},
					}

					for _, pkg := range pkgs {
						DeleteDeploymentPackage(ctx, c, accessToken, pkg.projectID, pkg.name, pkg.version, ignoreRESTResponse)
					}
					for _, deletedPkg := range pkgs {
						GetDeploymentPackage(ctx, c, accessToken, deletedPkg.projectID, deletedPkg.name, deletedPkg.version, http.StatusNotFound, checkRESTResponse)
					}

					// Delete and check applications
					apps := map[string]CatalogItem{
						"nginx-0.1.0":     {projectID: testProject, name: "nginx", version: "0.1.0"},
						"nginx-0.1.1":     {projectID: testProject, name: "nginx", version: "0.1.1"},
						"wordpress-0.1.0": {projectID: testProject, name: "wordpress", version: "0.1.0"},
						"wordpress-0.1.1": {projectID: testProject, name: "wordpress", version: "0.1.1"},
					}
					for _, app := range apps {
						DeleteApplication(ctx, c, accessToken, app.projectID, app.name, app.version, ignoreRESTResponse)
					}
					for _, deletedApp := range apps {
						GetApplication(ctx, c, accessToken, deletedApp.projectID, deletedApp.name, deletedApp.version, http.StatusNotFound, checkRESTResponse)
					}

					// Delete and check Registries
					registries := map[string]CatalogItem{
						"bitnami": {projectID: testProject, name: "bitnami", version: ""},
					}
					for _, reg := range registries {
						DeleteRegistry(ctx, c, accessToken, reg.projectID, reg.name, ignoreRESTResponse)
					}
					for _, deletedReg := range registries {
						GetRegistry(ctx, c, accessToken, deletedReg.projectID, deletedReg.name, http.StatusNotFound, checkRESTResponse)
					}
				})
			})
		})
	})
)
