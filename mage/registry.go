// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"crypto/tls"
	"encoding/pem"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/magefile/mage/sh"
)

func (Registry) getRegistryCert(registryUrl string, filePath string, insecure bool) error {
	conf := &tls.Config{
		InsecureSkipVerify: insecure, //nolint: gosec
	}

	parsedUrl, err := url.Parse(registryUrl)
	if err != nil {
		return fmt.Errorf("invalid registry URL: %w", err)
	}
	hostAddr := fmt.Sprintf("%s:443", parsedUrl.Host)
	conn, err := tls.Dial("tcp", hostAddr, conf)
	if err != nil {
		return fmt.Errorf("dial registry cache: %w", err)
	}
	defer conn.Close()

	// Concatenate the PEM-encoded certificates presented by the peer in leaf to CA ascending order
	var pemBytes []byte
	for _, cert := range conn.ConnectionState().PeerCertificates {
		pemBytes = append(
			pemBytes,
			pem.EncodeToMemory(
				&pem.Block{
					Type:  "CERTIFICATE",
					Bytes: cert.Raw,
				},
			)...,
		)
	}

	// Create the directory if it does not exist - needed for scenario cert creation is done by onprem-ke-installer
	dirPath := filepath.Dir(filePath)
	if !filepath.IsAbs(dirPath) {
		dirPath = filepath.Join(".", dirPath)
	}

	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		fmt.Printf("Directory `%s` does not exist, creating...\n", dirPath)
		if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
			return fmt.Errorf("directory `%s` did not exist but could not be created: %w", dirPath, err)
		}
	}

	if err := os.WriteFile(
		filePath,
		pemBytes,
		os.ModePerm,
	); err != nil {
		return fmt.Errorf("write registry cache certificate file: %w", err)
	}
	return nil
}

// loadRegistryCacheCerts loads the Intel Harbor registry cache's x509 CA certificate into the orchestrator nodes system
// trust store to allow the container runtime to pull images from Docker Hub through the registry cache over TLS.
// func (Registry) loadRegistryCacheCerts() error {
// 	certFile := filepath.Join("mage", "intel-harbor-ca.crt")

// 	// If the certificate doesn't exist, generate it
// 	if _, err := os.Stat(certFile); errors.Is(err, fs.ErrNotExist) {
// 		mg.Deps(Gen{}.RegistryCacheCert)
// 	}

// 	cpDst := "/usr/local/share/ca-certificates/intel-harbor-ca.crt"
// 	if err := sh.RunV(
// 		"sudo", "cp",
// 		certFile,
// 		cpDst); err != nil {
// 		return fmt.Errorf("error copying certificates into orchestrator node: %w", err)
// 	}

// 	if err := sh.RunV(
// 		"sudo", "update-ca-certificates"); err != nil {
// 		return fmt.Errorf("error executing update CA certificates command within orchestrator node: %w", err)
// 	}

// 	if err := sh.RunV(
// 		"sudo",
// 		"systemctl",
// 		"restart",
// 		"containerd"); err != nil {
// 		return fmt.Errorf("error executing containerd restart to apply CA certificates within orchestrator node: %w", err) //nolint:lll
// 	}

// 	return nil
// }

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
