// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	. "github.com/onsi/gomega"
)

type (
	Deployment struct {
		Name           string        `json:"name"`
		DisplayName    string        `json:"displayName"`
		AppName        string        `json:"appName"`
		AppVersion     string        `json:"appVersion"`
		ProfileName    string        `json:"profileName"`
		PublisherName  string        `json:"publisherName"`
		CreateTime     time.Time     `json:"createTime"`
		DeployId       string        `json:"deployId"`
		OverrideValues []interface{} `json:"overrideValues"`
		TargetClusters []struct {
			AppName string `json:"appName"`
			Labels  struct {
				ClusterOrchestrationIoProjectId string `json:"cluster.orchestration.io/project-id"`
				DefaultExtension                string `json:"default-extension"`
			} `json:"labels"`
			ClusterId string `json:"clusterId"`
		} `json:"targetClusters"`
		Status struct {
			State   string `json:"state"`
			Message string `json:"message"`
			Summary struct {
				Total   int    `json:"total"`
				Running int    `json:"running"`
				Down    int    `json:"down"`
				Type    string `json:"type"`
				Unknown int    `json:"unknown"`
			} `json:"summary"`
		} `json:"status"`
		Apps                 []interface{} `json:"apps"`
		DefaultProfileName   string        `json:"defaultProfileName"`
		DeploymentType       string        `json:"deploymentType"`
		NetworkName          string        `json:"networkName"`
		ServiceExports       []interface{} `json:"serviceExports"`
		AllAppTargetClusters interface{}   `json:"allAppTargetClusters"`
	}
	ListDeploymentsResponse struct {
		Deployments   []Deployment `json:"deployments"`
		TotalElements int          `json:"totalElements"`
	}
)

// constructADMURL constructs a URL for the catalog service give a project and an object
func constructADMURL(projectName string, endpoint string) string {
	retval := fmt.Sprintf("%s/%s/projects/%s/appdeployment", apiBaseURL, ADMApiVersion, projectName)
	if endpoint != "" {
		retval = retval + "/" + endpoint
	}
	return retval
}

// doADMRest executes a REST API request to the app deployment manager service
func doADMREST(
	ctx context.Context,
	c *http.Client,
	method string,
	endpoint string,
	projectName string,
	accessToken string,
	body io.Reader,
	expectedStatus int,
	ignoreResponse bool,
) *http.Response {
	ADMURL := constructADMURL(projectName, endpoint)
	return doREST(ctx, c, method, ADMURL, accessToken, body, expectedStatus, ignoreResponse)
}

// listDeploymentsByDisplayName use the ADM REST API to query a deployment
func listDeploymentsByDisplayName(
	ctx context.Context,
	c *http.Client,
	accessToken string,
	project string,
	displayName string,
	expectedStatus int,
	ignoreResponse bool,
) []Deployment {
	resp := doADMREST(
		ctx,
		c,
		http.MethodGet,
		"deployments?displayName="+displayName,
		project,
		accessToken,
		nil,
		expectedStatus,
		ignoreResponse,
	)
	defer resp.Body.Close()

	var deploymentsResponse ListDeploymentsResponse
	err := json.NewDecoder(resp.Body).Decode(&deploymentsResponse)
	Expect(err).ToNot(HaveOccurred())

	for i, deployment := range deploymentsResponse.Deployments {
		fmt.Printf("Deployment Response %d: %s\n", i, deployment.ProfileName)
	}
	Expect(deploymentsResponse.Deployments).To(HaveLen(3), "checking Deployment %s", displayName)
	return deploymentsResponse.Deployments
}

// getDeploymentByDisplayNameAndProfileName finds a specific deployment by its display name and profile name
func getDeploymentByDisplayNameAndProfileName(
	ctx context.Context,
	c *http.Client,
	accessToken string,
	project string,
	displayName string,
	profileName string,
	expectedStatus int,
	ignoreResponse bool,
) Deployment {
	var retval Deployment
	deployments := listDeploymentsByDisplayName(ctx, c, accessToken, project, displayName, expectedStatus, ignoreResponse)
	for _, deployment := range deployments {
		if deployment.ProfileName == profileName {
			retval = deployment
		}
	}
	return retval
}
