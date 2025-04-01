// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/open-edge-platform/edge-manageability-framework/e2e-tests/orchestrator/helpers"
	util "github.com/open-edge-platform/edge-manageability-framework/mage"
)

const (
	orgAdmin                    = "observability-org-admin"
	serviceAdmin                = "observability-service-admin"
	observabilityUser           = "observability-user"
	observabilityOperatorUser   = "observability-operator"
	observabilityOnboardingUser = "observability-onboarding-user"
	enAgentUser                 = "observability-en-agent"
	noRolesUser                 = "no-roles-observability-user"
	observability               = "observability"
	// genericEnAgentUser is a user with en-agent-rw role, but not project prefixed.
	// If `en-agent-rw` role is removed - remove this user too.
	genericEnAgentUser = "observability-generic-en-agent"
)

var _ = Describe("Observability Test:", Ordered, Label(observability), func() {
	var (
		// Use a random org and project name to avoid conflicts with other tests.
		orgName  = "tenancy-o11y-org-" + generateRandomDigits(10)
		projName = "tenancy-o11y-project-" + generateRandomDigits(10)
		projUID  string
		orgUID   string
		token    *string
		user     string
		pass     string
		password string
		cli      *http.Client
		err      error
	)

	BeforeAll(func() {
		password, err = util.GetDefaultOrchPassword()
		Expect(err).ToNot(HaveOccurred())

		cli := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		}
		logInfo("Creating users, org and project for testing...")
		ctx := context.Background()

		err := helpers.CreateUser(ctx, orgAdmin)
		Expect(err).ToNot(HaveOccurred())
		err = helpers.AddUserToGroup(ctx, orgAdmin, "org-admin-group")
		Expect(err).ToNot(HaveOccurred())

		orgUID, projUID, err = helpers.PrepareOrgAndProject(ctx, cli, orgName, projName, serviceDomainWithPort, orgAdmin, password)
		Expect(err).ToNot(HaveOccurred())

		err = helpers.CreateUser(ctx, observabilityUser)
		Expect(err).ToNot(HaveOccurred())
		err = helpers.AddUserToGroup(ctx, observabilityUser, fmt.Sprintf("%v_Edge-Manager-Group", projUID))
		Expect(err).ToNot(HaveOccurred())

		err = helpers.CreateUser(ctx, observabilityOperatorUser)
		Expect(err).ToNot(HaveOccurred())
		err = helpers.AddUserToGroup(ctx, observabilityOperatorUser, fmt.Sprintf("%v_Edge-Operator-Group", projUID))
		Expect(err).ToNot(HaveOccurred())

		err = helpers.CreateUser(ctx, observabilityOnboardingUser)
		Expect(err).ToNot(HaveOccurred())
		err = helpers.AddUserToGroup(ctx, observabilityOnboardingUser, fmt.Sprintf("%v_Edge-Onboarding-Group", projUID))
		Expect(err).ToNot(HaveOccurred())

		err = helpers.CreateUser(ctx, genericEnAgentUser)
		Expect(err).ToNot(HaveOccurred())
		err = helpers.AddRealmRoleToUser(ctx, genericEnAgentUser, "en-agent-rw")
		Expect(err).ToNot(HaveOccurred())

		err = helpers.CreateUser(ctx, enAgentUser)
		Expect(err).ToNot(HaveOccurred())
		err = helpers.AddUserToGroup(ctx, enAgentUser, fmt.Sprintf("%v_Edge-Node-M2M-Service-Account", projUID))
		Expect(err).ToNot(HaveOccurred())

		err = helpers.CreateUser(ctx, serviceAdmin)
		Expect(err).ToNot(HaveOccurred())
		err = helpers.AddUserToGroup(ctx, serviceAdmin, "service-admin-group")
		Expect(err).ToNot(HaveOccurred())
		err = helpers.AddRealmRoleToUser(ctx, serviceAdmin, fmt.Sprintf("%v_%v_m", orgUID, projUID))
		Expect(err).ToNot(HaveOccurred())

		err = helpers.CreateUser(ctx, noRolesUser)
		Expect(err).ToNot(HaveOccurred())
		logInfo("Users, org and project for test created")
	})

	BeforeEach(func() {
		cli = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		}
		fmt.Printf("serviceDomain: %v\n", serviceDomain)
	})

	JustBeforeEach(func() {
		var err error
		token, err = util.GetApiToken(cli, user, pass)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("alerting endpoints", func() {
		When("using user in Edge Manager group", func() {
			BeforeEach(func() {
				user = observabilityUser
				pass = password
			})

			It("northbound API with alerts should be accessible over HTTPS", func() {
				resp, err := makeAuthorizedRequest(http.MethodGet, "https://api."+serviceDomainWithPort+"/v1/projects/"+projName+"/alerts", *token, nil, cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				content, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(ContainSubstring(`"alerts":`))
			})

			It("northbound API with alerts definitions should be accessible over HTTPS", func() {
				resp, err := makeAuthorizedRequest(http.MethodGet,
					"https://api."+serviceDomainWithPort+"/v1/projects/"+projName+"/alerts/definitions", *token, nil, cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				content, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(ContainSubstring(`"alertDefinitions":`))
			})

			It("northbound API with alerts receivers shouldn't be accessible over HTTPS without alrt-rx-rw role", func() {
				resp, err := makeAuthorizedRequest(http.MethodGet,
					"https://api."+serviceDomainWithPort+"/v1/projects/"+projName+"/alerts/receivers", *token, nil, cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		When("using user in Edge Operator group", func() {
			BeforeEach(func() {
				user = observabilityOperatorUser
				pass = password
			})

			It("northbound API with alerts should be accessible over HTTPS", func() {
				resp, err := makeAuthorizedRequest(http.MethodGet, "https://api."+serviceDomainWithPort+"/v1/projects/"+projName+"/alerts", *token, nil, cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				content, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(ContainSubstring(`"alerts":`))
			})

			It("northbound API with alerts definitions should be accessible over HTTPS", func() {
				resp, err := makeAuthorizedRequest(http.MethodGet,
					"https://api."+serviceDomainWithPort+"/v1/projects/"+projName+"/alerts/definitions", *token, nil, cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				content, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(ContainSubstring(`"alertDefinitions":`))
			})

			It("northbound API with alerts receivers shouldn't be accessible over HTTPS without alrt-rx-rw role", func() {
				resp, err := makeAuthorizedRequest(http.MethodGet,
					"https://api."+serviceDomainWithPort+"/v1/projects/"+projName+"/alerts/receivers", *token, nil, cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		When("using user in service-admin-group and <org>_<project>_m role", func() {
			BeforeEach(func() {
				user = serviceAdmin
				pass = password
			})

			It("northbound API with alerts should be accessible over HTTPS", func() {
				resp, err := makeAuthorizedRequest(http.MethodGet, "https://api."+serviceDomainWithPort+"/v1/projects/"+projName+"/alerts", *token, nil, cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				content, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(ContainSubstring(`"alerts":`))
			})

			It("northbound API with alerts definitions should be accessible over HTTPS", func() {
				resp, err := makeAuthorizedRequest(http.MethodGet,
					"https://api."+serviceDomainWithPort+"/v1/projects/"+projName+"/alerts/definitions", *token, nil, cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				content, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(ContainSubstring(`"alertDefinitions":`))
			})

			It("Northbound API with alerts receivers should be accessible", func() {
				resp, err := makeAuthorizedRequest(http.MethodGet,
					"https://api."+serviceDomainWithPort+"/v1/projects/"+projName+"/alerts/receivers", *token, nil, cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				content, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(ContainSubstring(`"receivers":`))
			})
		})

		When("using user without required permissions", func() {
			BeforeEach(func() {
				user = noRolesUser
				pass = password
			})

			It("northbound API with alerts shouldn't be accessible over HTTPS without alrt-r role", func() {
				resp, err := makeAuthorizedRequest(http.MethodGet, "https://api."+serviceDomainWithPort+"/v1/projects/"+projName+"/alerts", *token, nil, cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
			})

			It("northbound API with alerts definitions shouldn't be accessible over HTTPS without alrt-r role", func() {
				resp, err := makeAuthorizedRequest(http.MethodGet,
					"https://api."+serviceDomainWithPort+"/v1/projects/"+projName+"/alerts/definitions", *token, nil, cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
			})

			It("northbound API with alerts receivers shouldn't be accessible over HTTPS without alrt-rx-rw role", func() {
				resp, err := makeAuthorizedRequest(http.MethodGet,
					"https://api."+serviceDomainWithPort+"/v1/projects/"+projName+"/alerts/receivers", *token, nil, cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
			})
		})
	})

	Describe("edgenode Observability UI service", func() {
		BeforeEach(func() {
			user = observabilityUser
			pass = password
		})

		It("should be accessible over HTTPS", func() {
			resp, err := makeAuthorizedRequest(http.MethodGet, "https://observability-ui."+serviceDomainWithPort, *token, nil, cli)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			_, err = io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("edgenode Observability metrics endpoint", func() {
		When("using user with valid roles", func() {
			BeforeEach(func() {
				user = enAgentUser
				pass = password
			})

			It("should be accessible HTTPS", func() {
				resp, err := makeAuthorizedRequest(http.MethodGet,
					"https://metrics-node."+serviceDomainWithPort+"/prometheus/api/v1/query?query=up", *token, nil, cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				content, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("success"))
			})
		})

		When("using user without project prefixed role", func() {
			BeforeEach(func() {
				user = genericEnAgentUser
				pass = password
			})

			It("should NOT be accessible over HTTPS ", func() {
				resp, err := makeAuthorizedRequest(http.MethodGet,
					"https://metrics-node."+serviceDomainWithPort+"/prometheus/api/v1/query?query=up", *token, nil, cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
				content, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("Invalid claims"))
			})
		})

		When("using user without valid roles", func() {
			BeforeEach(func() {
				user = noRolesUser
				pass = password
			})

			It("should NOT be accessible over HTTPS ", func() {
				resp, err := makeAuthorizedRequest(http.MethodGet,
					"https://metrics-node."+serviceDomainWithPort+"/prometheus/api/v1/query?query=up", *token, nil, cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
				content, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("Invalid claims"))
			})

			It("should NOT be accessible over HTTPS using no token", func() {
				req, err := http.NewRequest(http.MethodGet, "https://metrics-node."+serviceDomainWithPort+"/prometheus/api/v1/query?query=up", nil)
				Expect(err).ToNot(HaveOccurred())
				resp, err := cli.Do(req)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
			})

			It("should NOT be accessible over HTTPS using invalid token", func() {
				const invalid = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c" //nolint:lll // Token needed for testing - can't split
				resp, err := makeAuthorizedRequest(http.MethodGet,
					"https://metrics-node."+serviceDomainWithPort+"/prometheus/api/v1/query?query=up", invalid, nil, cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
			})
		})
	})

	Describe("edgenode Observability logs endpoint", func() {
		When("using user with valid roles", func() {
			BeforeEach(func() {
				user = enAgentUser
				pass = password
			})

			It("should be accessible over HTTPS", func() {
				resp, err := makeAuthorizedRequest(http.MethodPost, "https://logs-node."+serviceDomainWithPort+"/v1/logs", *token, []byte(`{}`), cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				content, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(ContainSubstring(`{"partialSuccess":{}}`))
			})
		})

		When("using user with valid roles - provisioning logs", func() {
			BeforeEach(func() {
				user = observabilityOnboardingUser
				pass = password
			})

			It("should be accessible over HTTPS", func() {
				resp, err := makeAuthorizedRequest(http.MethodPost, "https://logs-node."+serviceDomainWithPort+"/v1/logs", *token, []byte(`{}`), cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				content, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(ContainSubstring(`{"partialSuccess":{}}`))
			})
		})

		When("using user without project prefixed role", func() {
			BeforeEach(func() {
				user = genericEnAgentUser
				pass = password
			})

			It("should NOT be accessible over HTTPS", func() {
				resp, err := makeAuthorizedRequest(http.MethodPost, "https://logs-node."+serviceDomainWithPort+"/v1/logs", *token, []byte(`{}`), cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
				content, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("Invalid claims"))
			})
		})

		When("using user without valid roles", func() {
			BeforeEach(func() {
				user = noRolesUser
				pass = password
			})

			It("should NOT be accessible over HTTPS", func() {
				resp, err := makeAuthorizedRequest(http.MethodPost, "https://logs-node."+serviceDomainWithPort+"/v1/logs", *token, []byte(`{}`), cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
				content, err := io.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("Invalid claims"))
			})

			It("should not be accessible over HTTPS when using no token", func() {
				bodyReader := bytes.NewReader([]byte(`{}`))
				req, err := http.NewRequest(http.MethodPost, "https://logs-node."+serviceDomainWithPort+"/v1/logs", bodyReader)
				Expect(err).ToNot(HaveOccurred())
				resp, err := cli.Do(req)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
			})

			It("should NOT be accessible over HTTPS when uses invalid token", func() {
				const invalid = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c" //nolint:lll // Token needed for testing - can't split
				resp, err := makeAuthorizedRequest(http.MethodPost, "https://logs-node."+serviceDomainWithPort+"/v1/logs", invalid, []byte(`{}`), cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
			})
		})
	})

	AfterAll(func() {
		logInfo("Deleting users, org and project used by test...")
		token, err := util.GetApiToken(cli, orgAdmin, password)
		Expect(err).ToNot(HaveOccurred())

		ctx := context.Background()
		err = helpers.DeleteOrgAndProject(ctx, cli, orgName, projName, *token, serviceDomainWithPort)
		Expect(err).ToNot(HaveOccurred())

		err = util.ManageTenancyUserAndRoles(ctx, cli, "", "", http.MethodDelete, observabilityUser, true)
		Expect(err).ToNot(HaveOccurred())

		err = util.ManageTenancyUserAndRoles(ctx, cli, "", "", http.MethodDelete, observabilityOnboardingUser, true)
		Expect(err).ToNot(HaveOccurred())

		err = util.ManageTenancyUserAndRoles(ctx, cli, "", "", http.MethodDelete, genericEnAgentUser, true)
		Expect(err).ToNot(HaveOccurred())

		err = util.ManageTenancyUserAndRoles(ctx, cli, "", "", http.MethodDelete, noRolesUser, true)
		Expect(err).ToNot(HaveOccurred())

		err = util.ManageTenancyUserAndRoles(ctx, cli, "", "", http.MethodDelete, serviceAdmin, true)
		Expect(err).ToNot(HaveOccurred())

		err = util.ManageTenancyUserAndRoles(ctx, cli, "", "", http.MethodDelete, enAgentUser, true)
		Expect(err).ToNot(HaveOccurred())

		err = util.ManageTenancyUserAndRoles(ctx, cli, "", "", http.MethodDelete, orgAdmin, true)
		Expect(err).ToNot(HaveOccurred())
		logInfo("Users, org and project used by test deleted")
	})
})
