// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/bitfield/script"
)

const (
	baselineClusterTemplatePath = "./node/capi/baseline.json"
	defaultTemplate             = "baseline-v2.0.2"
	clusterApiBaseURLTemplate   = "https://api.%s/v2/projects/%s"
)

var (
	edgeMgrUser = getEnv("EDGE_MGR_USER", "sample-project-edge-mgr")
	project     = getEnv("PROJECT", "sample-project")
)

// TODO replace with open-edge-platform/cluster-manager types
type Node struct {
	ID   string `json:"id"`
	Role string `json:"role"`
}

type ClusterConfig struct {
	Name     string            `json:"name"`
	Template string            `json:"template"`
	Nodes    []Node            `json:"nodes"`
	Labels   map[string]string `json:"labels"`
}

func createCoreDNSConfigMap() error {
	// Get gateway IP
	primaryIP, err := getPrimaryIP()
	gatewayIP := primaryIP.String()
	if err != nil {
		return err
	}

	// Get CoreDNS config
	corednsConfig, err := getCoreDNSConfigMap("default", gatewayIP)
	if err != nil {
		return err
	}

	// Create the ConfigMap
	fmt.Println("Creating CoreDNS ConfigMap")
	output, err := script.Echo(string(corednsConfig)).Exec("kubectl apply -f -").String()
	if err != nil {
		return err
	}
	fmt.Println(output)
	return nil
}

func (cu CoUtils) CreateDefaultClusterTemplate() error {
	fmt.Println("Create CoreDNS ConfigMap referenced by cluster template")
	err := createCoreDNSConfigMap()
	if err != nil {
		return err
	}

	fmt.Println("Create default cluster template in project:", project)

	data, err := os.ReadFile(baselineClusterTemplatePath)
	if err != nil {
		return err
	}

	fmt.Println("POST request to cluster-manager")
	path := "/templates?default=false"
	url := fmt.Sprintf(clusterApiBaseURLTemplate+path, serviceDomain, project)
	resp, err := makeAuthorizedRequest("POST", url, data, &http.Client{})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create default template for project '%s': %s", project, string(body))
	}

	err = cu.SetDefaultTemplate("baseline", "v2.0.0")
	if err != nil {
		return err
	}

	return nil
}

func (CoUtils) CreateCluster(clusterName, nodeGUID string) error {
	fmt.Println("Create cluster with name:", clusterName, "and node id:", nodeGUID, "in project:", project)

	data, err := fillClusterConfig(clusterName, defaultTemplate, nodeGUID)
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	fmt.Println("POST request to cluster-manager")
	path := "/clusters"
	url := fmt.Sprintf(clusterApiBaseURLTemplate+path, serviceDomain, project)
	resp, err := makeAuthorizedRequest("POST", url, data, &http.Client{})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create cluster %s in project '%s': %s", clusterName, project, string(body))
	}

	return nil
}

func (CoUtils) DeleteCluster(clusterName string) error {
	fmt.Println("Delete cluster with name:", clusterName, "in project:", project)

	path := "/clusters/" + clusterName
	url := fmt.Sprintf(clusterApiBaseURLTemplate+path, serviceDomain, project)
	resp, err := makeAuthorizedRequest("DELETE", url, nil, &http.Client{})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to delete cluster %s in project '%s': %s", clusterName, project, string(body))
	}

	return nil
}

func (CoUtils) SetDefaultTemplate(templateName, templateVersion string) error {
	fmt.Printf("Set template %s in project %s as default\n", templateName, project)

	reqBody := map[string]string{
		"name":    templateName,
		"version": templateVersion,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/%s/default", templateName)
	url := fmt.Sprintf(clusterApiBaseURLTemplate+path, serviceDomain, project)
	resp, err := makeAuthorizedRequest("PUT", url, data, &http.Client{})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to set template %s as default in project '%s': %s", templateName, project, string(body))
	}

	fmt.Printf("Template %s set as default in project %s\n", templateName, project)
	return nil
}

func fillClusterConfig(clusterName, templateName, nodeGUID string) ([]byte, error) {
	config := ClusterConfig{
		Name:     clusterName,
		Template: templateName,
		Nodes: []Node{
			{
				ID:   nodeGUID,
				Role: "all",
			},
		},
		Labels: map[string]string{
			"users-label": "user-value",
		},
	}

	// Convert to JSON
	jsonData, err := json.Marshal(config)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return []byte{}, nil
	}

	return jsonData, nil
}

func getCoreDNSConfigMap(namespace, gatewayIP string) ([]byte, error) {
	data, err := os.ReadFile("./node/capi/coredns-config.yaml")
	if err != nil {
		return nil, err
	}

	data = bytes.ReplaceAll(data, []byte("NAMESPACE"), []byte(namespace))
	data = bytes.ReplaceAll(data, []byte("ORCHESTRATOR_IP"), []byte(gatewayIP))
	return data, nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func makeAuthorizedRequest(method, url string, body []byte, cli *http.Client) (*http.Response, error) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defaultOrchPassword, err := GetDefaultOrchPassword()
	if err != nil {
		return nil, err
	}
	keycloakSecret := getEnv("KEYCLOAK_SECRET", defaultOrchPassword)
	token, err := GetApiToken(cli, edgeMgrUser, keycloakSecret)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+*token)
	req.Header.Add("Content-Type", "application/json")
	return cli.Do(req)
}
