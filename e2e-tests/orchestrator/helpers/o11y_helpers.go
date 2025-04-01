// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
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

func GetLogs(cli *http.Client, endpoint, query, tenant string) (logs logsResponse, err error) {
	header := make(http.Header)
	header.Add("X-Scope-OrgID", tenant)

	params := url.Values{}
	params.Add("query", query)
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
