// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/open-edge-platform/infra-core/apiv2/v2/pkg/api/v2"

	deploymentV2 "github.com/open-edge-platform/app-orch-deployment/app-deployment-manager/api/nbi/v2/pkg/restClient"
	util "github.com/open-edge-platform/edge-manageability-framework/mage"

	cm "github.com/open-edge-platform/cluster-manager/v2/pkg/api"
)

var (
	regionName              = getEnv("REGION_NAME", randomString(8))
	siteName                = getEnv("SITE_NAME", randomString(8))
	clusterName             = getEnv("CLUSTER_NAME", randomString(8))
	nodeUUID                = getEnv("NODE_UUID", "")
	edgeMgrUser             = getEnv("EDGE_MGR_USER", "robot-edge-mgr")
	edgeInfraUser           = getEnv("EDGE_INFRA_USER", "robot-api-user")
	project                 = getEnv("PROJECT", "robot-project-1")
	extensionDeploymentPath = "../samples/"
	extensionPackageName    = "baseline-extensions-lite"
	appName                 = "baseline-extension-lite"
)

const (
	apiBaseURLTemplate        = "https://api.%s/v1/projects/%s"
	clusterApiBaseURLTemplate = "https://api.%s/v2/projects/%s"
	catalogApiBaseURLTemplate = "https://api.%s/v3/projects/%s"
	extensionProfileName      = "baseline-lite"
	extensionDeploymentType   = "auto-scaling"
	extensionAppVersion       = "0.1.0"
	extensionLabelKey         = "default-extension"
	extensionLabelValue       = "baseline"
)

type Node struct {
	Id   string `json:"id"`
	Role string `json:"role"`
}

type ClusterData struct {
	Name     string            `json:"name"`
	Labels   map[string]string `json:"labels"`
	Template string            `json:"template"`
	Nodes    []Node            `json:"nodes"`
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getHosts(token string) (string, error) {
	url := fmt.Sprintf(apiBaseURLTemplate+"/compute/hosts", serviceDomain, project)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	query := req.URL.Query()
	query.Add("filter", "(desiredState=HOST_STATE_ONBOARDED OR currentState=HOST_STATE_ONBOARDED) AND instance.desiredState=INSTANCE_STATE_RUNNING AND NOT has(instance.workloadMembers)")

	query.Add("offset", "0")
	query.Add("orderBy", "name asc")
	query.Add("pageSize", "100")
	req.URL.RawQuery = query.Encode()

	req.Header.Set("Authorization", "Bearer "+token)
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func getInstanceIdForHostGuid(token, guid string) string {
	instanceId := ""
	hostsResponse, err := getHosts(token)
	if err != nil {
		return instanceId
	}
	if hostsResponse == "" {
		return instanceId
	}
	var hosts api.HostServiceListHostsResponse
	err = json.Unmarshal([]byte(hostsResponse), &hosts)
	if err != nil {
		return instanceId
	}

	if hosts.JSON200 == nil || hosts.JSON200.Hosts == nil || len(hosts.JSON200.Hosts) == 0 {
		return instanceId
	}

	for _, host := range hosts.JSON200.Hosts {
		if host.Uuid == nil {
			return instanceId
		}
		if *host.Uuid == nodeUUID {
			if host.ResourceId == nil {
				return instanceId
			}
			if host.Instance == nil || host.Instance.ResourceId == nil {
				return instanceId
			}
			instanceId = *host.Instance.ResourceId
			fmt.Printf("Matching host resource Id found: %v\n", *host.ResourceId)
			return instanceId
		}
	}
	return instanceId
}

func checkClusterStatus(token string) (string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf(clusterApiBaseURLTemplate+"/clusters?pageSize=10&offset=0", serviceDomain, project), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

var _ = Describe("Cluster Orch Smoke Test", Ordered, Label(clusterOrchSmoke), func() {
	siteID := ""
	regionID := ""
	hostID := ""
	instanceID := ""
	baseExtensionLineDeploymentId := ""
	fleetClusterId := ""

	defaultTemplate := ""
	defaultTemplateName := ""
	defaultTemplateVersion := ""
	var edgeMgrToken *string
	var edgeInfraToken *string
	var keycloakSecret string

	cli := &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: tlsConfig,
		},
	}

	BeforeAll(func() {
		defaultOrchPassword, err := util.GetDefaultOrchPassword()
		Expect(err).ToNot(HaveOccurred())

		keycloakSecret = getEnv("KEYCLOAK_SECRET", defaultOrchPassword)
		edgeMgrToken, err = util.GetApiToken(cli, edgeMgrUser, keycloakSecret)
		Expect(err).ToNot(HaveOccurred())
		Expect(edgeMgrToken).ToNot(BeNil())
		Expect(*edgeMgrToken).ToNot(BeEmpty())

		edgeInfraToken, err = util.GetApiToken(cli, edgeInfraUser, keycloakSecret)
		Expect(err).ToNot(HaveOccurred())
		Expect(edgeInfraToken).ToNot(BeNil())
		Expect(*edgeInfraToken).ToNot(BeEmpty())

		Expect(nodeUUID).ToNot(BeEmpty(), "NODE_UUID environment variable is required")
	})

	Describe("Create Region", Label(clusterOrchSmoke), func() {
		It("should create a region successfully", func() {
			data := fmt.Sprintf(`{"name":"%s","metadata":[{"key":"city","value":"%s"}]}`, regionName, regionName)
			url := fmt.Sprintf(apiBaseURLTemplate+"/regions", serviceDomain, project)

			resp, err := makeAuthorizedRequest(http.MethodPost, url, *edgeInfraToken, []byte(data), cli)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var region api.RegionResource
			err = json.Unmarshal(body, &region)
			Expect(err).ToNot(HaveOccurred())
			Expect(region.RegionID).ToNot(BeNil())
			regionID = *region.RegionID
			fmt.Printf("Region created successfully with regionID=%s\n", *region.RegionID)
		})
	})

	Describe("Create Site", Label(clusterOrchSmoke), func() {
		It("should create a site successfully", func() {
			data := fmt.Sprintf(`{"name":"%s","metadata":[],"regionId":"%s"}`, siteName, regionID)
			url := fmt.Sprintf(apiBaseURLTemplate+"/regions/%s/sites", serviceDomain, project, regionID)

			resp, err := makeAuthorizedRequest(http.MethodPost, url, *edgeInfraToken, []byte(data), cli)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var site api.SiteResource
			err = json.Unmarshal(body, &site)
			Expect(err).ToNot(HaveOccurred())
			Expect(site.SiteID).ToNot(BeNil())
			siteID = *site.SiteID
			fmt.Printf("Site created successfully with siteID=%s\n", *site.SiteID)
		})
	})

	Describe("Find Host by UUID", Label(clusterOrchSmoke), func() {
		It("should find the host with the specified UUID", func() {
			findHost := func() (bool, error) {
				hostsResponse, err := getHosts(*edgeInfraToken)
				if err != nil {
					return false, err
				}
				if hostsResponse == "" {
					return false, fmt.Errorf("hosts response is empty")
				}
				var hosts api.HostServiceListHostsResponse
				err = json.Unmarshal([]byte(hostsResponse), &hosts)
				if err != nil {
					return false, err
				}

				if hosts.JSON200 == nil || hosts.JSON200.Hosts == nil || len(hosts.JSON200.Hosts) == 0 {
					return false, fmt.Errorf("hosts list is nil")
				}

				for _, host := range hosts.JSON200.Hosts {
					if host.Uuid == nil {
						return false, fmt.Errorf("host UUID is nil")
					}

					if *host.Uuid == nodeUUID {
						if host.ResourceId == nil {
							return false, fmt.Errorf("host resource ID is nil")
						}
						hostID = *host.ResourceId
						fmt.Printf("Matching host resource Id found: %v\n", *host.ResourceId)
						return true, nil
					}
				}
				return false, nil
			}

			Eventually(func() (bool, error) {
				return findHost()
			}, 600*time.Second, 10*time.Second).Should(BeTrue(), "timeout reached. No matching hosts found.")
			Expect(hostID).ToNot(BeEmpty(), "timeout reached. No matching hosts found.")
		})
	})

	Describe("Update Host", Label(clusterOrchSmoke), func() {
		It("should update the host successfully", func() {
			data := fmt.Sprintf(`{"name":"%s","siteId":"%s","metadata":[]}`, hostID, siteID)
			url := fmt.Sprintf(apiBaseURLTemplate+"/compute/hosts/%s", serviceDomain, project, hostID)

			resp, err := makeAuthorizedRequest(http.MethodPatch, url, *edgeInfraToken, []byte(data), cli)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Or(Equal(http.StatusOK), Equal(http.StatusCreated)))
			fmt.Printf("Host updated successfully with hostID=%s\n", hostID)
		})
	})

	Describe("Get all templates", Label(clusterOrchSmoke), func() {
		url := fmt.Sprintf(clusterApiBaseURLTemplate+"/templates?default=false", serviceDomain, project)

		It("should check that cluster templates are loaded", func() {
			resp, err := makeAuthorizedRequest(http.MethodGet, url, *edgeMgrToken, nil, cli)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(body).ToNot(BeEmpty())

			var templateList cm.TemplateInfoList

			err = json.Unmarshal(body, &templateList)
			Expect(err).ToNot(HaveOccurred(), "unmarshalling response body")

			Expect(templateList).ToNot(BeNil(), "cluster template list should not be empty")
			Expect(len(*templateList.TemplateInfoList)).To(BeNumerically(">=", 3), "expecting n cluster templates")
			for _, ctpl := range *templateList.TemplateInfoList {
				Expect(ctpl.Controlplaneprovidertype).ToNot(BeNil(), "checking if controlplane provider type is not nil for cluster template %s", ctpl.Name)
				Expect(*ctpl.Controlplaneprovidertype).To(Equal(cm.Rke2), "verifying the control plane provider type for cluster template %s", ctpl.Name)
			}
		})
	})

	Describe("Get Default Template", Label(clusterOrchSmoke), func() {
		It("should retrieve the default template successfully", func() {
			url := fmt.Sprintf(clusterApiBaseURLTemplate+"/templates?default=true", serviceDomain, project)
			resp, err := makeAuthorizedRequest(http.MethodGet, url, *edgeMgrToken, nil, cli)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(body).ToNot(BeEmpty())

			var templateList cm.TemplateInfoList

			err = json.Unmarshal(body, &templateList)
			Expect(err).ToNot(HaveOccurred(), "unmarshalling response body")
			Expect(templateList.DefaultTemplateInfo).ToNot(BeNil())
			Expect(templateList.DefaultTemplateInfo.Name).ToNot(BeNil())
			Expect(*templateList.DefaultTemplateInfo.Name).ToNot(BeEmpty())
			Expect(templateList.DefaultTemplateInfo.Version).ToNot(BeEmpty())
			defaultTemplateName = *templateList.DefaultTemplateInfo.Name
			defaultTemplateVersion = templateList.DefaultTemplateInfo.Version
			defaultTemplate = defaultTemplateName + "-" + defaultTemplateVersion
			fmt.Printf("Default template retrieved successfully: template=%s, name=%s, version=%s\n",
				defaultTemplate, defaultTemplateName, defaultTemplateVersion)
		})
	})

	Describe("Clear any unwanted deployments before creating cluster", Label(clusterOrchSmoke), func() {
		It("should delete any existing deployments", func() {
			url := fmt.Sprintf(apiBaseURLTemplate+"/appdeployment/deployments?offset=0&pageSize=10", serviceDomain, project)
			resp, err := makeAuthorizedRequest(http.MethodGet, url, *edgeMgrToken, nil, cli)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(body).ToNot(BeEmpty())

			var deployments deploymentV2.ListDeploymentsResponse
			err = json.Unmarshal(body, &deployments)
			Expect(err).ToNot(HaveOccurred())
			for _, deployment := range deployments.Deployments {
				// delete the deployment using the deployment.deployId
				url := fmt.Sprintf(apiBaseURLTemplate+"/appdeployment/deployments/%s", serviceDomain, project, *deployment.DeployId)
				resp, err := makeAuthorizedRequest(http.MethodDelete, url, *edgeMgrToken, nil, cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Or(Equal(http.StatusOK), Equal(http.StatusNoContent)), fmt.Sprintf("Failed to delete deployment %s, HTTP status code: %d", *deployment.DeployId, resp.StatusCode))
				fmt.Printf("Deployment %s (%s) has been successfully deleted.\n", *deployment.Name, *deployment.DeployId)
			}
		})
	})

	Describe("Create Cluster", Label(clusterOrchSmoke), func() {
		It("should create a cluster successfully", func() {
			data := ClusterData{
				Name:     clusterName,
				Labels:   map[string]string{},
				Template: defaultTemplate,
				Nodes: []Node{
					{Id: nodeUUID, Role: "all"},
				},
			}

			jsonData, err := json.Marshal(data)
			Expect(err).ToNot(HaveOccurred())

			url := fmt.Sprintf(clusterApiBaseURLTemplate+"/clusters", serviceDomain, project)
			resp, err := makeAuthorizedRequest(http.MethodPost, url, *edgeMgrToken, jsonData, cli)

			Expect(err).ToNot(HaveOccurred())
			Expect(resp).ToNot(BeNil())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			fmt.Printf("Cluster created successfully with regionID=%s, siteID=%s, templateName=%s, uuid=%s\n", regionID, siteID, defaultTemplate, nodeUUID)
		})
	})

	Describe("Check Cluster Status", Label(clusterOrchSmoke), func() {
		It("should check the cluster status until it becomes active", func() {
			clusterActive := func() (bool, error) {
				clusterStatusResponse, err := checkClusterStatus(*edgeMgrToken)
				if err != nil {
					return false, err
				}
				var clusters map[string]interface{}
				err = json.Unmarshal([]byte(clusterStatusResponse), &clusters)
				if err != nil {
					return false, err
				}

				for _, cluster := range clusters["clusters"].([]interface{}) {
					clusterMap := cluster.(map[string]interface{})
					if clusterMap["name"] == clusterName {
						providerStatus := clusterMap["providerStatus"].(map[string]interface{})
						indicator := providerStatus["indicator"].(string)
						fmt.Printf("Cluster status: %s\n", indicator)
						if indicator == "STATUS_INDICATION_IDLE" {
							fmt.Printf("Cluster is active")
							return true, nil
						}
					}
				}
				return false, nil
			}

			Eventually(func() (bool, error) {
				return clusterActive()
			}, 1200*time.Second, 10*time.Second).Should(BeTrue(), "timeout reached. Cluster did not become active")
		})
	})

	Describe("Attempt to Delete Cluster Template in Use", Label(clusterOrchSmoke), func() {
		It("should fail to delete the cluster template while it is in use", func() {
			Expect(defaultTemplateName).ToNot(BeEmpty(), "Default template name should not be empty")
			Expect(defaultTemplateVersion).ToNot(BeEmpty(), "Default template version should not be empty")

			// Attempt to delete the cluster template
			url := fmt.Sprintf(clusterApiBaseURLTemplate+"/templates/%s/versions/%s", serviceDomain, project, defaultTemplateName, defaultTemplateVersion)
			resp, err := makeAuthorizedRequest(http.MethodDelete, url, *edgeMgrToken, nil, cli)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			fmt.Println(string(body))

			// Expect the request to fail with a 409 Conflict status code
			Expect(resp.StatusCode).To(Equal(http.StatusConflict), "Expected 409 Conflict when deleting a template in use")
			Expect(body).To(ContainSubstring("clusterTemplate is in use"))
			fmt.Printf("Failed to delete template %s-%s as expected, HTTP status code: %d\n", defaultTemplateName, defaultTemplateVersion, resp.StatusCode)
		})
	})

	Describe("Get Fleet Clusters", Label(clusterOrchSmoke), func() {
		It("should get the fleet clusters successfully", func() {
			url := fmt.Sprintf(apiBaseURLTemplate+"/appdeployment/clusters", serviceDomain, project)

			// Initiate the GET request using makeAuthorizedRequest
			resp, err := makeAuthorizedRequest(http.MethodGet, url, *edgeMgrToken, nil, cli)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			// Expect 200 OK response
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			// Decode the GET response to deploymentV2.ListClustersResponse
			var listClustersResponse deploymentV2.ListClustersResponse
			body, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			err = json.Unmarshal(body, &listClustersResponse)
			Expect(err).ToNot(HaveOccurred())

			// Get the cluster ID that matches the cluster name
			for _, cluster := range listClustersResponse.Clusters {
				if *cluster.Name == clusterName {
					fleetClusterId = *cluster.Id
					break
				}
			}
			Expect(fleetClusterId).ToNot(BeEmpty())
			fmt.Printf("Fleet Cluster ID: %s\n", fleetClusterId)
		})
	})

	Describe("Load baseline lite deployment package", Label(clusterOrchSmoke), func() {
		It("should load the baseline lite deployment package successfully", func() {
			paths := []string{
				extensionDeploymentPath + extensionPackageName,
			}
			// Upload the files
			err := util.UploadFiles(paths, serviceDomain, project, edgeMgrUser, keycloakSecret)
			Expect(err).ToNot(HaveOccurred())

			type CatalogDeploymentPackagesResp struct {
				DeploymentPackages []struct {
					Name string `json:"name"`
				}
			}

			var catalog CatalogDeploymentPackagesResp

			// List the deployment packages
			url := fmt.Sprintf(catalogApiBaseURLTemplate+"/catalog/deployment_packages", serviceDomain, project)
			resp, err := makeAuthorizedRequest(http.MethodGet, url, *edgeMgrToken, nil, cli)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			body, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			err = json.Unmarshal(body, &catalog)
			Expect(err).ToNot(HaveOccurred())

			dpLoaded := func() (bool, error) {
				for _, dp := range catalog.DeploymentPackages {
					if dp.Name == extensionPackageName {
						return true, nil
					}
				}
				return false, nil
			}

			Eventually(func() (bool, error) {
				return dpLoaded()
			}, 1*time.Minute, 10*time.Second).Should(BeTrue(), "timeout reached. Did not find baseline lite deployment package")
		})
	})

	Describe("Create a deployment using the baseline lite package", Label(clusterOrchSmoke), func() {
		It("should create a deployment using the baseline lite package successfully", func() {
			url := fmt.Sprintf(apiBaseURLTemplate+"/appdeployment/deployments", serviceDomain, project)
			targetClusters := deploymentV2.TargetClusters{
				AppName: &appName,
				Labels: &map[string]string{
					extensionLabelKey: extensionLabelValue,
				},
			}
			profileName := extensionProfileName
			displayName := extensionPackageName
			deploymentType := extensionDeploymentType

			// Create the request body using deploymentV2.Deployment
			requestBody := deploymentV2.Deployment{
				AppName:     extensionPackageName,
				AppVersion:  extensionAppVersion,
				ProfileName: &profileName,
				TargetClusters: &[]deploymentV2.TargetClusters{
					targetClusters,
				},
				DisplayName:    &displayName,
				DeploymentType: &deploymentType,
				OverrideValues: nil,
			}

			// Encode the request body to JSON
			jsonData, err := json.Marshal(requestBody)
			Expect(err).ToNot(HaveOccurred())

			// Create the HTTP POST request using makeAuthorizedRequest
			resp, err := makeAuthorizedRequest(http.MethodPost, url, *edgeMgrToken, jsonData, cli)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			// Check for expected 200 OK response
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			// Decode the response body into deploymentV2.CreateDeploymentResponse
			var createDeploymentResponse deploymentV2.CreateDeploymentResponse
			body, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			err = json.Unmarshal(body, &createDeploymentResponse)
			Expect(err).ToNot(HaveOccurred())

			// Store the DeploymentId for future use
			baseExtensionLineDeploymentId = createDeploymentResponse.DeploymentId
			Expect(baseExtensionLineDeploymentId).ToNot(BeEmpty())
			fmt.Printf("Deployment ID: %s\n", baseExtensionLineDeploymentId)
		})
	})

	Describe("Wait for Extension Deployment to be Ready", func() {
		It("should wait for the extension deployment to be ready", func() {
			extensionReady := func() (bool, error) {
				url := fmt.Sprintf(apiBaseURLTemplate+"/appdeployment/deployments/%s", serviceDomain, project, baseExtensionLineDeploymentId)

				// Initiate the GET request using makeAuthorizedRequest
				resp, err := makeAuthorizedRequest(http.MethodGet, url, *edgeMgrToken, nil, cli)
				if err != nil {
					return false, err
				}
				defer resp.Body.Close()

				// Expect 200 OK response
				if resp.StatusCode != http.StatusOK {
					return false, fmt.Errorf("failed to get cluster info, HTTP status code: %d", resp.StatusCode)
				}

				// Decode the GET response to deploymentV2.GetDeploymentResponse
				var deployment deploymentV2.GetDeploymentResponse
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return false, err
				}
				if err = json.Unmarshal(body, &deployment); err != nil {
					return false, err
				}

				if deployment.Deployment.Status == nil || deployment.Deployment.Status.State == nil {
					return false, fmt.Errorf("deployment status or state is nil")
				}

				// Use Eventually function to wait for deployment to be State_RUNNING
				if *deployment.Deployment.Status.State != deploymentV2.RUNNING {
					return false, nil
				}
				fmt.Printf("Deployment is ready\n")
				return true, nil
			}

			Eventually(func() (bool, error) {
				return extensionReady()
			}, 600*time.Second, 10*time.Second).Should(BeTrue(), "timeout reached. deployment not in RUNNING state.")
		})
	})

	// Cleanup after the test suite
	AfterAll(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		done := make(chan struct{})

		go func() {
			defer GinkgoRecover()
			defer close(done) // End fast in case assertion failed. TODO: refactor this.

			if clusterName != "" {
				Expect(edgeMgrToken).ToNot(BeNil())
				url := fmt.Sprintf(clusterApiBaseURLTemplate+"/clusters/%s", serviceDomain, project, clusterName)
				resp, err := makeAuthorizedRequest(http.MethodDelete, url, *edgeMgrToken, nil, cli)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusNoContent))
				fmt.Printf("Cluster deleted successfully with clusterName=%s\n", clusterName)

				// Wait for cluster to get deleted before releasing resource to Edge Infrastructure Manager
				clusterDeleted := func() (bool, error) {
					resp, err := makeAuthorizedRequest(http.MethodGet, fmt.Sprintf(clusterApiBaseURLTemplate+"/clusters", serviceDomain, project), *edgeMgrToken, nil, cli)
					if err != nil {
						return false, err
					}
					defer resp.Body.Close()
					body, err := io.ReadAll(resp.Body)
					if err != nil {
						return false, err
					}

					var clusters map[string]interface{}
					err = json.Unmarshal(body, &clusters)
					if err != nil {
						return false, err
					}

					for _, cluster := range clusters["clusters"].([]interface{}) {
						clusterMap := cluster.(map[string]interface{})
						if clusterMap["name"] == clusterName {
							return false, nil
						}
					}

					fmt.Printf("Cluster %s has been successfully deleted.\n", clusterName)
					return true, nil
				}

				Eventually(func() (bool, error) {
					return clusterDeleted()
				}, 140*time.Second, 5*time.Second).Should(BeTrue(), fmt.Sprintf("Cluster %s was not deleted within the expected time frame.", clusterName))
			}

			// Delete the instance
			if instanceID == "" {
				instanceID = getInstanceIdForHostGuid(*edgeInfraToken, nodeUUID)
			}
			if instanceID != "" {
				url := fmt.Sprintf(apiBaseURLTemplate+"/compute/instances/%s", serviceDomain, project, instanceID)
				resp, err := makeAuthorizedRequest(http.MethodDelete, url, *edgeInfraToken, nil, cli)
				Expect(err).ToNot(HaveOccurred(), "creating new HTTP request")
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Or(Equal(http.StatusOK), Equal(http.StatusNoContent)), fmt.Sprintf("Failed to delete instance %s, HTTP status code: %d", instanceID, resp.StatusCode))
				fmt.Printf("Instance %s has been successfully deleted.\n", instanceID)
			}

			// Delete the host
			if hostID != "" {
				url := fmt.Sprintf(apiBaseURLTemplate+"/compute/hosts/%s", serviceDomain, project, hostID)
				resp, err := makeAuthorizedRequest(http.MethodDelete, url, *edgeInfraToken, nil, cli)
				Expect(err).ToNot(HaveOccurred(), "creating new HTTP request")
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Or(Equal(http.StatusOK), Equal(http.StatusNoContent)), fmt.Sprintf("Failed to delete host %s, HTTP status code: %d", hostID, resp.StatusCode))
				fmt.Printf("Host %s has been successfully deleted.\n", hostID)
			}

			// Delete the site
			if siteID != "" {
				Expect(regionID).ToNot(BeEmpty())
				Expect(siteID).ToNot(BeEmpty())

				siteDeleted := func() bool {
					url := fmt.Sprintf(apiBaseURLTemplate+"/regions/%s/sites/%s", serviceDomain, project, regionID, siteID)
					resp, err := makeAuthorizedRequest(http.MethodDelete, url, *edgeInfraToken, nil, cli)
					if err != nil {
						fmt.Printf("Error creating new HTTP request: %v\n", err)
						return false
					}
					defer resp.Body.Close()

					if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
						fmt.Printf("Site %s has been successfully deleted.\n", siteID)
						return true
					} else {
						return false
					}
				}

				Eventually(siteDeleted, 50*time.Second, 5*time.Second).Should(BeTrue(), fmt.Sprintf("Failed to delete site %s within the expected time frame.", siteID))
			}

			// Delete the region
			if regionID != "" {
				regionDeleted := func() bool {
					url := fmt.Sprintf(apiBaseURLTemplate+"/regions/%s", serviceDomain, project, regionID)
					resp, err := makeAuthorizedRequest(http.MethodDelete, url, *edgeInfraToken, nil, cli)
					if err != nil {
						fmt.Printf("Error creating new HTTP request: %v\n", err)
						return false
					}
					defer resp.Body.Close()

					if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
						fmt.Printf("Region %s has been successfully deleted.\n", regionID)
						return true
					} else {
						return false
					}
				}

				Eventually(regionDeleted, 50*time.Second, 5*time.Second).Should(BeTrue(), fmt.Sprintf("Failed to delete region %s within the expected time frame.", regionID))
			}
		}()
		select {
		case <-done:
			// Cleanup completed within the timeout or marked as Failed by assertion
		case <-ctx.Done():
			// Timeout occurred
			Fail("AfterAll block timed out")
		}
	})
})
