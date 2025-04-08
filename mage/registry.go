// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"fmt"
	"os"

	"github.com/magefile/mage/sh"
)

// Start a local docker container registry.
func (Registry) StartLocalRegistry() error {
	// Try to start the registry to check if it already exitsts.
	err := sh.Run("docker", "start", fmt.Sprintf("kind-registry.%s", serviceDomain))
	if err == nil {
		// Already exists, do nothing
		return nil
	}
	if err := sh.RunV("docker", "run", "-d", "--name", fmt.Sprintf("kind-registry.%s", serviceDomain), "registry:2-intel"); err != nil {
		return err
	}
	if err := sh.RunV("docker", "network", "inspect", "kind"); err != nil {
		if err := sh.RunV("docker", "network", "create", "kind"); err != nil {
			return err
		}
	}
	return sh.RunV("docker", "network", "connect", "kind", fmt.Sprintf("kind-registry.%s", serviceDomain))
}

func (Registry) GetRegistryURL() string {
	// Return the registry URL, if it is in the environment use it, otherwise return local one
	registryURL := os.Getenv("DOCKER_REGISTRY_URL")
	if registryURL == "" {
		registryURL = fmt.Sprintf("http://kind-registry.%s:5000", serviceDomain)
	}

	return registryURL
}
