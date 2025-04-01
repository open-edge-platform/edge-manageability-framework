// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"testing"

	"github.com/bitfield/script"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// This test suite requires a running k8s instance of orchestrator to execute tests against.
// It implicitly assumes domain name resolution is setup on the machine executing these tests.
// All tests in this suite should be black-box tests. In other words, the tests
// should never manipulate systems under tests in a way that an end user cannot. For example,
// a test that remote connects over SSH to change a parameter in order to make assertions
// against the system fails this condition. Alternatively, a public API should be exposed to
// manipulate this parameter in order to write tests that make assertions on actual system behavior.
func TestOrchestrator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Orchestrator Integration Suite")
}

// tlsConfig requires all peer server certificates to be issued by the Orchestrator CA.
var tlsConfig *tls.Config

// pemBytes is the orchestrator CA cert.
var pemBytes []byte

var _ = SynchronizedBeforeSuite(func() []byte {
	// This function executes for 1st parallel run only

	// This test uses different authentication methods to validate Vault actual configuration against its expected
	// configuration. The VAULT_TOKEN environment variable should not be set, otherwise it will interfere with the
	// test results.
	Expect(os.Getenv("VAULT_TOKEN")).To(BeEmpty(), "VAULT_TOKEN environment should not be set")

	var err error
	if serviceDomain == defaultServiceDomain {
		pemBytes, err = script.File("../../orch-ca.crt").Bytes()
		Expect(err).ToNot(HaveOccurred(), "load Orchestrator CA certificate. Did you deploy Orchestrator?: %s", err)
		Expect(pemBytes).ToNot(BeEmpty(), "./orch-ca.crt must not be empty")
	} else {
		conf := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}

		conn, err := tls.Dial("tcp", "web-ui."+serviceDomainWithPort, conf)
		Expect(err).ToNot(HaveOccurred())
		defer conn.Close()

		// Concatenate the PEM-encoded certificates presented by the peer in leaf to CA ascending order
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
	}

	// Need to write the Root CA to Docker Server so that we can access Harbor Registry from Docker Client
	dockerCaRegistry := fmt.Sprintf("/etc/docker/certs.d/registry-oci.%s", serviceDomain)

	errWrite := os.WriteFile("/tmp/ca.crt", pemBytes, 0o644)
	Expect(errWrite).ToNot(HaveOccurred(), "creating file /tmp/ca.crt")

	_, err = script.Exec(fmt.Sprintf("sudo mkdir -p %s", dockerCaRegistry)).Stdout()
	Expect(err).ToNot(HaveOccurred(), "error creating directory %s", dockerCaRegistry)

	// Warning: parallelization of this can cause race conditions in the `cp` command
	_, err = script.Exec(fmt.Sprintf("sudo cp /tmp/ca.crt %s", dockerCaRegistry)).Stdout()
	Expect(err).ToNot(HaveOccurred(), "moving /tmp/ca.crt to %s", dockerCaRegistry)

	return pemBytes
}, func(pemBytesInit []byte) {
	// This function runs for all other parallel runs after 1st run
	pemBytes = pemBytesInit
	caPool := x509.NewCertPool()
	Expect(caPool.AppendCertsFromPEM(pemBytesInit)).To(BeTrue())

	tlsConfig = &tls.Config{ //nolint: gosec
		RootCAs: caPool,
	}
})
