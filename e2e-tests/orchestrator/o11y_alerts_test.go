// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/open-edge-platform/edge-manageability-framework/e2e-tests/orchestrator/helpers"
	util "github.com/open-edge-platform/edge-manageability-framework/mage"
)

const (
	alertsObs = "observability-alerts"
	// Org and project names must be the same as in step in CI.
	alertOrgName     = "sample-org"
	alertProjectName = "sample-project"
	alertUser        = "alert-user"
	alertOpUser      = "alert-operator-user"
	mailpitNamespace = "mailpit-dev"
	mailpitSvc       = "mailpit-svc"
	mailpitPort      = 15000
)

var alertDef = []string{
	"ClusterRAMUsageExceedsThreshold",
	"DiskUsageExceedsThreshold",
	"DeploymentStatusDown",
	"DeploymentStatusError",
	"CPUUsageExceedsThreshold",
	"HostStatusError",
	"HostStatusConnectionLost",
	"HostCPUTemperatureExceedsThreshold",
	"RAMUsageExceedsThreshold",
	"DeploymentStatusNoTargetClusters",
	"DeploymentStatusInternalError",
	"HostStatusUpdateFailed",
	"DeploymentInstanceStatusDown",
	"ClusterCPUUsageExceedsThreshold",
	"HostStatusProvisionFailed",
	"HighNetworkUsage",
}

var expectedAlerts = []string{
	"CPUUsageExceedsThreshold",
	"RAMUsageExceedsThreshold",
}

var _ = Describe("Observability Alerts Test:", Ordered, Label(alertsObs), func() {
	var (
		cli                *http.Client
		projectID          string
		token              *string
		initialDefinitions *helpers.AlertDefinitionsArray
		receiverID         string
		mailpitURL         string
		mailpitPortFwdCmd  *exec.Cmd
		user               string
		password           string
	)

	BeforeAll(func() {
		ctx := context.Background()

		err := helpers.CreateUser(ctx, alertUser)
		Expect(err).ToNot(HaveOccurred())

		projectID, err = util.GetProjectId(ctx, alertOrgName, alertProjectName)
		Expect(err).ToNot(HaveOccurred())

		err = helpers.AddUserToGroup(ctx, alertUser, fmt.Sprintf("%v_Edge-Manager-Group", projectID))
		Expect(err).ToNot(HaveOccurred())

		err = helpers.AddUserToGroup(ctx, alertUser, "service-admin-group")
		Expect(err).ToNot(HaveOccurred())

		err = helpers.CreateUser(ctx, alertOpUser)
		Expect(err).ToNot(HaveOccurred())

		err = helpers.AddUserToGroup(ctx, alertOpUser, fmt.Sprintf("%v_Edge-Operator-Group", projectID))
		Expect(err).ToNot(HaveOccurred())

		password, err = util.GetDefaultOrchPassword()
		Expect(err).ToNot(HaveOccurred())

		mailpitURL = fmt.Sprintf("http://localhost:%v", mailpitPort)
		args := []string{
			"port-forward",
			fmt.Sprintf("svc/%v", mailpitSvc),
			"-n",
			mailpitNamespace,
			strconv.Itoa(mailpitPort) + ":8025",
		}
		mailpitPortFwdCmd = exec.Command("kubectl", args...)
		err = mailpitPortFwdCmd.Start()
		Expect(err).ToNot(HaveOccurred())

		initialDefinitions = new(helpers.AlertDefinitionsArray)
	})

	BeforeEach(func() {
		cli = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		}
	})

	JustBeforeEach(func() {
		var err error
		token, err = util.GetApiToken(cli, user, password)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("Alerts tests", func() {
		When("using user in Edge Manager group", func() {
			BeforeEach(func() {
				user = alertUser
			})

			It("verify that alert receivers can be patched correctly", func() {
				By("getting receivers and extracting its id")
				resp, err := makeAuthorizedRequest(http.MethodGet,
					"https://api."+serviceDomainWithPort+"/v1/projects/"+alertProjectName+"/alerts/receivers", *token, nil, cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				receivers := new(helpers.AlertReceiversArray)
				err = helpers.ParseJSONBody(resp.Body, receivers)
				Expect(err).ToNot(HaveOccurred())
				Expect(receivers.Receivers[0].EmailConfig.From).To(ContainSubstring("Open Edge Platform Alert"))
				receiverID = receivers.Receivers[0].ID
				Expect(receiverID).ToNot(BeEmpty())

				By("enabling alertUser email for sending notifications")
				emailConfig := helpers.PatchReceiverBody{}
				alertUserEmail := fmt.Sprintf("%s %s <%s@observability-user.com>", alertUser, alertUser, alertUser)
				emailConfig.EmailConfig.To.Enabled = []string{alertUserEmail}
				reqBody, err := json.Marshal(emailConfig)
				Expect(err).ToNot(HaveOccurred())

				resp, err = makeAuthorizedRequest(http.MethodPatch,
					"https://api."+serviceDomainWithPort+"/v1/projects/"+alertProjectName+"/alerts/receivers/"+receiverID, *token, reqBody, cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

				By("verifying that alertUser's email is in enabled list")
				found, err := helpers.IsEmailEnabled(cli, serviceDomainWithPort, *token, alertProjectName, receiverID, alertUserEmail)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				By("verifying that receiver is in applied state")
				Eventually(helpers.VerifyReceiverState,
					2*time.Minute, 10*time.Second).WithArguments(cli,
					serviceDomainWithPort, *token, alertProjectName, receiverID, "Applied").Should(Succeed(), "eventually receiver has to be applied")
			})

			It("verify that CPU and Memory alerts are not present", func() {
				endpoint := "https://api." + serviceDomainWithPort + "/v1/projects/" + alertProjectName + "/alerts"
				resp, err := makeAuthorizedRequest(http.MethodGet, endpoint, *token, nil, cli)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				defer resp.Body.Close()

				alerts := new(helpers.Alerts)
				err = helpers.ParseJSONBody(resp.Body, alerts)
				Expect(err).ToNot(HaveOccurred())

				for _, alert := range alerts.Alerts {
					Expect(alert.Labels.Alertname).ToNot(BeElementOf("CPUUsageExceedsThreshold", "RAMUsageExceedsThreshold"))
				}
			})

			It("verify that alert definitions contain expected definitions", func() {
				resp, err := makeAuthorizedRequest(http.MethodGet,
					"https://api."+serviceDomainWithPort+"/v1/projects/"+alertProjectName+"/alerts/definitions", *token, nil, cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()

				err = helpers.ParseJSONBody(resp.Body, initialDefinitions)
				Expect(err).ToNot(HaveOccurred())

				definitionsNames := make([]string, len(initialDefinitions.AlertDefinitions))
				for i, definition := range initialDefinitions.AlertDefinitions {
					definitionsNames[i] = definition.Name
				}

				Expect(definitionsNames).To(ContainElements(alertDef), "expected definitions not found")
			})

			It("verify that alert definition templates can be accessed and alert definitions can be patched correctly", func() {
				By("getting definition templates and patching alert definitions")
				for _, definition := range initialDefinitions.AlertDefinitions {
					template, err := func() (*helpers.AlertDefinitionTemplate, error) {
						endpoint := fmt.Sprintf("https://api."+serviceDomainWithPort+"/v1/projects/"+alertProjectName+"/alerts/definitions/%v/template",
							definition.ID)

						resp, err := makeAuthorizedRequest(http.MethodGet, endpoint, *token, nil, cli)
						if err != nil {
							return nil, err
						}
						defer resp.Body.Close()
						if resp.StatusCode != http.StatusOK {
							return nil, fmt.Errorf("endpoint returned non 200 status, returned code: %v", resp.StatusCode)
						}

						template := new(helpers.AlertDefinitionTemplate)
						return template, helpers.ParseJSONBody(resp.Body, template)
					}()
					Expect(err).ToNot(HaveOccurred())

					patchJSON := helpers.PatchDefinitionBody{
						Values: helpers.PatchDefinitionValues{
							Duration:  definition.Values.Duration,
							Threshold: template.Annotations.AmThresholdMin,
							Enabled:   definition.Values.Enabled,
						},
					}
					reqBody, err := json.Marshal(patchJSON)
					Expect(err).ToNot(HaveOccurred())

					statusCode, err := func() (int, error) {
						endpoint := fmt.Sprintf("https://api."+serviceDomainWithPort+"/v1/projects/"+alertProjectName+"/alerts/definitions/%v",
							definition.ID)
						resp, err := makeAuthorizedRequest(http.MethodPatch, endpoint, *token, reqBody, cli)
						if err != nil {
							return 0, err
						}
						defer resp.Body.Close()
						return resp.StatusCode, nil
					}()
					Expect(err).ToNot(HaveOccurred())
					Expect(statusCode).To(Equal(http.StatusNoContent))
				}

				By("verifying that alert definitions have minimal thersholds")
				for _, initialDefinition := range initialDefinitions.AlertDefinitions {
					template, err := func() (*helpers.AlertDefinitionTemplate, error) {
						endpoint := fmt.Sprintf("https://api."+serviceDomainWithPort+"/v1/projects/"+alertProjectName+"/alerts/definitions/%v/template",
							initialDefinition.ID)
						resp, err := makeAuthorizedRequest(http.MethodGet, endpoint, *token, nil, cli)
						if err != nil {
							return nil, err
						}
						defer resp.Body.Close()

						template := new(helpers.AlertDefinitionTemplate)
						return template, helpers.ParseJSONBody(resp.Body, template)
					}()
					Expect(err).ToNot(HaveOccurred())

					definition, err := func() (*helpers.AlertDefinition, error) {
						endpoint := fmt.Sprintf("https://api."+serviceDomainWithPort+"/v1/projects/"+alertProjectName+"/alerts/definitions/%v",
							initialDefinition.ID)
						resp, err := makeAuthorizedRequest(http.MethodGet, endpoint, *token, nil, cli)
						if err != nil {
							return nil, err
						}
						defer resp.Body.Close()
						if resp.StatusCode != http.StatusOK {
							return nil, fmt.Errorf("endpoint returned non 200 status, returned code: %v", resp.StatusCode)
						}

						definition := new(helpers.AlertDefinition)
						return definition, helpers.ParseJSONBody(resp.Body, definition)
					}()
					Expect(err).ToNot(HaveOccurred())
					Expect(definition.Values.Threshold).To(Equal(template.Annotations.AmThresholdMin))
				}
			})

			It("verify that expected alerts are coming", func() {
				Eventually(func() ([]string, error) {
					endpoint := "https://api." + serviceDomainWithPort + "/v1/projects/" + alertProjectName + "/alerts"
					resp, err := makeAuthorizedRequest(http.MethodGet, endpoint, *token, nil, cli)
					if err != nil {
						return nil, err
					}
					defer resp.Body.Close()
					if resp.StatusCode != http.StatusOK {
						return nil, fmt.Errorf("endpoint returned non 200 status, returned code: %v", resp.StatusCode)
					}

					alerts := new(helpers.Alerts)
					err = helpers.ParseJSONBody(resp.Body, alerts)
					if err != nil {
						return nil, err
					}

					alertsNames := make([]string, 0)
					for _, alert := range alerts.Alerts {
						alertsNames = append(alertsNames, alert.Labels.Alertname)
					}
					return alertsNames, nil
				}, 4*time.Minute, 10*time.Second).Should(ContainElements(expectedAlerts), "eventually expected alerts should be fired")
			})

			It("verify that there aren't any resolved alerts", func() {
				endpoint := "https://api." + serviceDomainWithPort + "/v1/projects/" + alertProjectName + "/alerts?suppressed=false&active=false&resolved=true"
				resp, err := makeAuthorizedRequest(http.MethodGet, endpoint, *token, nil, cli)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				defer resp.Body.Close()

				alerts := new(helpers.Alerts)
				err = helpers.ParseJSONBody(resp.Body, alerts)
				Expect(err).ToNot(HaveOccurred())

				Expect(alerts.Alerts).To(BeEmpty())
			})

			It("verify that there aren't any suppressed alerts", func() {
				endpoint := "https://api." + serviceDomainWithPort + "/v1/projects/" + alertProjectName + "/alerts?suppressed=true&active=false&resolved=false"
				resp, err := makeAuthorizedRequest(http.MethodGet, endpoint, *token, nil, cli)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				defer resp.Body.Close()

				alerts := new(helpers.Alerts)
				err = helpers.ParseJSONBody(resp.Body, alerts)
				Expect(err).ToNot(HaveOccurred())

				Expect(alerts.Alerts).To(BeEmpty())
			})

			It("verify that email notifications are being sent", func() {
				Eventually(func() error {
					messagesURL := mailpitURL + "/api/v1/messages"
					resp, err := helpers.MakeRequest(http.MethodGet, messagesURL, nil, cli, nil)
					if err != nil {
						return err
					}
					defer resp.Body.Close()
					if resp.StatusCode != http.StatusOK {
						return fmt.Errorf("endpoint returned non 200 status, status returned: %v", resp.StatusCode)
					}

					mails := new(helpers.MailList)
					err = helpers.ParseJSONBody(resp.Body, mails)
					if err != nil {
						return err
					}
					if mails.Total == 0 {
						return errors.New("no mails received yet")
					}

					var ramUsageFound, cpuUsageFound bool
					for _, mail := range mails.Messages {
						err := func(mailID string) error {
							messageURL := fmt.Sprintf("%s/api/v1/message/%s", mailpitURL, mailID)
							resp, err := helpers.MakeRequest(http.MethodGet, messageURL, nil, cli, nil)
							if err != nil {
								return err
							}
							defer resp.Body.Close()
							if resp.StatusCode != http.StatusOK {
								return fmt.Errorf("message endpoint returned non 200 status, status returned: %v", resp.StatusCode)
							}

							content, err := io.ReadAll(resp.Body)
							if err != nil {
								return err
							}

							if strings.Contains(string(content), "Host RAM Usage Exceeds Threshold") {
								ramUsageFound = true
							}
							if strings.Contains(string(content), "Host CPU Usage Exceeds Threshold") {
								cpuUsageFound = true
							}
							return nil
						}(mail.ID)
						if err != nil {
							return err
						}
					}

					if ramUsageFound && cpuUsageFound {
						return nil
					}
					return errors.New("expected notifications not found yet")
				}, 4*time.Minute, 10*time.Second).Should(Succeed(), "all expected email notifications should be sent")
			})

			It("verify that alert receivers email can be disabled correctly", func() {
				By("verifying that alertUser's email is in enabled list")
				alertUserEmail := fmt.Sprintf("%s %s <%s@observability-user.com>", alertUser, alertUser, alertUser)
				found, err := helpers.IsEmailEnabled(cli, serviceDomainWithPort, *token, alertProjectName, receiverID, alertUserEmail)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				By("disabling alertUser email for sending notifications")
				emailConfig := helpers.PatchReceiverBody{}
				reqBody, err := json.Marshal(emailConfig)
				Expect(err).ToNot(HaveOccurred())

				resp, err := makeAuthorizedRequest(http.MethodPatch,
					"https://api."+serviceDomainWithPort+"/v1/projects/"+alertProjectName+"/alerts/receivers/"+receiverID, *token, reqBody, cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

				By("verifying that alertUser's email is not in enabled list")
				found, err = helpers.IsEmailEnabled(cli, serviceDomainWithPort, *token, alertProjectName, receiverID, alertUserEmail)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())

				By("verifying that receiver is in applied state")
				Eventually(helpers.VerifyReceiverState,
					2*time.Minute, 10*time.Second).WithArguments(cli,
					serviceDomainWithPort, *token, alertProjectName, receiverID, "Applied").Should(Succeed(), "eventually receiver has to be applied")
			})
		})

		When("using user in Edge Operator group", func() {
			BeforeEach(func() {
				user = alertOpUser
			})

			It("verify that alert definitions templates can be accessed", func() {
				definition := initialDefinitions.AlertDefinitions[0]
				endpoint := fmt.Sprintf("https://api."+serviceDomainWithPort+"/v1/projects/"+alertProjectName+"/alerts/definitions/%v/template",
					definition.ID)

				resp, err := makeAuthorizedRequest(http.MethodGet, endpoint, *token, nil, cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
			})

			It("verify that alert definitions cannot be patched", func() {
				definition := initialDefinitions.AlertDefinitions[0]

				patchJSON := helpers.PatchDefinitionBody{
					Values: helpers.PatchDefinitionValues{
						Duration:  definition.Values.Duration,
						Threshold: definition.Values.Threshold,
						Enabled:   definition.Values.Enabled,
					},
				}
				reqBody, err := json.Marshal(patchJSON)
				Expect(err).ToNot(HaveOccurred())

				endpoint := fmt.Sprintf("https://api."+serviceDomainWithPort+"/v1/projects/"+alertProjectName+"/alerts/definitions/%v",
					definition.ID)
				resp, err := makeAuthorizedRequest(http.MethodPatch, endpoint, *token, reqBody, cli)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	AfterAll(func() {
		logInfo("Deleting users used by test and reverting alert threshold...")

		user = alertUser
		token, err := util.GetApiToken(cli, user, password)
		Expect(err).ToNot(HaveOccurred())

		err = helpers.PatchAlertDefinitions(cli, serviceDomainWithPort, *token, alertProjectName, *initialDefinitions)
		Expect(err).ToNot(HaveOccurred())

		ctx := context.Background()
		err = util.ManageTenancyUserAndRoles(ctx, cli, "", "", http.MethodDelete, alertUser, true)
		Expect(err).ToNot(HaveOccurred())

		err = util.ManageTenancyUserAndRoles(ctx, cli, "", "", http.MethodDelete, alertOpUser, true)
		Expect(err).ToNot(HaveOccurred())

		messagesURL := mailpitURL + "/api/v1/messages"
		resp, err := helpers.MakeRequest(http.MethodDelete, messagesURL, nil, cli, nil)
		Expect(err).ToNot(HaveOccurred())
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		err = mailpitPortFwdCmd.Process.Kill()
		Expect(err).ToNot(HaveOccurred())
	})
})
