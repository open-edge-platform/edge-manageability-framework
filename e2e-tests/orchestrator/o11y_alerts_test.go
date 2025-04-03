// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/open-edge-platform/edge-manageability-framework/e2e-tests/orchestrator/helpers"
	util "github.com/open-edge-platform/edge-manageability-framework/mage"
)

const (
	// Org and project names must match those set by CI pipeline.
	alertOrgName                          = "sample-org"
	alertProjectName                      = "sample-project"
	alertUser                             = "alert-user"
	alertOpUser                           = "alert-operator-user"
	hostUser                              = "host-user"
	mailpitNamespace                      = "mailpit-dev"
	mailpitSvc                            = "mailpit-svc"
	mailpitPort                           = 15000
	hostConnectionLostErrorString         = "Host Status Connection Lost"
	hostConnectionLostAlertDisplayName    = "Host Status Connection Lost"
	tplConnectionLostResolved             = `\*\[[0-9]+\] Resolved\* \*Project:\* %s\\n\*Alert Name:\* %s`
	tplConnectionLostFiring               = `\*\[[0-9]+\] Firing\* \*Project:\* %s\\n\*Alert Name:\* %s`
	receiverUpdateAppliedTimeout          = 2 * time.Minute
	resourceUsageAlertFiringTimeout       = 4 * time.Minute
	resourceUsageAlertNotificationTimeout = 4 * time.Minute
	hostConnectionLostAlertFiringTimeout  = 8 * time.Minute
	hostConnectionLostAlertAbsentTimeout  = 4 * time.Minute
	alertSuppressionInMaintenanceTimeout  = 4 * time.Minute
	hostConnectionLostNotificationTimeout = 8 * time.Minute
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

var expectedCPUMemAlerts = []string{
	"CPUUsageExceedsThreshold",
	"RAMUsageExceedsThreshold",
}

var suppressedAlertsQueryParams = []string{
	"suppressed=true",
	"active=false",
}

var _ = Describe("Observability Alerts Test:", Ordered, Label(helpers.LabelAlerts, helpers.LabelApplicationMonitor), func() {
	var (
		cli                     *http.Client
		projectID               string
		token                   *string
		initialDefinitions      *helpers.AlertDefinitionsArray
		receiverID              string
		mailpitURL              string
		mailpitPortFwdCmd       *exec.Cmd
		user                    string
		password                string
		suppressContext         *helpers.MaintenanceModeContext
		apiClient               *helpers.APIClient
		rConnectionLostFiring   *regexp.Regexp
		rConnectionLostResolved *regexp.Regexp
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

		err = helpers.CreateUser(ctx, hostUser)
		Expect(err).ToNot(HaveOccurred())

		err = helpers.AddUserToGroup(ctx, hostUser, fmt.Sprintf("%v_Host-Manager-Group", projectID))
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

		suppressContext = new(helpers.MaintenanceModeContext)

		rConnectionLostFiring, err = regexp.Compile(fmt.Sprintf(tplConnectionLostFiring, projectID, hostConnectionLostErrorString))
		Expect(err).ToNot(HaveOccurred())
		rConnectionLostResolved, err = regexp.Compile(fmt.Sprintf(tplConnectionLostResolved, projectID, hostConnectionLostErrorString))
		Expect(err).ToNot(HaveOccurred())
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
		apiClient = helpers.NewAPIClient(cli, serviceDomainWithPort, alertProjectName, *token)
	})

	Context("Running preparatory test cases", func() {
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
				Expect(receivers.Receivers).ToNot(BeEmpty())
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
					receiverUpdateAppliedTimeout, 10*time.Second).WithArguments(cli,
					serviceDomainWithPort, *token, alertProjectName, receiverID, "Applied").Should(Succeed(), "eventually receiver has to be applied")
			})

			It("delete any existing alert receiver mail messages", func() {
				err := helpers.DeleteAlertReceiverMessages(cli, mailpitURL)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Context("Alerts tests for edge node metrics", func() {
		When("using user in Edge Manager group", func() {
			BeforeEach(func() {
				user = alertUser
			})

			It("verify that CPU and Memory alerts are not present", func() {
				alertNames, err := helpers.GetAlertNames(apiClient)
				Expect(err).ToNot(HaveOccurred())
				Expect(alertNames).ToNot(Or(ContainElement("CPUUsageExceedsThreshold"), ContainElement("RAMUsageExceedsThreshold")))
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

				By("verifying that alert definitions have minimal thresholds")
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

			It("verify that expected alerts are coming", Label(helpers.LabelEnic), func() {
				Eventually(helpers.GetAlertNames, resourceUsageAlertFiringTimeout, 10*time.Second).WithArguments(apiClient).Should(
					ContainElements(expectedCPUMemAlerts),
					"eventually expected alerts should be fired",
				)
			})

			It("verify that email notifications are being sent", func() {
				Eventually(func() error {
					messages, err := helpers.GetAlertReceiverMessages(cli, mailpitURL)
					if err != nil {
						return err
					}
					var ramUsageFound, cpuUsageFound bool
					if slices.ContainsFunc(messages, func(msg string) bool {
						return strings.Contains(msg, "Host RAM Usage Exceeds Threshold")
					}) {
						ramUsageFound = true
					}
					if slices.ContainsFunc(messages, func(msg string) bool {
						return strings.Contains(msg, "Host CPU Usage Exceeds Threshold")
					}) {
						cpuUsageFound = true
					}
					if ramUsageFound && cpuUsageFound {
						return nil
					}
					return errors.New("expected notifications not found yet")
				}, resourceUsageAlertNotificationTimeout, 10*time.Second).Should(
					Succeed(), "all expected email notifications should be sent",
				)
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

	Context("Alerts tests for host status metrics",
		Label(helpers.LabelEnic, helpers.LabelExtended, helpers.LabelApplicationInfraCore), func() {
			When("using user in Edge Manager group", func() {
				BeforeEach(func() {
					user = alertUser
				})

				It("Verify that no HostStatus* alerts are present", func() {
					alertNames, err := helpers.GetAlertNames(apiClient)
					Expect(err).ToNot(HaveOccurred())
					Expect(alertNames).ToNot(Or(ContainElement("HostStatusError"), ContainElement("HostStatusConnectionLost")))
				})

				It("Verify that HostStatusConnectionLost alert fires", func() {
					By("Blocking the connection from the host")
					err := helpers.BlockEnicTraffic()
					Expect(err).ToNot(HaveOccurred())

					By("Waiting for the alert to fire")
					Eventually(helpers.GetAlertNames, hostConnectionLostAlertFiringTimeout, 10*time.Second).WithArguments(apiClient).Should(
						ContainElements("HostStatusConnectionLost"),
						"eventually expected alerts should fire",
					)
				})

				It("verify that there aren't any suppressed alerts", func() {
					alertNames, err := helpers.GetAlertNames(apiClient, suppressedAlertsQueryParams...)
					Expect(err).ToNot(HaveOccurred())
					Expect(alertNames).To(BeEmpty())
				})

				It("Verify that HostStatusConnectionLost alert is suppressed when the host is put into maintenance mode", func() {
					By("Scheduling maintenance mode for the host")

					hostApiToken, err := util.GetApiToken(cli, hostUser, password)
					Expect(err).ToNot(HaveOccurred())
					hostApiClient := helpers.NewAPIClient(cli, serviceDomainWithPort, alertProjectName, *hostApiToken)
					err = helpers.SetMaintenanceModeForEnic(hostApiClient, suppressContext)
					Expect(err).ToNot(HaveOccurred())

					By("Waiting for the alert to be suppressed")
					Eventually(func() ([]string, error) {
						return helpers.GetAlertNames(apiClient, suppressedAlertsQueryParams...)
					}, alertSuppressionInMaintenanceTimeout, 10*time.Second).Should(
						ContainElements("HostStatusConnectionLost"),
						"eventually expected alert should be suppressed",
					)
				})

				It("verify that email notification for HostStatusConnectionLost alert firing is sent", func() {
					Eventually(func() error {
						mails, err := helpers.GetAlertReceiverMessages(cli, mailpitURL)
						if err != nil {
							return err
						}
						if slices.ContainsFunc(mails, rConnectionLostFiring.MatchString) {
							return nil
						}
						return errors.New("expected email notification not found")
					}, hostConnectionLostNotificationTimeout, 10*time.Second).Should(Succeed(), "eventually email notification should be sent")
				})

				It("Verify that HostStatusConnectionLost alert is absent", func() {
					By("Unblocking the connection from the host")
					err := helpers.UnblockEnicTraffic()
					Expect(err).ToNot(HaveOccurred())

					By("Waiting for the alert to be absent")
					Eventually(func() ([]string, error) {
						return helpers.GetAlertNames(apiClient, suppressedAlertsQueryParams...)
					}, hostConnectionLostAlertAbsentTimeout, 10*time.Second).ShouldNot(
						ContainElements("HostStatusConnectionLost"),
						"eventually previously fired alerts should be absent",
					)
				})

				It("verify that alert HostStatusConnectionLost is resolved in email notifications", func() {
					Eventually(func() error {
						mails, err := helpers.GetAlertReceiverMessages(cli, mailpitURL)
						if err != nil {
							return err
						}
						if slices.ContainsFunc(mails, rConnectionLostResolved.MatchString) {
							return nil
						}
						return errors.New("expected email notification not found")
					}, hostConnectionLostNotificationTimeout, 10*time.Second).Should(Succeed(), "eventually email notification should be sent")
				})
			})
		})

	Context("Closing alerts test cases", func() {
		When("using user in Edge Manager group", func() {
			BeforeEach(func() {
				user = alertUser
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
					receiverUpdateAppliedTimeout, 10*time.Second).WithArguments(cli,
					serviceDomainWithPort, *token, alertProjectName, receiverID, "Applied").Should(Succeed(), "eventually receiver has to be applied")
			})
		})
	})

	AfterAll(func() {
		By("Deleting users used by test and reverting alert threshold...")

		hostApiToken, err := util.GetApiToken(cli, hostUser, password)
		Expect(err).ToNot(HaveOccurred())
		hostApiClient := helpers.NewAPIClient(cli, serviceDomainWithPort, alertProjectName, *hostApiToken)

		err = helpers.PatchAlertDefinitions(cli, serviceDomainWithPort, *token, alertProjectName, *initialDefinitions)
		Expect(err).ToNot(HaveOccurred())

		ctx := context.Background()
		err = util.ManageTenancyUserAndRoles(ctx, cli, "", "", http.MethodDelete, alertUser, true)
		Expect(err).ToNot(HaveOccurred())

		err = util.ManageTenancyUserAndRoles(ctx, cli, "", "", http.MethodDelete, alertOpUser, true)
		Expect(err).ToNot(HaveOccurred())

		err = util.ManageTenancyUserAndRoles(ctx, cli, "", "", http.MethodDelete, hostUser, true)
		Expect(err).ToNot(HaveOccurred())

		By("Unblocking ENiC traffic")
		err = helpers.UnblockEnicTraffic()
		Expect(err).ToNot(HaveOccurred())

		By("Cleaning up after maintenance mode for the host")
		err = helpers.UnsetMaintenanceModeForEnic(hostApiClient, suppressContext)
		Expect(err).ToNot(HaveOccurred())

		By("Deleting alert notifications")
		err = helpers.DeleteAlertReceiverMessages(cli, mailpitURL)
		Expect(err).ToNot(HaveOccurred())

		err = mailpitPortFwdCmd.Process.Kill()
		Expect(err).ToNot(HaveOccurred())
	})
})
