// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/magefile/mage/sh"
)

func (Deploy) rke2Cluster() error { //nolint: cyclop
	dockerUser, dockerUserPresent := os.LookupEnv("DOCKER_USERNAME")
	dockerPass, dockerPassPresent := os.LookupEnv("DOCKER_PASSWORD")

	var args []string
	if dockerUserPresent && dockerPassPresent {
		fmt.Println("Using Docker credentials for customizing RKE2 installation")
		args = append(args, "-u", dockerUser, "-p", dockerPass)
	}

	if err := sh.RunV(filepath.Join("rke2", "rke2installerlocal.sh")); err != nil {
		return fmt.Errorf("error running rke2installerlocal.sh: %w", err)
	}

	if err := sh.RunV("/bin/bash", append([]string{filepath.Join("rke2", "customize-rke2.sh")}, args...)...); err != nil {
		return fmt.Errorf("error running customize-rke2.sh: %w", err)
	}

	// We need to wait for all deployments and pods to be Ready also before deploying OpenEBS
	if err := testDeploymentAndPods(); err != nil {
		return fmt.Errorf("error testing deployments and pods: %w", err)
	}

	// Remove CRDs installed by RKE2 that are not needed for OpenEBS
	if err := sh.RunV("kubectl", "delete", "crd", "volumesnapshotcontents.snapshot.storage.k8s.io"); err != nil {
		return fmt.Errorf("error deleting volumesnapshotcontents CRD: %w", err)
	}
	if err := sh.RunV("kubectl", "delete", "crd", "volumesnapshots.snapshot.storage.k8s.io"); err != nil {
		return fmt.Errorf("error deleting volumesnapshots CRD: %w", err)
	}
	if err := sh.RunV("kubectl", "delete", "crd", "volumesnapshotclasses.snapshot.storage.k8s.io"); err != nil {
		return fmt.Errorf("error deleting volumesnapshots CRD: %w", err)
	}

	// Add and update the OpenEBS Helm repository
	if err := sh.RunV("helm", "repo", "add", "openebs", "https://openebs.github.io/openebs"); err != nil {
		return fmt.Errorf("error adding OpenEBS Helm repo: %w", err)
	}
	if err := sh.RunV("helm", "repo", "update"); err != nil {
		return fmt.Errorf("error updating Helm repos: %w", err)
	}

	// Install OpenEBS Helm chart with customized values
	helmArgs := []string{
		"install", "openebs", "openebs/openebs",
		"--namespace", "openebs",
		"--create-namespace",
		"--version", "4.3.0",
		"--set", "engines.replicated.mayastor.enabled=false",
		"--set", "engines.local.zfs.enabled=false",
		"--set", "engines.local.lvm.enabled=false",
		"--set", "zfs-localpv.enabled=false",
		"--set", "lvm-localpv.enabled=false",
		"--set", "localpv-provisioner.hostpathClass.isDefaultClass=true",
		"--set", "localpv-provisioner.localpv.enableLeaderElection=false",
		"--set", "localpv-provisioner.localpv.replicas=1",
		"--set", "localpv-provisioner.analytics.enabled=false",
		"--set", "loki.enabled=false",
		"--set", "alloy.enabled=false",
	}
	if err := sh.RunV("helm", helmArgs...); err != nil {
		return fmt.Errorf("error installing OpenEBS Helm chart: %w", err)
	}

	// create etcd-cert secret
	if err := sh.RunV("kubectl", "create", "secret", "generic", "etcd-certs",
		"--from-file=/var/lib/rancher/rke2/server/tls/etcd/server-client.crt",
		"--from-file=/var/lib/rancher/rke2/server/tls/etcd/server-client.key",
		"--from-file=/var/lib/rancher/rke2/server/tls/etcd/server-ca.crt"); err != nil {
		return fmt.Errorf("error creating etcd-certs secret: %w", err)
	}

	// create cron job that periodically defrags etcd
	if err := sh.RunV("kubectl", "apply", "-f",
		filepath.Join("rke2", "defrag-etcd-job.yaml")); err != nil {
		return fmt.Errorf("error applying defrag-etcd-job.yaml: %w", err)
	}

	// Do a final verification (after installing OpenEBS) of all deployments and pods
	// before declaring cluster is ready
	// We need to wait for all deployments and pods to be Ready also before deploying OpenEBS
	if err := testDeploymentAndPods(); err != nil {
		return fmt.Errorf("error testing deployments and pods after OpenEBS installation: %w", err)
	}

	if err := sh.RunV(filepath.Join("rke2", "customize-rke2.sh")); err != nil {
		return fmt.Errorf("error running customize-rke2.sh after OpenEBS installation: %w", err)
	}

	fmt.Println("RKE2 cluster ready: 😊")
	return nil
}

func downloadFile(filepath string, url string) error {
	// Get the data
	// Disable below golangci-lint errors. They are not relevant in this context.
	// 1. G107: Potential HTTP request made with variable url
	// 2. net/http.Get must not be called
	resp, err := http.Get(url) //nolint: gosec, noctx
	if err != nil {
		return fmt.Errorf("error making HTTP GET request to %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response status code %d from %s", resp.StatusCode, url)
	}

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("error creating file %s: %w", filepath, err)
	}
	defer out.Close()

	if _, err = io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("error writing to file %s: %w", filepath, err)
	}
	return nil
}

func testDeploymentAndPods() error {
	if err := (Test{}.deployment()); err != nil {
		return fmt.Errorf("timeout waiting for deployment to be ready: %w", err)
	}

	if err := (Test{}.pods()); err != nil {
		return fmt.Errorf("error waiting for pods to be ready: %w", err)
	}
	return nil
}
