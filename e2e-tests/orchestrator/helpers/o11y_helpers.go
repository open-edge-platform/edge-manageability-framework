// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"slices"
	"strings"
	"time"

	invapi "github.com/open-edge-platform/infra-core/api/pkg/api/v0"
)

const (
	addRegionBody             = `{"name":"en-obs-region","metadata":[{"key":"region","value":"en-obs-region"}]}`
	addSiteBodyTemplate       = `{"name":"en-obs-site","metadata":[],"regionId":"%s"}`
	configureHostBodyTemplate = `{"name":"%s","siteId":"%s","metadata":[]}`
	postHostScheduleTemplate  = `{"name":"en-obs-schedule","scheduleStatus":"SCHEDULE_STATUS_MAINTENANCE","targetHostId":"%s","startSeconds":%d,"endSeconds":%d}`
)

func CheckMetric(cli *http.Client, endpoint, metric, tenant string) (found bool, err error) {
	header := make(http.Header)
	header.Add("X-Scope-OrgID", tenant)
	endpoint += "?query=" + metric
	resp, err := MakeRequest(http.MethodGet, endpoint, nil, cli, header)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("endpoint returned non 200 status, returned code: %v", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	var metrics metricsResponse
	err = json.Unmarshal(content, &metrics)
	if err != nil {
		return false, err
	}

	return len(metrics.Data.Result) != 0, nil
}

func GetLogs(cli *http.Client, endpoint, query, since, tenant string) (logs logsResponse, err error) {
	header := make(http.Header)
	header.Add("X-Scope-OrgID", tenant)

	params := url.Values{}
	params.Add("query", query)
	if since != "" {
		params.Add("since", since)
	}
	endpoint += "?" + params.Encode()

	resp, err := MakeRequest(http.MethodGet, endpoint, nil, cli, header)
	if err != nil {
		return logs, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return logs, fmt.Errorf("endpoint returned non 200 status, returned code: %v", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return logs, err
	}

	err = json.Unmarshal(content, &logs)
	if err != nil {
		return logs, err
	}

	return logs, nil
}

// It is caller's responsibility to close the response body.
func MakeRequest(method, address string, body []byte, cli *http.Client, headers http.Header) (*http.Response, error) {
	req, err := http.NewRequest(method, address, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	if headers != nil {
		req.Header = headers
	}
	return cli.Do(req)
}

func PatchAlertDefinitions(cli *http.Client, serviceDomainWithPort, token, projectName string, definitions AlertDefinitionsArray) error {
	for _, definition := range definitions.AlertDefinitions {
		patchJSON := PatchDefinitionBody{}
		patchJSON.Values = definition.Values
		reqBody, err := json.Marshal(patchJSON)
		if err != nil {
			return err
		}

		endpoint := fmt.Sprintf("https://api."+serviceDomainWithPort+"/v1/projects/"+projectName+"/alerts/definitions/%v",
			definition.ID)
		resp, err := makeAuthorizedRequest(cli, http.MethodPatch, endpoint, token, reqBody)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("endpoint returned non 204 status, returned code: %v", resp.StatusCode)
		}
		resp.Body.Close()
	}
	return nil
}

func UnblockEnicTraffic() error {
	// delete network policy
	cmd := exec.Command("kubectl", "delete", "NetworkPolicy", "deny-all-traffic", "-n", "enic", "--ignore-not-found")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete network policy: %w, output: %s", err, output)
	}
	return nil
}

func BlockEnicTraffic() error {
	const networkPolicy = `
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: deny-all-traffic
  namespace: enic
spec:
  podSelector:
    matchLabels:
      app: enic
  policyTypes:
  - Egress
  - Ingress
`

	// apply network policy
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = bytes.NewBufferString(networkPolicy)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to apply network policy: %w, output: %s", err, output)
	}

	// restart node-agent in enic
	cmd = exec.Command("kubectl", "exec", "enic-0", "-n", "enic", "-c", "edge-node", "--", "systemctl", "restart", "node-agent")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restart node-agent: %w, output: %s", err, output)
	}
	return nil
}

func GetEnicUUID() (string, error) {
	cmd := exec.Command("kubectl", "exec", "enic-0", "-n", "enic", "-c", "edge-node", "--", "dmidecode", "-s", "system-uuid")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get ENiC UUID: %w, output: %s", err, output)
	}
	enicUUID := strings.TrimSpace(string(output))
	if enicUUID == "" {
		return "", errors.New("empty enic UUID")
	}

	return enicUUID, nil
}

func IsEmailEnabled(cli *http.Client, serviceDomainWithPort, token, projectName, receiverID, email string) (bool, error) {
	resp, err := makeAuthorizedRequest(cli, http.MethodGet,
		"https://api."+serviceDomainWithPort+"/v1/projects/"+projectName+"/alerts/receivers/"+receiverID, token, nil)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("endpoint returned non 200 status, returned code: %v", resp.StatusCode)
	}

	receiver := new(AlertReceiver)
	err = ParseJSONBody(resp.Body, receiver)
	if err != nil {
		return false, err
	}
	return slices.Contains(receiver.EmailConfig.To.Enabled, email), nil
}

func VerifyReceiverState(cli *http.Client, serviceDomainWithPort, token, projectName, receiverID, state string) error {
	resp, err := makeAuthorizedRequest(cli, http.MethodGet,
		"https://api."+serviceDomainWithPort+"/v1/projects/"+projectName+"/alerts/receivers/"+receiverID, token, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("endpoint returned non 200 status, status returned: %v", resp.StatusCode)
	}

	receiver := new(AlertReceiver)
	err = ParseJSONBody(resp.Body, receiver)
	if err != nil {
		return err
	}

	if receiver.State != state {
		return fmt.Errorf("receiver not applied yet, current state: %v", receiver.State)
	}
	return nil
}

func NewAPIClient(httpClient *http.Client, serviceDomainWithPort, projectName, token string) *APIClient {
	return &APIClient{
		HTTPClient:            httpClient,
		ServiceDomainWithPort: serviceDomainWithPort,
		ProjectName:           projectName,
		Token:                 token,
	}
}

// MakeAPICallParseResp makes an API call to the given endpoint and parses the response body into the target object.
func (c *APIClient) MakeAPICallParseResp(method, path string, body []byte, target any) (int, error) {
	endpointURL := "https://api." + c.ServiceDomainWithPort + "/v1/projects/" + c.ProjectName + path
	resp, err := makeAuthorizedRequest(c.HTTPClient, method, endpointURL, c.Token, body)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("endpoint returned non 2xx status, returned code: %v", resp.StatusCode)
	}

	if target != nil {
		err = ParseJSONBody(resp.Body, target)
		return resp.StatusCode, err
	}

	return resp.StatusCode, nil
}

func GetAlertNames(client *APIClient, queryParams ...string) ([]string, error) {
	queryString := ""
	if len(queryParams) > 0 {
		queryString = "?" + strings.Join(queryParams, "&")
	}
	endpoint := "/alerts" + queryString
	alerts := new(Alerts)
	statusCode, err := client.MakeAPICallParseResp(http.MethodGet, endpoint, nil, alerts)
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("endpoint returned non 200 status, returned code: %v", statusCode)
	}

	alertsNames := make([]string, 0, len(alerts.Alerts))
	for _, alert := range alerts.Alerts {
		alertsNames = append(alertsNames, alert.Labels.Alertname)
	}
	return alertsNames, nil
}

func GetHostByUUID(client *APIClient, uuid string) (string, error) {
	if client == nil {
		return "", errors.New("API client is nil")
	}
	if uuid == "" {
		return "", errors.New("empty UUID")
	}

	// one host is expected in o11y tests, no need to check the pagination
	hostsList := new(invapi.HostsList)
	statusCode, err := client.MakeAPICallParseResp(http.MethodGet, "/compute/hosts", nil, hostsList)
	if err != nil {
		return "", fmt.Errorf("error accessing get hosts API endpoint: %w", err)
	}
	if statusCode != http.StatusOK {
		return "", fmt.Errorf("get hosts API endpoint returned non 200 status, returned code: %v", statusCode)
	}

	for _, host := range *hostsList.Hosts {
		if host.Uuid.String() == uuid {
			hostID := *host.ResourceId
			if hostID == "" {
				return "", errors.New("empty host resource ID")
			}
			return hostID, nil
		}
	}
	return "", fmt.Errorf("host with UUID %v not found", uuid)
}

func SetMaintenanceModeForEnic(apiClient *APIClient, context *MaintenanceModeContext) error {
	if apiClient == nil {
		return errors.New("API client is nil")
	}
	if context == nil {
		return errors.New("maintenance mode context is nil")
	}

	hostUUID, err := GetEnicUUID()
	if err != nil {
		return fmt.Errorf("error when getting the host UUID for enic: %w", err)
	}

	regionResponse := GenericCreateResourceResponse{}
	statusCode, err := apiClient.MakeAPICallParseResp(http.MethodPost, "/regions", []byte(addRegionBody), &regionResponse)
	if err != nil {
		return fmt.Errorf("error when adding a region: %w", err)
	}
	if statusCode != http.StatusCreated {
		return fmt.Errorf("when adding a region expected status code %v, got %v", http.StatusCreated, statusCode)
	}
	// get region resource ID
	context.RegionResourceID = regionResponse.ResourceID
	if context.RegionResourceID == "" {
		return fmt.Errorf("empty region resource ID when adding a region")
	}

	siteResponse := GenericCreateResourceResponse{}
	addSiteBody := fmt.Sprintf(addSiteBodyTemplate, context.RegionResourceID)
	statusCode, err = apiClient.MakeAPICallParseResp(http.MethodPost, "/regions/"+context.RegionResourceID+"/sites", []byte(addSiteBody), &siteResponse)
	if err != nil {
		return fmt.Errorf("error when adding a site to the region: %w", err)
	}
	if statusCode != http.StatusCreated {
		return fmt.Errorf("when adding a site to the region expected status code %v, got %v", http.StatusCreated, statusCode)
	}
	// get site resource ID
	context.SiteResourceID = siteResponse.ResourceID
	if context.SiteResourceID == "" {
		return fmt.Errorf("empty site resource ID when adding a site to the region")
	}

	context.HostID, err = GetHostByUUID(apiClient, hostUUID)
	if err != nil {
		return fmt.Errorf("error when getting the host ID for enic: %w", err)
	}

	configureHostBody := fmt.Sprintf(configureHostBodyTemplate, context.HostID, context.SiteResourceID)
	statusCode, err = apiClient.MakeAPICallParseResp(http.MethodPatch, "/compute/hosts/"+context.HostID, []byte(configureHostBody), nil)
	if err != nil {
		return fmt.Errorf("error when configuring the host: %w", err)
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("when configuring the host expected status code %v, got %v", http.StatusOK, statusCode)
	}

	scheduleResponse := GenericCreateResourceResponse{}
	timeStart := time.Now().Add(1 * time.Minute).Unix()
	timeEnd := time.Now().Add(1 * time.Hour).Unix()
	postHostScheduleBody := fmt.Sprintf(postHostScheduleTemplate, context.HostID, timeStart, timeEnd)
	statusCode, err = apiClient.MakeAPICallParseResp(http.MethodPost, "/schedules/single", []byte(postHostScheduleBody), &scheduleResponse)
	if err != nil {
		return fmt.Errorf("error when scheduling maintenance mode for the host: %w", err)
	}
	if statusCode != http.StatusCreated {
		return fmt.Errorf("when scheduling maintenance mode for the host expected status code %v, got %v", http.StatusCreated, statusCode)
	}
	context.ScheduleID = scheduleResponse.ResourceID
	if context.ScheduleID == "" {
		return errors.New("empty schedule response ID when scheduling maintenance mode for the host")
	}

	return nil
}

func UnsetMaintenanceModeForEnic(apiClient *APIClient, context *MaintenanceModeContext) error {
	if apiClient == nil {
		return errors.New("API client is nil")
	}
	if context == nil {
		return errors.New("maintenance mode context is nil")
	}

	if context.ScheduleID != "" {
		statusCode, err := apiClient.MakeAPICallParseResp(http.MethodDelete, "/schedules/single/"+context.ScheduleID, nil, nil)
		if err != nil {
			return fmt.Errorf("error when deleting the schedule for the enic host: %w", err)
		}
		if statusCode != http.StatusNoContent {
			return fmt.Errorf("when deleting the schedule for the enic host expected status code %v, got %v", http.StatusNoContent, statusCode)
		}
	}

	if context.HostID != "" {
		configureHostBody := fmt.Sprintf(configureHostBodyTemplate, context.HostID, "")
		statusCode, err := apiClient.MakeAPICallParseResp(http.MethodPatch, "/compute/hosts/"+context.HostID, []byte(configureHostBody), nil)
		if err != nil {
			return fmt.Errorf("error when unconfiguring the enic host: %w", err)
		}
		if statusCode != http.StatusOK {
			return fmt.Errorf("when unconfiguring the enic host expected status code %v, got %v", http.StatusOK, statusCode)
		}
	}

	if context.SiteResourceID != "" && context.RegionResourceID != "" {
		statusCode, err := apiClient.MakeAPICallParseResp(http.MethodDelete, "/regions/"+context.RegionResourceID+"/sites/"+context.SiteResourceID, nil, nil)
		if err != nil {
			return fmt.Errorf("error when deleting site: %w", err)
		}
		if statusCode != http.StatusNoContent {
			return fmt.Errorf("when deleting site expected status code %v, got %v", http.StatusNoContent, statusCode)
		}
	}

	if context.RegionResourceID != "" {
		statusCode, err := apiClient.MakeAPICallParseResp(http.MethodDelete, "/regions/"+context.RegionResourceID, nil, nil)
		if err != nil {
			return fmt.Errorf("error when deleting region: %w", err)
		}
		if statusCode != http.StatusNoContent {
			return fmt.Errorf("when deleting region expected status code %v, got %v", http.StatusNoContent, statusCode)
		}
	}

	return nil
}

func GetAlertReceiverMessages(cli *http.Client, mailpitURL string) ([]string, error) {
	messagesURL := mailpitURL + "/api/v1/messages"
	resp, err := MakeRequest(http.MethodGet, messagesURL, nil, cli, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("endpoint returned non 200 status, status returned: %v", resp.StatusCode)
	}

	mails := new(MailList)
	err = ParseJSONBody(resp.Body, mails)
	if err != nil {
		return nil, err
	}
	if mails.Total == 0 {
		return nil, errors.New("no mails received yet")
	}
	mailContents := make([]string, 0, mails.Total)

	for _, mail := range mails.Messages {
		mailContent, err := func(mailID string) (string, error) {
			messageURL := fmt.Sprintf("%s/api/v1/message/%s", mailpitURL, mailID)
			resp, err := MakeRequest(http.MethodGet, messageURL, nil, cli, nil)
			if err != nil {
				return "", err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return "", fmt.Errorf("message endpoint returned non 200 status, status returned: %v", resp.StatusCode)
			}

			content, err := io.ReadAll(resp.Body)
			if err != nil {
				return "", fmt.Errorf("parsing mail message body failed: %w", err)
			}

			return string(content), nil
		}(mail.ID)
		if err != nil {
			return nil, err
		}
		mailContents = append(mailContents, mailContent)
	}

	return mailContents, nil
}

func DeleteAlertReceiverMessages(cli *http.Client, mailpitURL string) error {
	messagesURL := mailpitURL + "/api/v1/messages"
	resp, err := MakeRequest(http.MethodDelete, messagesURL, nil, cli, nil)
	if err != nil {
		return fmt.Errorf("error when deleting messages: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("endpoint returned non 200 status, status returned: %v", resp.StatusCode)
	}
	return nil
}
