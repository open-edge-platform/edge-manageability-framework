// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/open-edge-platform/edge-manageability-framework/on-prem-installers/mage"
)

const header = `
 _  ________   _____ _   _  _____ _______       _      _      ______ _____  
| |/ /  ____| |_   _| \ | |/ ____|__   __|/\   | |    | |    |  ____|  __ \ 
| ' /| |__      | | |  \| | (___    | |  /  \  | |    | |    | |__  | |__) |
|  < |  __|     | | | . | |\___ \   | | / /\ \ | |    | |    |  __| |  _  / 
| . \| |____   _| |_| |\  |____) |  | |/ ____ \| |____| |____| |____| | \ \ 
|_|\_\______| |_____|_| \_|_____/   |_/_/    \_\______|______|______|_|  \_\									   
`

const (
	deploymentTimeoutEnv     = "DEPLOYMENT_TIMEOUT"
	deploymentDefaultTimeout = "3600s" // must be a valid duration string
)

var upgrade = flag.Bool("upgrade", false, "determine if KE should be upgraded or installed")

func main() {
	if err := os.Setenv("KUBECONFIG", fmt.Sprintf("/home/%s/.kube/config", os.Getenv("USER"))); err != nil {
		fmt.Printf("Error setting KUBECONFIG environment variable: %s\n", err)
		os.Exit(1)
	}

	if err := os.Setenv("INSTALLER_DEPLOY", "true"); err != nil {
		fmt.Printf("Error setting INSTALLER_DEPLOY environment variable: %s\n", err)
		os.Exit(1)
	}

	// Verify deployment timeout is appropriately set, else set a good default value for offline deployment --start
	timeoutStr := os.Getenv(deploymentTimeoutEnv)
	if timeoutStr == "" {
		if err := os.Setenv(deploymentTimeoutEnv, fmt.Sprintf("%v", deploymentDefaultTimeout)); err != nil {
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

	flag.Parse()
	if *upgrade {
		if err := (mage.Upgrade{}).Rke2Cluster(); err != nil {
			fmt.Printf("Error upgrading cluster: %s\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Deploy Online OnPrem RKE2 cluster
	fmt.Print(header)
	if err := (mage.Deploy{}).Rke2Cluster(); err != nil {
		fmt.Printf("Error deploying local cluster: %s\n", err)
		os.Exit(1)
	}
}
