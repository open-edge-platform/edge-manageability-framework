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
	"os/exec"
	"time"

	"github.com/bitfield/script"
)

const (
	baselineClusterTemplatePath = "./node/capi/baseline.json"
	defaultTemplate             = "baseline-v0.0.1"
	portForwardAddress          = "0.0.0.0"
	portForwardService          = "svc/cluster-manager"
	portForwardServiceNamespace = "orch-cluster"
	portForwardLocalPort        = "8080"
	portForwardRemotePort       = "8080"
	clusterTemplateURL          = "http://127.0.0.1:8080/v2/templates"
	clusterCreateURL            = "http://127.0.0.1:8080/v2/clusters"
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

// creates default cluster template from file in given namespace <namespace>
func (cu CoUtils) CreateDefaultClusterTemplate(namespace string) error {
	fmt.Println("Create CoreDNS ConfigMap referenced by cluster template")
	err := createCoreDNSConfigMap()
	if err != nil {
		return err
	}

	fmt.Println("Create default cluster template in namespace:", namespace)
	cmd, err := portForwardToECM()
	if err != nil {
		return err
	}
	defer func() {
		if err := killportForwardToECM(cmd); err != nil {
			fmt.Println("Error killing port-forward process:", err)
		}
	}()

	data, err := os.ReadFile(baselineClusterTemplatePath)
	if err != nil {
		return err
	}

	fmt.Println("POST request to cluster-manager")
	req, err := http.NewRequest("POST", clusterTemplateURL, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	req.Header.Set("Activeprojectid", namespace)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create default template in namespace '%s': %s", namespace, string(body))
	}

	err = cu.SetDefaultTemplate("baseline", "v0.0.1", namespace)
	if err != nil {
		return err
	}

	return nil
}

// creates single node cluster and with given name, nodeid and namespace <cluster-name> <node-id> <namespace>
func (CoUtils) CreateCluster(clusterName, nodeGUID, namespace string) error {
	fmt.Println("Create cluster with name:", clusterName, "and node id:", nodeGUID, "in namespace:", namespace)

	cmd, err := portForwardToECM()
	if err != nil {
		return err
	}
	defer func() {
		if err := killportForwardToECM(cmd); err != nil {
			fmt.Println("Error killing port-forward process:", err)
		}
	}()

	data, err := fillClusterConfig(clusterName, defaultTemplate, nodeGUID)
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	fmt.Println("POST request to cluster-manager")
	req, err := http.NewRequest("POST", clusterCreateURL, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	req.Header.Set("Activeprojectid", namespace)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create cluster %s in namespace '%s': %s", clusterName, namespace, string(body))
	}

	return nil
}

// deletes cluster with given name in given namespace <cluster-name> <namespace>
func (CoUtils) DeleteCluster(clusterName, namespace string) error {
	cmd, err := portForwardToECM()
	if err != nil {
		return err
	}
	defer func() {
		if err := killportForwardToECM(cmd); err != nil {
			fmt.Println("Error killing port-forward process:", err)
		}
	}()

	deleteUrl := fmt.Sprintf("%s/%s", clusterCreateURL, clusterName)
	req, err := http.NewRequest("DELETE", deleteUrl, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Activeprojectid", namespace)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to delete cluster %s in namespace '%s': %s", clusterName, namespace, string(body))
	}

	return nil
}

// sets default template in given namespace <template-name> <template-version> <namespace>
func (CoUtils) SetDefaultTemplate(templateName, templateVersion, namespace string) error {
	fmt.Printf("Set template %s in namespace %s as default\n", templateName, namespace)
	cmd, err := portForwardToECM()
	if err != nil {
		return err
	}
	defer func() {
		if err := killportForwardToECM(cmd); err != nil {
			fmt.Println("Error killing port-forward process:", err)
		}
	}()

	reqBody := map[string]string{
		"name":    templateName,
		"version": templateVersion,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	putUrl := fmt.Sprintf("%s/%s/default", clusterTemplateURL, templateName)
	req, err := http.NewRequest("PUT", putUrl, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	req.Header.Set("Activeprojectid", namespace)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to set template %s as default in namespace '%s': %s", templateName, namespace, string(body))
	}

	fmt.Printf("Template %s set as default in namespace %s\n", templateName, namespace)
	return nil
}

func portForwardToECM() (*exec.Cmd, error) {
	fmt.Println("port-forward to cluster-manager")
	cmd := exec.Command("kubectl", "port-forward", "-n", portForwardServiceNamespace, portForwardService, fmt.Sprintf("%s:%s", portForwardLocalPort, portForwardRemotePort), "--address", portForwardAddress)
	err := cmd.Start()
	time.Sleep(5 * time.Second) // Give some time for port-forwarding to establish

	return cmd, err
}

func killportForwardToECM(cmd *exec.Cmd) error {
	fmt.Println("kill process that port-forwards network to cluster-manager")
	if cmd != nil && cmd.Process != nil {
		return cmd.Process.Kill()
	}
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
