// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/magefile/mage/sh"
)

func (Registry) registryCert(insecure bool) error {
	// 	conf := &tls.Config{
	// 		InsecureSkipVerify: insecure, //nolint: gosec
	// 	}

	//  registryHostAddr := "registry-cache.domain.com:443"
	// 	conn, err := tls.Dial("tcp", registryHostAddr, conf)
	// 	if err != nil {
	// 		return fmt.Errorf("dial registry cache: %w", err)
	// 	}
	// 	defer conn.Close()

	// 	// Concatenate the PEM-encoded certificates presented by the peer in leaf to CA ascending order
	// 	var pemBytes []byte
	// 	for _, cert := range conn.ConnectionState().PeerCertificates {
	// 		pemBytes = append(
	// 			pemBytes,
	// 			pem.EncodeToMemory(
	// 				&pem.Block{
	// 					Type:  "CERTIFICATE",
	// 					Bytes: cert.Raw,
	// 				},
	// 			)...,
	// 		)
	// 	}

	// 	// Create the directory if it does not exist - needed for scenario cert creation is done by onprem-ke-installer
	// 	if _, err := os.Stat("mage"); os.IsNotExist(err) {
	// 		fmt.Println("Directory `mage` does not exist, creating...")
	// 		if err := os.Mkdir("mage", os.ModePerm); err != nil {
	// 			return fmt.Errorf("mage directory did not exist but could not be created: %w", err)
	// 		}
	// 	}

	// 	if err := os.WriteFile(
	// 		filepath.Join("mage", "intel-harbor-ca.crt"),
	// 		pemBytes,
	// 		os.ModePerm,
	// 	); err != nil {
	// 		return fmt.Errorf("write registry cache certificate file: %w", err)
	// 	}
	return nil
}

// loadRegistryCacheCerts loads the Intel Harbor registry cache's x509 CA certificate into the Orchestrator nodes system
// trust store to allow the container runtime to pull images from Docker Hub through the registry cache over TLS.
func (Registry) loadRegistryCacheCerts() error {
	certFile := filepath.Join("mage", "intel-harbor-ca.crt")

	// If the certificate doesn't exist, generate it
	if _, err := os.Stat(certFile); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("registry cache certificate file does not exist, please provision it.")
		// mg.Deps(Gen{}.RegistryCacheCert)
	}

	cpDst := "/usr/local/share/ca-certificates/intel-harbor-ca.crt"
	if err := sh.RunV(
		"sudo", "cp",
		certFile,
		cpDst); err != nil {
		return fmt.Errorf("error copying certificates into orchestrator node: %w", err)
	}

	if err := sh.RunV(
		"sudo", "update-ca-certificates"); err != nil {
		return fmt.Errorf("error executing update CA certificates command within orchestrator node: %w", err)
	}

	if err := sh.RunV(
		"sudo",
		"systemctl",
		"restart",
		"containerd"); err != nil {
		return fmt.Errorf("error executing containerd restart to apply CA certificates within orchestrator node: %w", err) //nolint:lll
	}

	return nil
}
