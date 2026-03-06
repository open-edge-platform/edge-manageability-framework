// SPDX-FileCopyrightText: 2026 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/bitfield/script"
	"github.com/magefile/mage/sh"

	"github.com/open-edge-platform/edge-manageability-framework/internal/retry"
)

func (Upgrade) rke2Cluster() error {
	var stdout, stderr bytes.Buffer

	// Get Orchestrator node name
	_, err := sh.Exec(nil, &stdout, &stderr, "kubectl", "get", "nodes", "-oname")
	if err != nil {
		return err
	}
	nodeName := sanitizeString(stdout.String())

	// Get current RKE2 version
	currentVersion, err := getCurrentRKE2Version(nodeName)
	if err != nil {
		return fmt.Errorf("failed to get current RKE2 version: %w", err)
	}
	fmt.Printf("Current RKE2 version: %s\n", currentVersion)

	// Target version
	targetVersion := "v1.35.1+rke2r1"

	// Check if already at target version
	if currentVersion == targetVersion {
		fmt.Println("RKE2 is already at the target version. No upgrade needed.")
		return nil
	}

	// Determine upgrade path from current version to target
	upgradePath := determineUpgradePath(currentVersion, targetVersion)
	if len(upgradePath) == 0 {
		return fmt.Errorf("unable to determine upgrade path from %s to %s", currentVersion, targetVersion)
	}

	fmt.Printf("Upgrade path: %v\n", upgradePath)

	// Install the system-upgrade-controller to perform automated upgrade
	if err := sh.RunV("kubectl", "apply", "-f",
		"https://github.com/rancher/system-upgrade-controller/releases/download/v0.13.2/system-upgrade-controller.yaml",
	); err != nil {
		return err
	}

	// Wait for deployment to be Ready
	if err := sh.RunV("kubectl", "rollout", "status", "deployment/system-upgrade-controller",
		"-n", "system-upgrade", "--timeout=10m",
	); err != nil {
		return err
	}

	// Wait for CRDs to create
	time.Sleep(15 * time.Second)

	// Delete all existing upgrade Plans
	// Ignore error as CRD might not yet have been created and it's fine for us
	if err := sh.RunV("kubectl", "delete", "-n", "system-upgrade", "plans.upgrade.cattle.io", "--all"); err != nil {
		fmt.Printf("failed to delete existing upgrade plans: %s\n", err)
		fmt.Printf("ignoring this error as it might be caused by the CRD not being created yet\n")
	}

	// Label Orchestrator node to mark it as ready for the upgrade
	if err := sh.RunV("kubectl", "label", nodeName, "rke2-upgrade=true", "--overwrite"); err != nil {
		return err
	}

	// Perform upgrades along the determined path
	for i, rke2UpgradeVersion := range upgradePath {
		// Set version in upgrade Plan and render template.
		tmpl, err := template.ParseFiles(filepath.Join("rke2", "upgrade-plan.tmpl"))
		if err != nil {
			return err
		}

		upgradePlan, err := os.Create(filepath.Join("rke2", "upgrade-plan.yaml"))
		if err != nil {
			return err
		}
		defer func() {
			if err := upgradePlan.Close(); err != nil {
				fmt.Printf("Warning: failed to close upgrade plan file: %v\n", err)
			}
		}()

		if err := tmpl.Execute(upgradePlan, struct{ Version string }{Version: rke2UpgradeVersion}); err != nil {
			return err
		}

		// Apply the upgrade Plan CRD
		if err := sh.RunV("kubectl", "apply", "-f", filepath.Join("rke2", "upgrade-plan.yaml")); err != nil {
			return err
		}

		fmt.Printf("RKE2 upgrade Plan applied, waiting for upgrade to version %s to complete...\n", rke2UpgradeVersion)

		// Wait for node to upgrade to new rke2 version
		// The kubeletVersion field uses "+" instead of "-" in its version string, so we replace "-" with "+" here.
		if err := waitForNewVersion(nodeName, strings.ReplaceAll(rke2UpgradeVersion, "-", "+")); err != nil {
			return err
		}

		// Then wait for Ready state which means upgrade has been completed
		if err := waitForNodeStatus(nodeName, "Ready"); err != nil {
			return err
		}

		if i < len(upgradePath)-1 {
			fmt.Printf("RKE2 upgraded to intermediate version %s, starting next upgrade...\n", rke2UpgradeVersion)
		}
	}

	// Clean up after upgrade
	if err := sh.RunV("kubectl", "label", nodeName, "rke2-upgrade=false", "--overwrite"); err != nil {
		return err
	}

	// Delete finalizers as they sometimes cause the delete operation to block indefinitely
	if err := sh.RunV(
		"kubectl", "patch", "clusterrolebinding", "system-upgrade", "-p", `{"metadata":{"finalizers":null}}`,
	); err != nil {
		return err
	}
	if err := sh.RunV("kubectl", "delete", "-f",
		"https://github.com/rancher/system-upgrade-controller/releases/download/v0.13.2/system-upgrade-controller.yaml",
	); err != nil {
		return err
	}

	fmt.Println("RKE2 cluster upgraded: 😊")

	return nil
}

// nodeName should be passed in format 'node/<node-name>'
// status argument must be either 'Ready' or 'NotReady'.
// If arguments are not in correct format, function will not behave correctly.
func waitForNodeStatus(nodeName, status string) error {
	timeout, err := parseDeploymentTimeout()
	if err != nil {
		return err
	}
	expireTime := time.Now().Add(timeout)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := retry.UntilItSucceeds(ctx, func() error {
		fmt.Println("~~~~~~~~~~")
		ready, err := script.NewPipe().
			Exec(fmt.Sprintf("kubectl get %s -o json", nodeName)).
			JQ(`.status.conditions[] | select(.type=="Ready") | .status`).
			String()
		if err != nil {
			fmt.Println("kubectl command not successful but it's temporary")
			fmt.Println("Time remaining ⏰: ", time.Until(expireTime))
			return err
		}
		ready = sanitizeString(ready)

		if status == "NotReady" {
			if ready != "True" {
				return nil
			}

			fmt.Printf("Orchestrator is not in %s state yet...\n", status)
			fmt.Println("Time remaining ⏰: ", time.Until(expireTime))
			return fmt.Errorf("orchestrator is not in %s state", status)
		}

		schedulable, err := script.NewPipe().
			Exec(fmt.Sprintf("kubectl get %s -o json", nodeName)).
			JQ(`if (.spec.taints // [] | map(select(.effect == "NoSchedule")) | length == 0) then "True" else "False" end`).
			String()
		if err != nil {
			fmt.Println("kubectl command not successful but it's temporary")
			fmt.Println("Time remaining ⏰: ", time.Until(expireTime))
			return err
		}
		schedulable = sanitizeString(schedulable)

		if ready == "False" || schedulable == "False" {
			fmt.Printf("Orchestrator is not in %s state yet...\n", status)
			fmt.Println("Time remaining ⏰: ", time.Until(expireTime))
			return fmt.Errorf("orchestrator is not in %s state", status)
		}

		return nil
	}, 5*time.Second); err != nil {
		return fmt.Errorf("orchestrator not in %s state and timeout elapsed ❌", status)
	}

	secondsRemaining := int(time.Until(expireTime).Seconds())
	if secondsRemaining < 0 {
		secondsRemaining = 0
	}

	timeRemaining := fmt.Sprintf("%ds", secondsRemaining)
	if err := setDeploymentTimeout(timeRemaining); err != nil {
		return err
	}

	fmt.Printf("Node has status %s, proceed ✅\n", status)

	return nil
}

// nodeName should be passed in format 'node/<node-name>'
func waitForNewVersion(nodeName, version string) error {
	timeout, err := parseDeploymentTimeout()
	if err != nil {
		return err
	}
	expireTime := time.Now().Add(timeout)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := retry.UntilItSucceeds(ctx, func() error {
		fmt.Println("~~~~~~~~~~")
		foundVersion, err := script.NewPipe().
			Exec(fmt.Sprintf("kubectl get %s -o json", nodeName)).
			JQ(`.status.nodeInfo.kubeletVersion`).
			String()
		if err != nil {
			fmt.Println("kubectl command not successful but it's temporary")
			fmt.Println("Time remaining ⏰: ", time.Until(expireTime))
			return err
		}
		foundVersion = sanitizeString(foundVersion)

		if foundVersion != version {
			fmt.Printf("RKE2 version is not %s yet...\n", version)
			fmt.Println("Time remaining ⏰: ", time.Until(expireTime))
			return fmt.Errorf("RKE2 version is not %s", version)
		}

		return nil
	}, 5*time.Second); err != nil {
		return fmt.Errorf("RKE2 version is not %s and timeout elapsed ❌", version)
	}

	// Include time spent on waiting in deploymentTimeout
	timeRemaining := fmt.Sprintf("%ds", int(time.Until(expireTime).Seconds()))
	if err := setDeploymentTimeout(timeRemaining); err != nil {
		return err
	}

	fmt.Printf("RKE2 has version %s, proceed ✅\n", version)
	return nil
}

// remove newline and double quote characters from the input string.
func sanitizeString(str string) string {
	return strings.Trim(str, "\"\n\r\t ")
}

// getCurrentRKE2Version retrieves the current RKE2 version from the node.
// nodeName should be in format 'node/<node-name>'
func getCurrentRKE2Version(nodeName string) (string, error) {
	version, err := script.NewPipe().
		Exec(fmt.Sprintf("kubectl get %s -o json", nodeName)).
		JQ(`.status.nodeInfo.kubeletVersion`).
		String()
	if err != nil {
		return "", err
	}
	return sanitizeString(version), nil
}

// determineUpgradePath determines the upgrade path from current to target version.
// It skips versions already installed and only includes necessary intermediate versions.
func determineUpgradePath(currentVersion, targetVersion string) []string {
	// All available versions in order
	allVersions := []string{
		"v1.30.14+rke2r2", // Patch update within 1.30
		"v1.31.13+rke2r1", // Upgrade to 1.31
		"v1.32.9+rke2r1",  // Upgrade to 1.32
		"v1.33.5+rke2r1",  // Upgrade to 1.33
		"v1.34.1+rke2r1",  // Upgrade to 1.34.1
		"v1.34.3+rke2r1",  // Upgrade to 1.34.3
		"v1.35.1+rke2r1",  // Final target version
	}

	// Extract minor version from full version string (e.g., "v1.30.14+rke2r2" -> "1.30")
	extractMinorVersion := func(version string) string {
		parts := strings.Split(version, ".")
		if len(parts) >= 2 {
			return parts[0] + "." + parts[1]
		}
		return version
	}

	currentMinor := extractMinorVersion(currentVersion)
	targetMinor := extractMinorVersion(targetVersion)

	// Find starting index
	startIdx := -1
	for i, v := range allVersions {
		if v == currentVersion {
			startIdx = i
			break
		}
		if strings.Contains(v, currentMinor) && startIdx == -1 {
			startIdx = i
		}
	}

	// If current version not found in list, start from beginning
	if startIdx == -1 {
		startIdx = 0
	} else {
		startIdx++ // Start from next version after current
	}

	// Find ending index
	endIdx := -1
	for i, v := range allVersions {
		if v == targetVersion {
			endIdx = i
			break
		}
		if strings.Contains(v, targetMinor) {
			endIdx = i
		}
	}

	// If target version not found, include everything to the end
	if endIdx == -1 {
		endIdx = len(allVersions) - 1
	}

	// Build upgrade path
	var upgradePath []string
	if startIdx <= endIdx {
		upgradePath = allVersions[startIdx : endIdx+1]
	}

	return upgradePath
}
