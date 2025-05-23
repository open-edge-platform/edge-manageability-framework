// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/bitfield/script"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"

	"github.com/open-edge-platform/edge-manageability-framework/internal/secrets"
)

// verboseLevel is a package-level variable initialized during startup.
var verboseLevel = func() int {
	if getDebug() {
		return 9
	}
	return 1
}()

// getDebug reads environment variable to set the debug level.
func getDebug() bool {
	value := os.Getenv("MAGEFILE_DEBUG")
	return value == "1"
}

// LookupOrchestratorIP returns the ip for the orchestrator
func LookupOrchestratorIP() (string, error) {
	return lookupOrchIP()
}

// LookupOrchestratorDomain returns the domain for the orchestrator
// By default it will look for the domain in the configmap orchestrator-domain
func LookupOrchestratorDomain() (string, error) {
	kubeCmd := "kubectl -n orch-gateway get configmap orchestrator-domain -o json"
	data, err := script.NewPipe().Exec(kubeCmd).String()
	if err != nil {
		return "", fmt.Errorf("error getting k8s dns Names from configmap: %w", err)
	}
	domain, err := script.Echo(data).JQ(".data.orchestratorDomainName").Replace(`"`, "").String()
	domain = strings.ReplaceAll(domain, "\n", "")

	if err != nil {
		return "", fmt.Errorf("error getting k8s dns Names from configmap: %w", err)
	}
	return domain, nil
}

func lookupOrchIP() (string, error) {
	if orchIP, exists := os.LookupEnv("ORCHESTRATOR_IP"); exists {
		return orchIP, nil
	}

	data, err := script.Exec("kubectl -n orch-gateway get svc traefik -o json").String()
	if err != nil {
		return data, fmt.Errorf("error running 'kubectl -n orch-gateway get svc traefik -o json' %w", err)
	}
	ip, err := script.Echo(data).JQ(".status.loadBalancer.ingress | .[0] | .ip ").Replace(`"`, "").String()
	if err != nil {
		return ip, fmt.Errorf("orch lb ip lookup: %w", err)
	}
	orchIP := strings.TrimSpace(ip)
	if orchIP == "" {
		return "", fmt.Errorf("orch IP is empty")
	}
	return orchIP, nil
}

// Get primary ip of this machine
func getPrimaryIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return net.IP{}, err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP, nil
}

func addCATrustStore(certName string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("%s OS not supported. Orchestrator CA must be installed manually. Submit a PR? 🤔", runtime.GOOS)
	}

	contents, err := os.ReadFile("/etc/issue")
	if err != nil {
		return fmt.Errorf("read system identification file: %w", err)
	}
	if !bytes.Contains(contents, []byte("Ubuntu")) {
		return fmt.Errorf("ubuntu is the only supported distro at this time. Submit a PR? 🤔")
	}

	if certName == "orch-ca.crt" {
		mg.Deps(Gen{}.OrchCA)
	}

	// Get current user. Used to see if sudo is required for permission to install CA.
	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("unable to get current user: %w", err)
	}

	// Determine if sudo is available. Typically not present in containers.
	_, err = exec.LookPath("sudo")
	hasSudo := err == nil

	// Command line to copy the Orchestrator CA to the local root store. Must be run as root.
	copyCertCmd := []string{
		"cp",
		certName,
		filepath.Join("/", "usr", "local", "share", "ca-certificates", certName),
	}

	// Command line to update CA certs. Must be run as root.
	updateCertCmd := []string{
		"update-ca-certificates",
	}

	if currentUser.Uid != "0" {
		// User does not have root, so cert installation must elevate with sudo.
		if hasSudo {
			// Prepend the command lines with sudo since it is available.
			copyCertCmd = append([]string{"sudo"}, copyCertCmd...)
			updateCertCmd = append([]string{"sudo"}, updateCertCmd...)
		} else {
			// No root and no sudo, but we must exit gracefully with a message and no error
			// to avoid breaking containerized automation on an optional step.
			fmt.Println("Unable to load Orchestrator CA into system trust store. Orchestrator CA must be installed manually. ⛔")
			return nil
		}
	}

	// Copy the CA to the root store, elevating with sudo if required.
	if err := sh.RunV(copyCertCmd[0], copyCertCmd[1:]...); err != nil {
		return fmt.Errorf("copy certificate to system CA directory: %w", err)
	}

	// Update CA certs, elevating with sudo if required.
	if err := sh.RunV(updateCertCmd[0], updateCertCmd[1:]...); err != nil {
		return fmt.Errorf("exec certificate reload: %w", err)
	}

	fmt.Println("Successfully loaded Orchestrator CA certificate into system trust store 🔒")

	return nil
}

func uniqueHosts() ([]string, string, error) {
	// hosts needs to be unsorted that's why we dont use (Gen{}).kubeDnslookupDockerInternal()
	kubeCmd := fmt.Sprintf("kubectl --v=%d -n orch-gateway get configmap kubernetes-docker-internal -o json", verboseLevel)
	hosts, err := Gen{}.dnsNamesConfigMap(kubeCmd)
	if err != nil {
		return nil, "", err
	}
	if len(hosts) == 0 {
		return nil, "", fmt.Errorf("no ingress entries found. Is orchestrator deployed")
	}

	// Put in to a map to prevent duplicates
	uniquehosts := map[string]struct{}{}

	domainName := hosts[0]
	// Print host if unique
	for _, host := range hosts {
		host = strings.TrimSpace(host)
		if _, ok := uniquehosts[host]; !ok {
			uniquehosts[host] = struct{}{}
		}
	}

	keys := make([]string, 0, len(uniquehosts))
	for k := range uniquehosts {
		keys = append(keys, k)
	}

	// Sort to make it easier for humans to search
	sort.Strings(keys)

	return keys, domainName, nil
}

// gatewayTLSSecretValid checks if the orch-gateway tls-orch secret is valid
// Returns true if the secret exists and is not expired, false otherwise.
func gatewayTLSSecretValid() bool {
	kubeCmd := "kubectl -n orch-gateway get secret tls-autocert -o json"
	secret, err := script.NewPipe().Exec(kubeCmd).String()
	if err != nil {
		return false
	}
	// retrieve the tls.crt field from the secret
	cert, err := script.Echo(secret).JQ(".data	.\"tls.crt\"").Replace(`"`, "").String()
	if err != nil {
		fmt.Println("parse JSON for certificate: %w", err)
		return false
	}
	caCertBytes, err := base64.StdEncoding.DecodeString(cert)
	if err != nil {
		fmt.Println("decode base64 certificate: %w", err)
		return false
	}

	var blocks []byte
	rest := caCertBytes
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			fmt.Printf("Error: PEM not parsed\n")
			break
		}
		blocks = append(blocks, block.Bytes...)
		if len(rest) == 0 {
			break
		}
	}
	certs, err := x509.ParseCertificates(blocks)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
	}

	// Check if the first certificate in the chain is the leaf certificate
	// (assuming the leaf certificate comes first in the chain)
	if len(certs) > 0 {
		if !certs[0].IsCA {
			// check that certs[0] is not expired
			if time.Now().After(certs[0].NotAfter) {
				fmt.Println("certificate expired") // Expired
				return false
			} else {
				return true // Not Expired
			}
		} else if certs[0].IsCA {
			if strings.Contains(certs[0].DNSNames[0], defaultClusterDomain) {
				// self-signed certificate deployment
				// return false, indicates a user is switching from self-signed to auto cert deployment
				return false
			}
			fmt.Println("certificate found but not a leaf certificate")
			return false
		}
	}
	fmt.Println("no certificate found in secret")
	return false
}

// saveGatewayTLSSecret saves the orch-gateway tls-autocert secret to persistence (aws secrets manager)
// Used to restore the secret later in case of an auto cert deployment
func saveGatewayTLSSecret(loc string) error {
	kubeCmd := "kubectl -n orch-gateway get secret tls-autocert -o yaml"
	secret, err := script.NewPipe().Exec(kubeCmd).String()
	if err != nil {
		return err
	}

	secret = base64.StdEncoding.EncodeToString([]byte(secret))

	if loc == "aws" {
		// Write yaml secret to AWS Secret Manager
		if err := secrets.NewAWSSM("tls-cert."+serviceDomain, "").SaveSecret(secret); err != nil {
			return err
		}
	}

	if loc == "file" {
		// Write  yaml secret string to disk
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		secretPath := filepath.Join(homeDir, serviceDomain+".yaml")
		file, err := os.OpenFile(secretPath, os.O_RDWR, 0o644)
		if err != nil {
			return err
		}
		defer file.Close()
		if err := secrets.NewFileSaver().SaveSecret(file, secret); err != nil {
			return err
		}
	}

	return nil
}

// restoreGatewayTLSSecret restores the orch-gateway tls-orch secret from persistence
// Used to restore the secret in case of an auto cert deployment
func restoreGatewayTLSSecret(loc string) error {
	var tlsSecret string
	if loc == "file" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		file, err := os.OpenFile(homeDir+"/"+serviceDomain+".yaml", os.O_RDONLY, 0o600)
		if err != nil {
			return err
		}
		// Read yaml secret string from disk
		tlsSecret, err = secrets.NewFileSaver().GetSecret(file, "")
		if err != nil {
			return err
		}
	}

	if loc == "aws" {
		var err error
		// Read yaml secret string from AWS Secret Manager
		tlsSecret, err = secrets.NewAWSSM("tls-cert."+serviceDomain, "").GetSecret("")
		if err != nil {
			return err
		}
	}

	if tlsSecret == "" {
		return fmt.Errorf("tls secret not found")
	}

	rtlsSecret, err := base64.StdEncoding.DecodeString(tlsSecret)
	if err != nil {
		return err
	}

	tlsSecret = string(rtlsSecret)

	// validate tlsSecret is a valid secret
	if !strings.Contains(tlsSecret, "tls.crt") {
		return fmt.Errorf("invalid tls secret retrieved from persistence")
	}

	// check if the secret already exists
	kubeCmd := "kubectl -n orch-gateway get secret tls-autocert"
	_, err = script.NewPipe().Exec(kubeCmd).String()
	if err != nil {
		fmt.Println("creating tls-orch secret using saved secret on file")
		// Apply secret to k8s
		output, err := script.Echo(tlsSecret).Exec("kubectl apply -n orch-gateway -f -").String()
		if strings.Contains(output, "secret/tls-orch created") {
			return nil
		}

		if err != nil {
			return err
		}
	}

	return nil
}
