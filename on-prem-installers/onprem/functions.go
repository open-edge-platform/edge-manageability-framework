// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

//go:build mage

package main

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/magefile/mage/mg"
)

type OnPrem mg.Namespace

// Create a harbor admin credential secret
func (OnPrem) CreateHarborSecret(namespace, password string) error {
	exec.Command("kubectl", "-n", namespace, "delete", "secret", "harbor-admin-credential", "--ignore-not-found").Run()
	secret := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: harbor-admin-credential
  namespace: %s
stringData:
  credential: "admin:%s"
`, namespace, password)
	return applySecret(secret)
}

// Create a harbor admin password secret
func (OnPrem) CreateHarborPassword(namespace, password string) error {
	exec.Command("kubectl", "-n", namespace, "delete", "secret", "harbor-admin-password", "--ignore-not-found").Run()
	secret := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: harbor-admin-password
  namespace: %s
stringData:
  HARBOR_ADMIN_PASSWORD: "%s"
`, namespace, password)
	return applySecret(secret)
}

// Create a keycloak admin password secret
func (OnPrem) CreateKeycloakPassword(namespace, password string) error {
	exec.Command("kubectl", "-n", namespace, "delete", "secret", "platform-keycloak", "--ignore-not-found").Run()
	secret := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: platform-keycloak
  namespace: %s
stringData:
  admin-password: "%s"
`, namespace, password)
	return applySecret(secret)
}

// Create a postgres password secret
func (OnPrem) CreatePostgresPassword(namespace, password string) error {
	exec.Command("kubectl", "-n", namespace, "delete", "secret", "postgresql", "--ignore-not-found").Run()
	secret := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: postgresql
  namespace: %s
stringData:
  postgres-password: "%s"
`, namespace, password)
	return applySecret(secret)
}

// Generate a random password with requirements
func (OnPrem) GeneratePassword() (string, error) {
	lower := randomChars("abcdefghijklmnopqrstuvwxyz", 1)
	upper := randomChars("ABCDEFGHIJKLMNOPQRSTUVWXYZ", 1)
	digit := randomChars("0123456789", 1)
	special := randomChars("!@#$%^&*()_+{}|:<>?", 1)
	remaining := randomChars("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()_+{}|:<>?", 21)
	password := lower + upper + digit + special + remaining
	shuffled := shuffleString(password)
	fmt.Println(shuffled)
	return shuffled, nil
}

// Check if oras is installed
func (OnPrem) CheckOras() error {
	_, err := exec.LookPath("oras")
	if err != nil {
		return fmt.Errorf("Oras is not installed, install oras, exiting...")
	}
	return nil
}

// Install jq tool
func (OnPrem) InstallJq() error {
	_, err := exec.LookPath("jq")
	if err == nil {
		fmt.Println("jq tool found in the path")
		return nil
	}
	cmd := exec.Command("sudo", "NEEDRESTART_MODE=a", "apt-get", "install", "-y", "jq")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Install yq tool
func (OnPrem) InstallYq() error {
	_, err := exec.LookPath("yq")
	if err == nil {
		fmt.Println("yq tool found in the path")
		return nil
	}
	cmd := exec.Command("bash", "-c", "curl -jL https://github.com/mikefarah/yq/releases/download/v4.42.1/yq_linux_amd64 -o /tmp/yq && sudo mv /tmp/yq /usr/bin/yq && sudo chmod 755 /usr/bin/yq")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Download artifacts from OCI registry in Release Service
func (OnPrem) DownloadArtifacts(cwd, dirName, rsURL, rsPath string, artifacts ...string) error {
	os.MkdirAll(fmt.Sprintf("%s/%s", cwd, dirName), 0755)
	os.Chdir(fmt.Sprintf("%s/%s", cwd, dirName))
	for _, artifact := range artifacts {
		cmd := exec.Command("sudo", "oras", "pull", "-v", fmt.Sprintf("%s/%s/%s", rsURL, rsPath, artifact))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return os.Chdir(cwd)
}

// Get JWT token from Azure
func (OnPrem) GetJWTToken(refreshToken, rsURL string) (string, error) {
	cmd := exec.Command("curl", "-X", "POST", "-d", fmt.Sprintf("refresh_token=%s&grant_type=refresh_token", refreshToken), fmt.Sprintf("https://%s/oauth/token", rsURL))
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	jq := exec.Command("jq", "-r", ".id_token")
	jq.Stdin = bytes.NewReader(out)
	token, err := jq.Output()
	return strings.TrimSpace(string(token)), err
}

// Wait for pods in namespace to be in Ready state
func (OnPrem) WaitForPodsRunning(namespace string) error {
	cmd := exec.Command("kubectl", "wait", "pod", "--selector=!job-name", "--all", "--for=condition=Ready", "--namespace="+namespace, "--timeout=600s")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Wait for deployment to be in Ready state
func (OnPrem) WaitForDeploy(deployment, namespace string) error {
	cmd := exec.Command("kubectl", "rollout", "status", "deploy/"+deployment, "-n", namespace, "--timeout=30m")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Wait for namespace to be created
func (OnPrem) WaitForNamespaceCreation(namespace string) error {
	for {
		cmd := exec.Command("kubectl", "get", "ns", namespace, "-o", "json")
		out, err := cmd.Output()
		if err != nil {
			return err
		}
		jq := exec.Command("jq", ".status.phase", "-r")
		jq.Stdin = bytes.NewReader(out)
		phase, err := jq.Output()
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(phase)) == "Active" {
			break
		}
		time.Sleep(5 * time.Second)
	}
	return nil
}

// --- Helper functions ---

func applySecret(secret string) error {
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(secret)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func randomChars(charset string, length int) string {
	result := make([]byte, length)
	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[num.Int64()]
	}
	return string(result)
}

func shuffleString(input string) string {
	r := []rune(input)
	for i := len(r) - 1; i > 0; i-- {
		j, _ := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		r[i], r[j.Int64()] = r[j.Int64()], r[i]
	}
	return string(r)
}
