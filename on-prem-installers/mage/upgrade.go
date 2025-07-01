// SPDX-FileCopyrightText: 2025 Intel Corporation
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
	_ = sh.RunV("kubectl", "delete", "-n", "system-upgrade", "plans.upgrade.cattle.io", "--all")

	// Label Orchestrator node to mark it as ready for the upgrade
	if err := sh.RunV("kubectl", "label", nodeName, "rke2-upgrade=true", "--overwrite"); err != nil {
		return err
	}

	/* NOTE: MINOR version cannot be skipped when upgrading Kubernetes e.g. if you're planning to go from 1.26 to 1.28,
	   1.27 needs to be installed first.
	   TODO: Add logic to determine version hops dynamically instead of hardcoding them.
	   NOTE: EMF v3.0.0 uses "v1.30.10+rke2r1"
	*/
	for i, rke2UpgradeVersion := range []string{"v1.30.10+rke2r1"} {
		// Set version in upgrade Plan and render template.
		tmpl, err := template.ParseFiles(filepath.Join("rke2", "upgrade-plan.tmpl"))
		if err != nil {
			return err
		}

		upgradePlan, err := os.Create(filepath.Join("rke2", "upgrade-plan.yaml"))
		if err != nil {
			return err
		}

		if err := tmpl.Execute(upgradePlan, struct{ Version string }{Version: rke2UpgradeVersion}); err != nil {
			return err
		}
		upgradePlan.Close()

		// Apply the upgrade Plan CRD
		if err := sh.RunV("kubectl", "apply", "-f", filepath.Join("rke2", "upgrade-plan.yaml")); err != nil {
			return err
		}

		fmt.Printf("RKE2 upgrade Plan applied, waiting for upgrade to version %s to complete...\n", rke2UpgradeVersion)

		// Wait for node to upgrade to new rke2 version
		if err := waitForNewVersion(nodeName, strings.ReplaceAll(rke2UpgradeVersion, "-", "+")); err != nil {
			return err
		}

		// Then wait for Ready state which means upgrade has been completed
		if err := waitForNodeStatus(nodeName, "Ready"); err != nil {
			return err
		}

		if i == 0 {
			fmt.Printf("RKE2 upgraded to intermediate version %s, starting another upgrade...\n", rke2UpgradeVersion)
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
	},
		5*time.Second,
	); err != nil {
		return fmt.Errorf("orchestrator not in %s state and timeout elapsed ❌", status)
	}

	// Include time spent on waiting in deploymentTimeout
	timeRemaining := fmt.Sprintf("%ds", int(time.Until(expireTime).Seconds()))
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
	}, 5*time.Second,
	); err != nil {
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

func sanitizeString(str string) string {
	sanitized := strings.ReplaceAll(str, "\n", "")
	sanitized = strings.ReplaceAll(sanitized, `"`, "")

	return sanitized
}
