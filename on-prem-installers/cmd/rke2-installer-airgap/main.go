// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/open-edge-platform/edge-manageability-framework/on-prem-installers/mage"
)

const header = `
__  .__   __. .___________. _______  __          __      .______    __  ___  _______
|  | |  \ |  | |           ||   ____||  |        |  |     |   _  \  |  |/  / |   ____|
|  | |   \|  | |---|  |----||  |__   |  |        |  |     |  |_)  | |  '  /  |  |__
|  | |  . |  |     |  |     |   __|  |  |        |  |     |   ___/  |    <   |   __|
|  | |  |\   |     |  |     |  |____ |  |----.   |  |----.|  |      |  .  \  |  |____
|__| |__| \__|     |__|     |_______||_______|   |_______|| _|      |__|\__\ |_______|
`

const (
	deploymentTimeoutEnv            = "DEPLOYMENT_TIMEOUT"
	offlineDeploymentDefaultTimeout = "3600s" // must be a valid duration string
)

func main() {
	fmt.Print(header)

	if err := os.Setenv("KUBECONFIG", fmt.Sprintf("/home/%s/.kube/config", os.Getenv("USER"))); err != nil {
		fmt.Printf("Error setting KUBECONFIG environment variable: %s\n", err)
		os.Exit(1)
	}

	// Verify deployment timeout is appropriately set, else set a good default value for offline deployment --start
	timeoutStr := os.Getenv(deploymentTimeoutEnv)
	if timeoutStr == "" {
		if err := os.Setenv(deploymentTimeoutEnv, fmt.Sprintf("%v", offlineDeploymentDefaultTimeout)); err != nil {
			fmt.Printf("Error setting %v environment variable: %s\n", deploymentTimeoutEnv, err)
			os.Exit(1)
		}
	} else {
		_, err := time.ParseDuration(timeoutStr)
		if err != nil {
			fmt.Printf("deployment timeout must be a valid duration string: %v", err)
			os.Exit(1)
		}
	}
	// --end

	// Deploy AirGap RKE2 cluster
	if err := (mage.Deploy{}).Rke2ClusterAirGap(); err != nil {
		fmt.Printf("Error deploying local airgap cluster: %s\n", err)
		os.Exit(1)
	}
}
