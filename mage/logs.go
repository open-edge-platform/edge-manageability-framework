// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

const (
	// Specify the script file you want to run
	scriptFile = "iam_debug.sh"
)

// Collect logs for debug
func (LogUtils) CollectLogsForDebug() {
	fmt.Printf("Collecting logs for debug \n")

	// Get the current working directory
	dir, err := os.Getwd()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Execute the script
	script := fmt.Sprintf("%s/mage/%s", dir, scriptFile)
	cmd := exec.Command("bash", script)

	// Capture the standard output and error
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("failed to execute script: %s", err)
	}

	// Print output for confirmation
	fmt.Println("Script output:", string(output))
}

// Collect Argo CD diagnosis information
func (LogUtils) CollectArgoDiags() error {
	if err := (Argo{}).Login(); err != nil {
		return err
	}

	appList, err := fetchAppList()
	if err != nil {
		return err
	}
	fmt.Println("=== BEGIN Argo CD Application List ===")
	fmt.Print(appList)
	fmt.Println("=== END Argo CD Application List ===")

	lines := strings.Split(appList, "\n")
	var appsToProcess []string
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) >= 6 && (fields[4] != "Synced" || fields[5] != "Healthy") {
			appsToProcess = append(appsToProcess, fields[0])
		}
	}
	for _, app := range appsToProcess {
		appManifests, err := printAppManifests(app)
		if err != nil {
			return err
		}
		fmt.Printf("=== BEGIN %s Manifests ===\n", app)
		fmt.Print(appManifests)
		fmt.Printf("=== END %s Manifests ===\n", app)

		appDiff, err := printAppDiff(app)
		if err != nil {
			return err
		}
		fmt.Printf("=== BEGIN %s Diff ===\n", app)
		fmt.Print(appDiff)
		fmt.Printf("=== END %s Diff ===\n", app)
	}
	return nil
}

func fetchAppList() (string, error) {
	cmd := exec.Command("argocd", "app", "list", "--grpc-web")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func printAppManifests(app string) (string, error) {
	cmd := exec.Command("argocd", "app", "manifests", app, "--grpc-web")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func printAppDiff(app string) (string, error) {
	cmd := exec.Command("argocd", "app", "diff", app, "--grpc-web")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}
