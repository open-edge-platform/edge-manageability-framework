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
	"strings"
	"time"

	projectv1 "github.com/open-edge-platform/orch-utils/tenancy-datamodel/build/apis/project.edge-orchestrator.intel.com/v1"

	"github.com/bitfield/script"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	util "github.com/open-edge-platform/edge-manageability-framework/mage"
)

const (
	tenancy     = "tenancy"
	tenancyUser = "tenancy-user"
)

var _ = Describe("Tenancy integration test", Label(tenancy), func() {
	var cli *http.Client

	password := func() string {
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
	Describe("Tenancy API services- Token validation", Ordered, Label(tenancy), func() {
		PIt("Tenancy API services should NOT be accessible over HTTPS when using valid but expired token", func() {
			adminPass, err := util.GetKeycloakSecret()
			Expect(err).ToNot(HaveOccurred())
			Expect(saveTokenUser(cli, "admin", adminPass)).To(Succeed())

			jwt, err := script.File(outputFile).String()
			Expect(err).ToNot(HaveOccurred())
			Expect(jwt).ToNot(BeEmpty())

			isUnexpired, err := isTokenUnexpired(jwt)
			Expect(err).ToNot(HaveOccurred())
			if isUnexpired {
				Skip("Skipping this test because JWT Token is NOT expired")
			}

			request, err := http.NewRequest("GET", "https://api."+serviceDomainWithPort+"/v1/orgs", nil)
			Expect(err).ToNot(HaveOccurred())

			// adding JWT to the Authorization header
			request.Header.Add("Authorization", "Bearer "+jwt)

			response, err := cli.Do(request)
			Expect(err).ToNot(HaveOccurred())
			defer response.Body.Close()

			Expect(response.StatusCode).To(Equal(http.StatusForbidden))
		})
		It("should be accessible over HTTPS when using valid token", func() {
			req, err := http.NewRequest("GET", "https://api."+serviceDomainWithPort+"/v1/orgs", nil)
			Expect(err).ToNot(HaveOccurred())
			token := getKeycloakJWT(cli, "admin")
			req.Header.Add("Authorization", "Bearer "+token)
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
			_, err = io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
		})
		It("should NOT be accessible over HTTPS when using no token", func() {
			req, err := http.NewRequest("GET", "https://api."+serviceDomainWithPort+"/v1/orgs", nil)
			Expect(err).ToNot(HaveOccurred())
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
		})
		It("should NOT be accessible over HTTPS when using invalid token", func() {
			req, err := http.NewRequest("GET", "https://api."+serviceDomainWithPort+"/v1/orgs", nil)
			Expect(err).ToNot(HaveOccurred())
			const invalid = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c" //nolint: lll
			req.Header.Add("Authorization", "Bearer "+invalid)
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
		})
		It("should be accessible over HTTPS when using valid token", func() {
			req, err := http.NewRequest(http.MethodGet, "https://api."+serviceDomainWithPort+"/v1/projects/itest", nil)
			Expect(err).ToNot(HaveOccurred())
			token := getKeycloakJWT(cli, "admin")
			req.Header.Add("Authorization", "Bearer "+token)
			req.Header.Add("Content-Type", "application/json")
			resp, err := cli.Do(req)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			_, err = io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("Tenancy API services - CRUD", Ordered, Label(tenancy), func() {
		var (
			// Use a random org and project name to avoid conflicts with other tests.
			orgName  = "tenancy-org-" + generateRandomDigits(10)
			projName = "tenancy-project-" + generateRandomDigits(10)

			orgUID  = ""
			projUID = ""
		)

		It("Validate CRUD Operations", func() {
			err := util.ManageTenancyUserAndRoles(context.Background(), cli, "", "", http.MethodPost, tenancyUser, false)
			Expect(err).ToNot(HaveOccurred())
			logInfo("Create Org '%s' without roles", orgName)
			err = util.ManageTenancyUserAndRoles(context.Background(), cli, "", "", http.MethodDelete, tenancyUser, false)
			Expect(err).ToNot(HaveOccurred())
			token, err := util.GetApiToken(cli, tenancyUser, password)
			Expect(err).ToNot(HaveOccurred())

			orgDesc := []byte(`{ "description":
				"Tenancy Test Organization"
			}`)

			resp, err := makeAuthorizedRequest(http.MethodPut, "https://api."+serviceDomainWithPort+"/v1/orgs/"+orgName, *token, orgDesc, cli)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))

			logInfo("Create Org '%s' using valid roles", orgName)

			err = util.ManageTenancyUserAndRoles(context.Background(), cli, "", "", http.MethodPost, tenancyUser, false)
			Expect(err).ToNot(HaveOccurred())
			token, err = util.GetApiToken(cli, tenancyUser, password)
			Expect(err).ToNot(HaveOccurred())

			resp, err = makeAuthorizedRequest(http.MethodPut, "https://api."+serviceDomainWithPort+"/v1/orgs/"+orgName, *token, orgDesc, cli)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			logInfo("Get All Orgs and check if Org '%s' exists", orgName)
			Eventually(func() ([]Orgs, error) {
				resp, err = makeAuthorizedRequest(http.MethodGet, "https://api."+serviceDomainWithPort+"/v1/orgs", *token, nil, cli)
				if err != nil {
					return nil, fmt.Errorf("failed to get orgs list: %w", err)
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
				}
				orgsList, err := parseOrgsList(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to parse orgs list: %w", err)
				}
				return orgsList, nil
			}, 1*time.Minute, 5*time.Second).Should(
				MatchElements(
					func(element interface{}) string {
						return element.(Orgs).Name
					},
					IgnoreExtras,
					Elements{
						orgName: MatchFields(
							IgnoreExtras,
							Fields{
								"Name": Equal(orgName),
							},
						),
					},
				),
				"Org not found in the list",
			)

			logInfo("Get Specific Org '%s'", orgName)
			Eventually(
				func() error {
					resp, err = makeAuthorizedRequest(http.MethodGet, "https://api."+serviceDomainWithPort+"/v1/orgs/"+orgName, *token, nil, cli)
					if err != nil {
						return fmt.Errorf("Failed to get organization: %s with error: %w", orgName, err)
					}
					defer resp.Body.Close()
					if resp.StatusCode != http.StatusOK {
						return fmt.Errorf("Failed to get organization: %s with StatusCode: %d", orgName, resp.StatusCode)
					}
					org, err := parseOrg(resp.Body)
					if err != nil {
						return fmt.Errorf("failed to parse organization: %s with error: %w", orgName, err)
					}

					if org.Status.OrgStatus.StatusIndicator != "STATUS_INDICATION_IDLE" {
						return fmt.Errorf("organization %s is not active with status: %v", orgName, org.Status)
					}
					orgUID = org.Status.OrgStatus.UID
					if orgUID == "" {
						return fmt.Errorf("orgUID is empty for organization: %s with status: %v", orgName, org.Status)
					}
					return nil
				},
				3*time.Minute,
				15*time.Second,
			).Should(Succeed())

			Expect(orgUID).ToNot(Equal(""))

			logInfo("Create Project '%s' without Org Admin roles", projName)
			projDesc := []byte(`{ "description": "Tenancy Test Project" }`)
			resp, err = makeAuthorizedRequest(http.MethodPut, "https://api."+serviceDomainWithPort+"/v1/projects/"+projName, *token, projDesc, cli)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))

			logInfo("Create Project '%s' with Org Admin roles", projName)

			err = util.ManageTenancyUserAndRoles(context.Background(), cli, orgUID, "", http.MethodPost, tenancyUser, false)
			Expect(err).ToNot(HaveOccurred())
			token, err = util.GetApiToken(cli, tenancyUser, password)
			Expect(err).ToNot(HaveOccurred())

			resp, err = makeAuthorizedRequest(http.MethodPut, "https://api."+serviceDomainWithPort+"/v1/projects/"+projName, *token, projDesc, cli)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			logInfo("Get All Projects and check if Project '%s' exists", projName)
			Eventually(func() ([]Projects, error) {
				resp, err = makeAuthorizedRequest(http.MethodGet, "https://api."+serviceDomainWithPort+"/v1/projects", *token, nil, cli)
				if err != nil {
					return nil, fmt.Errorf("failed to get projects list: %w", err)
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
				}
				projList, err := parseProjectsList(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to parse projects list: %w", err)
				}
				return projList, nil
			}, 20*time.Second, 5*time.Second).Should(
				MatchElements(
					func(element interface{}) string {
						return element.(Projects).Name
					},
					IgnoreExtras,
					Elements{
						projName: MatchFields(
							IgnoreExtras,
							Fields{
								"Name": Equal(projName),
							},
						),
					},
				),
				"Project not found in the list",
			)

			logInfo("Get Project '%s'", projName)
			resp, err = makeAuthorizedRequest(http.MethodGet, "https://api."+serviceDomainWithPort+"/v1/projects/"+projName, *token, nil, cli)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(err).ToNot(HaveOccurred())

			Eventually(
				func() error {
					By("Getting the project UID")
					resp, err = makeAuthorizedRequest(http.MethodGet, "https://api."+serviceDomainWithPort+"/v1/projects/"+projName, *token, nil, cli)
					if err != nil {
						return fmt.Errorf("failed to get Project: %s with error: %w", projName, err)
					}
					defer resp.Body.Close()
					if resp.StatusCode != http.StatusOK {
						return fmt.Errorf("failed to get Project: %s with StatusCode: %d", projName, resp.StatusCode)
					}
					project, err := parseProject(resp.Body)
					if err != nil {
						return fmt.Errorf("failed to parse project: %s with error: %w", projName, err)
					}

					if project.Status.ProjectStatus.StatusIndicator != "STATUS_INDICATION_IDLE" {
						return fmt.Errorf("project %s is not active with status: %v", projName, project.Status)
					}
					projUID = project.Status.ProjectStatus.UID
					if projUID == "" {
						return fmt.Errorf("projUID is empty for project: %s with status: %v ", projName, project.Status)
					}
					return nil
				},
				5*time.Minute,
				15*time.Second,
			).Should(Succeed())

			Expect(projUID).ToNot(Equal(""))

			logInfo("Update Org '%s'", orgName)

			orgDesc = []byte(`{ "description":
				"Tenancy Test Organization - Updated"
			}`)

			resp, err = makeAuthorizedRequest(http.MethodPut, "https://api."+serviceDomainWithPort+"/v1/orgs/"+orgName, *token, orgDesc, cli)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			logInfo("Validate Org '%s' update", orgName)
			Eventually(
				func() error {
					resp, err = makeAuthorizedRequest(http.MethodGet, "https://api."+serviceDomainWithPort+"/v1/orgs/"+orgName, *token, nil, cli)
					if err != nil {
						return fmt.Errorf("Failed to get organization: %s with error: %w", orgName, err)
					}
					defer resp.Body.Close()
					if resp.StatusCode != http.StatusOK {
						return fmt.Errorf("Failed to get organization: %s with StatusCode: %d", orgName, resp.StatusCode)
					}
					org, err := parseOrg(resp.Body)
					if err != nil {
						return fmt.Errorf("failed to parse organization: %s with error: %w", orgName, err)
					}

					if org.Spec.Description != "Tenancy Test Organization - Updated" {
						return fmt.Errorf("organization %s is not updated properly", orgName)
					}

					if org.Status.OrgStatus.StatusIndicator != "STATUS_INDICATION_IDLE" {
						return fmt.Errorf("organization %s is not active with status: %v", orgName, org.Status)
					}
					orgUID = org.Status.OrgStatus.UID
					if orgUID == "" {
						return fmt.Errorf("orgUID is empty for organization: %s with status: %v", orgName, org.Status)
					}
					return nil
				},
				1*time.Minute,
				5*time.Second,
			).Should(Succeed())

			Expect(orgUID).ToNot(Equal(""))

			logInfo("Update Project '%s'", projName)

			projDesc = []byte(`{ "description": "Tenancy Test Project - Updated" }`)
			resp, err = makeAuthorizedRequest(http.MethodPut, "https://api."+serviceDomainWithPort+"/v1/projects/"+projName, *token, projDesc, cli)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			logInfo("Validate Project '%s' update", projName)
			resp, err = makeAuthorizedRequest(http.MethodGet, "https://api."+serviceDomainWithPort+"/v1/projects/"+projName, *token, nil, cli)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(err).ToNot(HaveOccurred())

			Eventually(
				func() error {
					By("Getting the project UID")
					resp, err = makeAuthorizedRequest(http.MethodGet, "https://api."+serviceDomainWithPort+"/v1/projects/"+projName, *token, nil, cli)
					if err != nil {
						return fmt.Errorf("failed to get Project: %s with error: %w", projName, err)
					}
					defer resp.Body.Close()
					if resp.StatusCode != http.StatusOK {
						return fmt.Errorf("failed to get Project: %s with StatusCode: %d", projName, resp.StatusCode)
					}
					project, err := parseProject(resp.Body)
					if err != nil {
						return fmt.Errorf("failed to parse project: %s with error: %w", projName, err)
					}

					if project.Spec.Description != "Tenancy Test Project - Updated" {
						return fmt.Errorf("project %s is not updated properly", projName)
					}

					if project.Status.ProjectStatus.StatusIndicator != "STATUS_INDICATION_IDLE" {
						return fmt.Errorf("project %s is not active with status: %v", projName, project.Status)
					}
					projUID = project.Status.ProjectStatus.UID
					if projUID == "" {
						return fmt.Errorf("projUID is empty for project: %s with status: %v ", projName, project.Status)
					}
					return nil
				},
				1*time.Minute,
				5*time.Second,
			).Should(Succeed())

			Expect(projUID).ToNot(Equal(""))

			logInfo("Verify Edge Infra Manager services for Project: %s under Org: %s", projName, orgName)

			err = util.ManageTenancyUserAndRoles(context.Background(), cli, orgUID, projUID, http.MethodPost, tenancyUser, false)
			Expect(err).ToNot(HaveOccurred())
			token, err = util.GetApiToken(cli, tenancyUser, password)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() (*int, error) {
				resp, err = makeAuthorizedRequest(http.MethodGet, "https://api."+serviceDomainWithPort+"/v1/projects/"+projName+"/regions", *token, nil, cli)
				if err != nil {
					return nil, fmt.Errorf("failed to get regions: %w", err)
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
				}
				regionsList, err := parseRegionsList(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to parse regions list: %w", err)
				}
				return regionsList.TotalElements, nil
			}, 2*time.Minute, 10*time.Second).ShouldNot(BeNil(), "regions list should not be empty")

			logInfo("Verify Catalog services for Project: %s under Org: %s", projName, orgName)
			Eventually(func() (int, error) {
				resp, err = makeAuthorizedRequest(http.MethodGet, "https://api."+serviceDomainWithPort+"/v3/projects/"+projName+"/catalog/applications", *token, nil, cli)
				if err != nil {
					return 0, fmt.Errorf("failed to get applications: %w", err)
				}
				defer resp.Body.Close()
				return resp.StatusCode, nil
			}, 2*time.Minute, 10*time.Second).Should(Equal(http.StatusOK), "applications list should return 200 status code")

			logInfo("Delete Org '%s' without deleting projects", orgName)

			resp, err = makeAuthorizedRequest(http.MethodDelete, "https://api."+serviceDomainWithPort+"/v1/orgs/"+orgName, *token, nil, cli)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusConflict))

			logInfo("Delete Project '%s'", projName)

			resp, err = makeAuthorizedRequest(http.MethodDelete, "https://api."+serviceDomainWithPort+"/v1/projects/"+projName, *token, nil, cli)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			logInfo("Check if project is being processed for delete or deleted")
			projDeleted := false
			Eventually(func() error {
				resp, err = makeAuthorizedRequest(http.MethodGet, "https://api."+serviceDomainWithPort+"/v1/projects/"+projName, *token, nil, cli)
				if err != nil {
					return fmt.Errorf("failed to get project: %s with error: %w", projName, err)
				}
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusNotFound {
					logInfo("project not found, deleted")
					projDeleted = true
					return nil
				}
				project, err := parseProject(resp.Body)
				if err != nil {
					return fmt.Errorf("failed to parse project: %s with error: %w", projName, err)
				}
				hasMsg := strings.HasPrefix(project.Status.ProjectStatus.Message, "Waiting for watchers") &&
					strings.HasSuffix(project.Status.ProjectStatus.Message, "to be deleted")
				if project.Status.ProjectStatus.StatusIndicator == projectv1.StatusIndicationInProgress ||
					project.Status.ProjectStatus.StatusIndicator == projectv1.StatusIndicationError {
					if hasMsg {
						return fmt.Errorf("failed to delete project with status message '%s'", project.Status.ProjectStatus.Message)
					}
					return fmt.Errorf("failed to delete project: %s with status:%s and message:%s",
						projName, project.Status.ProjectStatus.StatusIndicator, project.Status.ProjectStatus.Message)
				}
				return fmt.Errorf("failed to delete project with statusmessage %s", project.Status.ProjectStatus.Message)
			}, 5*time.Minute, 20*time.Second).Should(Succeed(), "project should be deleted")

			orgDeleteRequestGiven := false
			logInfo("Delete Org '%s'", orgName)

			Eventually(func() error {
				if projDeleted {
					if !orgDeleteRequestGiven {
						resp, err = makeAuthorizedRequest(http.MethodDelete, "https://api."+serviceDomainWithPort+"/v1/orgs/"+orgName, *token, nil, cli)
						if err != nil {
							return fmt.Errorf("failed to delete organization: %s with error: %w", orgName, err)
						}
						defer resp.Body.Close()
						if resp.StatusCode != http.StatusOK {
							return fmt.Errorf("failed to delete org: %s with StatusCode: %d",
								orgName, resp.StatusCode)
						}
						orgDeleteRequestGiven = true
					}
					resp, err = makeAuthorizedRequest(http.MethodGet, "https://api."+serviceDomainWithPort+"/v1/orgs/"+orgName, *token, nil, cli)
					if err != nil {
						return fmt.Errorf("failed to get organization: %s with error: %w", orgName, err)
					}
					defer resp.Body.Close()
					if resp.StatusCode == http.StatusNotFound {
						logInfo("org not found, deleted")
						return nil
					}
					org, err := parseOrg(resp.Body)
					if err != nil {
						return fmt.Errorf("failed to parse org: %s with error: %w", orgName, err)
					}

					return fmt.Errorf("failed to delete org: %s with statusmessage: %s",
						orgName, org.Status.OrgStatus.Message)
				}
				logInfo("waiting on project deletion to issue org delete request")
				return fmt.Errorf("project is not yet deleted")
			}, 10*time.Minute, 20*time.Second).Should(Succeed(), "eventually should delete org after deleting the project")

			err = util.ManageTenancyUserAndRoles(context.Background(), cli, "", "", http.MethodDelete, tenancyUser, true)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
