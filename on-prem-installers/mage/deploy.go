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

	// Add OpenEBS LocalPV helm repository
	if err := sh.RunV("helm", "repo", "add", "openebs-localpv", "https://openebs.github.io/dynamic-localpv-provisioner"); err != nil {
		return fmt.Errorf("error adding openebs-localpv helm repo: %w", err)
	}

	// Update helm repositories
	if err := sh.RunV("helm", "repo", "update"); err != nil {
		return fmt.Errorf("error updating helm repos: %w", err)
	}

	// Install/upgrade OpenEBS LocalPV provisioner
	if err := sh.RunV("helm", "upgrade", "--install", "openebs-localpv", "openebs-localpv/localpv-provisioner",
		"--version", "4.3.0",
		"--namespace", "openebs-system",
		"--create-namespace",
		"--set", "hostpathClass.enabled=true",
		"--set", "hostpathClass.name=openebs-hostpath",
		"--set", "hostpathClass.isDefaultClass=true",
		"--set", "deviceClass.enabled=false"); err != nil {
		return fmt.Errorf("error installing openebs-localpv with helm: %w", err)
	}

	// create etcd-cert secret
	if err := sh.RunV("kubectl", "create", "secret", "generic", "etcd-certs",
		"--from-file=/var/lib/rancher/rke2/server/tls/etcd/server-client.crt",
		"--from-file=/var/lib/rancher/rke2/server/tls/etcd/server-client.key",
		"--from-file=/var/lib/rancher/rke2/server/tls/etcd/server-ca.crt"); err != nil {
		return fmt.Errorf("error creating etcd-certs secret: %w", err)
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
