// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/bitfield/script"

	"github.com/open-edge-platform/edge-manageability-framework/internal/retry"
	"gopkg.in/yaml.v3"
)

const (
	waitForReadyMin            = 5
	waitForEdgeClusterReadyMin = 15
	waitForNextSec             = 5
	enicReplicas               = 1

	enicContainerName = "edge-node"
	enicPodName       = "enic-0"
	enicNs            = "enic"
	enicPodExec       = "kubectl -n %s exec %s -c %s -- "
)

var (
	fleetNamespace string
	nodeGuid       string
)

func (Deploy) deployEnicCluster(targetEnv string, labels string) error {
	if err := cleanUpEnic(); err != nil {
		return err
	}

	if err := (DevUtils{}).DeployEnic(enicReplicas, targetEnv); err != nil {
		return err
	}

	// Allow some time for Helm to load ENiC
	time.Sleep(5 * time.Second)
	if err := (DevUtils{}).WaitForEnic(); err != nil {
		return fmt.Errorf("\n%w", err)
	}

	if err := enicGuid(); err != nil {
		return fmt.Errorf("\n%w", err)
	}

	if err := createNs(); err != nil {
		return err
	}

	if err := createCluster(); err != nil {
		return fmt.Errorf("\n%w", err)
	}

	// Before deploying any apps make sure cluster is ready
	if err := waitForEdgeClusterReady(); err != nil {
		return fmt.Errorf("\n%w", err)
	}

	clusterLabels := parseLabels(labels)
	if err := addClusterLabels(clusterLabels); err != nil {
		return err
	}

	if err := genKubeconfigEntry(); err != nil {
		return err
	}

	if err := (Use{}).EdgeCluster(); err != nil {
		return err
	}

	// Wait for edge node fleet agent to be ready
	if err := waitForENFleetAgentReady(); err != nil {
		return fmt.Errorf("\n%w", err)
	}

	if err := (Use{}).Orch(); err != nil {
		return err
	}

	fmt.Println("Assigned cluster labels: ")
	for _, l := range clusterLabels {
		fmt.Printf("\t%s\n", l)
	}

	fmt.Println("ENiC cluster ready 😊")

	return nil
}

func enicGuid() error {
	cmd := fmt.Sprintf(enicPodExec+"dmidecode -s system-uuid | tr -d ' \n'", enicNs, enicPodName, enicContainerName)
	nodeGuidOut, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return fmt.Errorf("cannot get system-uuid inside enic: %v", nodeGuidOut)
	}

	nodeGuid = string(nodeGuidOut)

	fmt.Printf("Retrieved nodeGuid: %v...\n", nodeGuid)

	return nil
}

func deleteEnic() error {
	fmt.Printf("Deleting previous Helm ENiC...\n")

	// Uninstalling Helm chart will not delete enic due to statefulset.apps
	out, err := script.Exec("helm -n utils uninstall root-app").String()
	if err != nil {
		if !(strings.Contains(out, "Error: uninstall: Release not loaded: root-app: release: not found")) {
			return err
		}
	}

	// Enic doesn't fully delete when helm uninstalls, need to delete statefulset first to prevent pod
	// from coming up again
	cmd := fmt.Sprintf("kubectl -n %s delete statefulset.apps/%s --wait=true", enicNs, enicNs)
	out, err = script.Exec(cmd).String()
	if err != nil {
		if !(strings.Contains(out, "Error from server (NotFound)")) {
			return err
		}
	}

	fmt.Printf("Deleting previous ENiC pod...\n")
	cmd = fmt.Sprintf("kubectl -n %s delete pod/%s --wait=true", enicNs, enicPodName)
	out, err = script.Exec(cmd).String()
	if err != nil {
		if !(strings.Contains(out, "Error from server (NotFound)")) {
			return err
		}
	}

	fmt.Printf("Deleted previous ENiC...\n")
	return nil
}

func createCluster() error {
	if err := (CoUtils{}).CreateCluster(edgeClusterName, nodeGuid); err != nil {
		return err
	}

	fmt.Printf("Created cluster %s...\n", edgeClusterName)

	return nil
}

func deleteCluster() error {
	if err := (CoUtils{}).DeleteCluster(edgeClusterName); err != nil {
		// No cluster to delete and exit
		if strings.Contains(err.Error(), fmt.Sprintf("cluster %s not found", edgeClusterName)) {
			return nil
		}
		return err
	}

	// Wait for cluster resource to fully delete
	if err := waitForEdgeClusterDelete(); err != nil {
		return fmt.Errorf("\n%w", err)
	}

	return nil
}

func waitForENFleetAgentReady() error {
	ctx, cancel := context.WithTimeout(context.Background(), waitForReadyMin*time.Minute)
	defer cancel()
	counter := 0
	totalPods := 0

	// Since kubeconfig context is Edge Cluster
	edgeClusterPodStatusCmd := "kubectl get pods -A"

	cmd := "kubectl -n cattle-fleet-system get pods fleet-agent-0 -o jsonpath='{.status.phase}'"

	fmt.Printf("Waiting %v minutes for EN Fleet Agent to start...\n", waitForReadyMin)
	fn := func() error {
		out, err := exec.Command("bash", "-c", cmd).Output()

		fleetAgentStatus := string(out)
		if fleetAgentStatus == "" {
			fleetAgentStatus = "Pending"
		}

		totalPods = printEdgeClusterPodStatus(edgeClusterPodStatusCmd, totalPods)

		totalTime := minsAndSecs(counter * waitForNextSec)
		if err != nil || fleetAgentStatus != "Running" {
			fmt.Printf("\rEN Fleet Agent Status: %s (%s)", fleetAgentStatus, totalTime)
			counter++
			return fmt.Errorf("fleet agent is not ready")
		} else {
			fmt.Printf("\nEN Fleet Agent Status: %s (%s)\n", fleetAgentStatus, totalTime)
			return nil
		}
	}

	if err := retry.UntilItSucceeds(ctx, fn, time.Duration(waitForNextSec)*time.Second); err != nil {
		return fmt.Errorf("cluster setup error: %w 😲", err)
	}

	return nil
}

func clusterAgentStatus() string {
	agentStatus := fmt.Sprintf("{.items[?(@.metadata.labels.cluster\\.x-k8s\\.io/cluster-name==\"%s\")]"+
		".metadata.annotations.intelmachine\\.infrastructure\\.cluster\\.x-k8s\\.io/agent-status}", edgeClusterName)

	out, err := exec.Command("bash", "-c",
		fmt.Sprintf("kubectl -n %s get intelmachine -o jsonpath='%s'",
			fleetNamespace, agentStatus)).Output()
	if err != nil {
		return "unknown"
	}

	return string(out)
}

func printEdgeClusterPodStatus(bashCmd string, totalPods int) int {
	podStatus := ""

	out, err := exec.Command("bash", "-c", bashCmd).Output()
	if err == nil {
		podStatus = string(out)
	}

	if podStatus != "" {
		for range totalPods {
			fmt.Print("\033[F\033[K")
		}

		// +1 to remove Edge Cluster Pods line
		totalPods = strings.Count(podStatus, "\n") + 1
		fmt.Printf("\r\033[2K== Edge Cluster Pods ==\n%s", podStatus)
	}
	return totalPods
}

func waitForEdgeClusterReady() error {
	ctx, cancel := context.WithTimeout(context.Background(), waitForEdgeClusterReadyMin*time.Minute)
	defer cancel()
	counter := 0
	totalPods := 0

	getPodsCmd := "/var/lib/rancher/rke2/bin/kubectl --kubeconfig /etc/rancher/rke2/rke2.yaml get pods -A"
	edgeClusterPodStatusCmd := fmt.Sprintf(enicPodExec+"%s", enicNs, enicPodName, enicContainerName, getPodsCmd)

	type Status struct {
		ControlPlaneReady   string `yaml:"controlPlaneReady"`
		InfrastructureReady string `yaml:"infrastructureReady"`
	}

	var status Status

	fmt.Printf("Waiting %v minutes for cluster to be ready...\n", waitForEdgeClusterReadyMin)
	fn := func() error {
		out, err := exec.Command("bash", "-c",
			fmt.Sprintf("kubectl -n %s get cluster.cluster.x-k8s.io %s -o jsonpath='{.status}'",
				fleetNamespace, edgeClusterName)).Output()
		if err != nil {
			return fmt.Errorf("cluster is not ready")
		}

		err = yaml.Unmarshal(out, &status)
		if err != nil {
			return err
		}

		controlPlaneStatus := status.ControlPlaneReady
		if controlPlaneStatus == "" {
			controlPlaneStatus = "false"
		}

		infraStatus := status.InfrastructureReady
		if infraStatus == "" {
			infraStatus = "false"
		}

		// Get the status of cluster agent being installed on the edge cluster
		clusterAgentStatus := clusterAgentStatus()
		if clusterAgentStatus == "" {
			clusterAgentStatus = "inactive"
		}

		if clusterAgentStatus == "active" {
			totalPods = printEdgeClusterPodStatus(edgeClusterPodStatusCmd, totalPods)
		}

		totalTime := minsAndSecs(counter * waitForNextSec)

		if controlPlaneStatus == "true" && infraStatus == "true" {
			fmt.Printf("\n\033[2KcontrolPlaneReady: %s  | infrastructureReady: %s | Cluster Agent Status: %s (%s)\n",
				controlPlaneStatus, infraStatus, clusterAgentStatus, totalTime)
			return nil
		} else {
			fmt.Printf("\r\033[2KcontrolPlaneReady: %s | infrastructureReady: %s | Cluster Agent Status: %s (%s)",
				controlPlaneStatus, infraStatus, clusterAgentStatus, totalTime)
			counter++
			return fmt.Errorf("cluster is not ready")
		}
	}

	if err := retry.UntilItSucceeds(ctx, fn, time.Duration(waitForNextSec)*time.Second); err != nil {
		return fmt.Errorf("cluster setup error: %w 😲", err)
	}

	return nil
}

func minsAndSecs(totalSecs int) string {
	mins := totalSecs / 60
	secs := totalSecs % 60
	totalTime := fmt.Sprintf("%vm%vs", mins, secs)
	if mins < 1 {
		totalTime = fmt.Sprintf("%vs", secs)
	}

	return totalTime
}

func waitForEdgeClusterDelete() error {
	ctx, cancel := context.WithTimeout(context.Background(), waitForReadyMin*time.Minute)
	defer cancel()
	counter := 0

	fmt.Printf("Waiting %v minutes for cluster to delete...\n", waitForReadyMin)
	fn := func() error {
		totalTime := minsAndSecs(counter * waitForNextSec)

		out, err := script.Exec(fmt.Sprintf("kubectl -n %s get cluster.cluster.x-k8s.io %s",
			fleetNamespace, edgeClusterName)).String()
		if err != nil {
			if strings.Contains(out, "Error from server (NotFound)") {
				fmt.Printf("\nDeleted  cluster...(%s)\n", totalTime)
				return nil
			}
			fmt.Printf("\rDeleting cluster...(%s)", totalTime)
			counter++
			return fmt.Errorf("deleting cluster")
		}
		fmt.Printf("\rDeleting cluster...(%s)", totalTime)
		counter++
		return fmt.Errorf("deleting cluster")
	}

	if err := retry.UntilItSucceeds(ctx, fn, time.Duration(waitForNextSec)*time.Second); err != nil {
		return fmt.Errorf("cluster deletion error: %w 😲", err)
	}

	return nil
}

func createNs() error {
	cmd := fmt.Sprintf("kubectl create ns %s", fleetNamespace)
	out, err := script.Exec(cmd).String()
	if err != nil {
		if !(strings.Contains(out, "Error from server (AlreadyExists)")) {
			return err
		}
	}

	return nil
}

// Add provided labels to the cluster.
func addClusterLabels(labels []string) error {
	for _, l := range labels {
		addLabelCmd := fmt.Sprintf(
			"kubectl -n %s label cluster.cluster.x-k8s.io/%s --overwrite %s", fleetNamespace, edgeClusterName, l)
		_, err := script.Exec(addLabelCmd).String()
		if err != nil {
			return err
		}
	}

	return nil
}

// parseLabels accepts a comma separated list of labels (eg: color=blue, foo=bar)
// and returns a sanitized list (remove spaces)
// NOTE that for backward compatibility if no labels are provided, we are adding `color=blue`.
func parseLabels(l string) []string {
	_labels := strings.Split(l, ",")

	var labels []string
	for _, kv := range _labels {
		sanitized := strings.TrimSpace(kv)
		labels = append(labels, sanitized)
	}

	return labels
}

// Copied over from smoke test
func getUserToken() (string, error) {
	pemBytes, err := script.File("./orch-ca.crt").Bytes()
	if err != nil {
		return "", err
	}

	caPool := x509.NewCertPool()
	ok := caPool.AppendCertsFromPEM(pemBytes)
	if !ok {
		return "", errors.New("unable to create cert pool")
	}

	tlsConfig := &tls.Config{ //nolint: gosec
		RootCAs: caPool,
	}

	cli := &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: tlsConfig,
		},
	}

	defaultOrchPassword, err := GetDefaultOrchPassword()
	if err != nil {
		return "", err
	}
	edgeMgrToken, err := GetApiToken(cli, edgeMgrUser, defaultOrchPassword)

	return *edgeMgrToken, err
}

// Add edge cluster to kubeconfig. By default, kubeconfig doesn't get
// updated with the CAPI edge node. Per CAPI, use clusterctl to get kubeconfig.
// But to avoid clusterctl, we will populate kubeconfig with kubectl.
// https://cluster-api.sigs.k8s.io/clusterctl/developers#get-the-kubeconfig-for-the-workload-cluster-when-using-docker-desktop
//
// A better approach would be to download the kubeconfig from the API.  It should have all
// the correct information in it.
func genKubeconfigEntry() error {
	fmt.Printf("Adding %s to kubeconfig entry...\n", edgeClusterName)

	cmd := "cat ./orch-ca.crt | base64 -w0"
	certificateAuthorityData, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return err
	}

	cmd = fmt.Sprintf("kubectl config set clusters.%s.certificate-authority-data %s", edgeClusterName, certificateAuthorityData)
	_, err = exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return err
	}

	cmd = fmt.Sprintf("kubectl -n %s get secrets %s-kubeconfig -o yaml | yq e '.data.value' | base64 -d | yq e '.clusters[0].cluster.server'",
		fleetNamespace, edgeClusterName)
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return err
	}

	replaceEdgeServer := string(out)
	insideClusterUrl := "http://cluster-connect-gateway.orch-cluster.svc:8080"
	outsideClusterUrl := "https://connect-gateway.kind.internal:443"

	edgeServer := strings.Replace(replaceEdgeServer, insideClusterUrl, outsideClusterUrl, 1)

	cmd = fmt.Sprintf("kubectl config set clusters.%s.server %s", edgeClusterName, edgeServer)
	_, err = script.Exec(cmd).String()
	if err != nil {
		return err
	}

	type User struct {
		ClientCertificateData string `yaml:"client-certificate-data"`
		ClientKeyData         string `yaml:"client-key-data"`
	}

	var user User

	cmd = fmt.Sprintf("kubectl -n %s get secrets %s-kubeconfig -o yaml | yq e '.data.value' | base64 -d | yq e '.users[0].user'", fleetNamespace, edgeClusterName) //nolint: lll
	out, err = exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(out, &user)
	if err != nil {
		return err
	}

	cmd = fmt.Sprintf("kubectl config set users.%s-admin.client-certificate-data %s", edgeClusterName, user.ClientCertificateData)
	_, err = exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return err
	}

	cmd = fmt.Sprintf("kubectl config set users.%s-admin.client-key-data %s", edgeClusterName, user.ClientKeyData)
	_, err = exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return err
	}

	// Add token so that kubeconfig will work with JWT auth enabled on CCG
	token, err := getUserToken()
	if err != nil {
		return err
	}
	cmd = fmt.Sprintf("kubectl config set users.%s-admin.token %s", edgeClusterName, token)
	_, err = exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return err
	}

	cmd = fmt.Sprintf("kubectl config set-context %s-admin --user=%s-admin --cluster=%s", edgeClusterName, edgeClusterName, edgeClusterName)
	_, err = exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return err
	}

	return nil
}

func rmKubeconfigEdgeCluster() error {
	fmt.Printf("Removing any previous %s from kubeconfig entry...\n", edgeClusterName)

	// Will not delete if current context is EdgeCluster
	if err := (Use{}).Orch(); err != nil {
		return err
	}

	cmd := fmt.Sprintf("kubectl config delete-cluster %s", edgeClusterName)
	out, err := script.Exec(cmd).String()
	if err != nil {
		if !(strings.Contains(out, fmt.Sprintf("%s, not in", edgeClusterName))) {
			return err
		}
	}

	cmd = fmt.Sprintf("kubectl config delete-context %s-admin", edgeClusterName)
	out, err = script.Exec(cmd).String()
	if err != nil {
		if !(strings.Contains(out, fmt.Sprintf("%s-admin, not in", edgeClusterName))) {
			return err
		}
	}

	cmd = fmt.Sprintf("kubectl config delete-user %s-admin", edgeClusterName)
	out, err = script.Exec(cmd).String()
	if err != nil {
		if !(strings.Contains(out, fmt.Sprintf("%s-admin, not in", edgeClusterName))) {
			return err
		}
	}

	return nil
}

func projectId(projectName string) (string, error) {
	projectId, err := RuntimeProjUid(projectName)
	if err != nil || projectId == "" || projectId == "null" {
		if err != nil {
			return "", fmt.Errorf("cannot find ProjectID from %s project: %w", projectName, err)
		}

		return "", fmt.Errorf("cannot find ProjectID from %s project", projectName)
	}

	fmt.Printf("Retrieved project id %s from %s project\n", projectId, projectName)

	return projectId, nil
}

func cleanUpEnic() error {
	if err := deleteCluster(); err != nil {
		return fmt.Errorf("\n%w", err)
	}

	// Clean up any previous entry
	if err := rmKubeconfigEdgeCluster(); err != nil {
		return err
	}

	if err := deleteEnic(); err != nil {
		return err
	}

	return nil
}
