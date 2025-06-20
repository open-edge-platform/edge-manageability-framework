// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package onprem

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
)

type GiteaStep struct {
	RootPath               string
	KeepGeneratedFiles     bool
	orchConfigReaderWriter config.OrchConfigReaderWriter
	StepLabels             []string
}

func CreateGiteaStep(rootPath string, keepGeneratedFiles bool, orchConfigReaderWriter config.OrchConfigReaderWriter) *GiteaStep {
	return &GiteaStep{
		RootPath:               rootPath,
		KeepGeneratedFiles:     keepGeneratedFiles,
		orchConfigReaderWriter: orchConfigReaderWriter,
		// TODO should this have labels?
	}
}

func (s *GiteaStep) Name() string {
	return "GiteaStep"
}

func (s *GiteaStep) Labels() []string {
	return s.StepLabels
}

func (s *GiteaStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

func (s *GiteaStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	// no-op for now
	return runtimeState, nil
}

func (s *GiteaStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if err := installGitea(); err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("Error installing Gitea %v \n", err),
		}
	}

	return runtimeState, nil
}

func (s *GiteaStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, prevStepError
}

func installGitea() error {
	const (
		passwordLength = 16
		giteaAsciiArt  = `
   ____ _ _
  / ___(_) |_ ___  __ _
 | |  _| | __/ _ \/ _  |
 | |_| | | ||  __/ (_| |
  \____|_|\__\___|\__,_|
  `
	)

	fmt.Println(giteaAsciiArt) // TODO log

	// Create namespaces
	createNsCmd := exec.Command("kubectl", "create", "ns", "gitea")
	if err := createNsCmd.Run(); err != nil && !strings.Contains(err.Error(), "AlreadyExists") {
		return fmt.Errorf("failed to create namespace gitea: %w", err)
	}
	createNsCmd = exec.Command("kubectl", "create", "ns", "orch-platform")
	if err := createNsCmd.Run(); err != nil && !strings.Contains(err.Error(), "AlreadyExists") {
		return fmt.Errorf("failed to create namespace orch-platform: %w", err)
	}

	// Check for secret or generate new
	getSecretCmd := exec.Command("kubectl", "-n", "gitea", "get", "secret", "gitea-tls-certs")
	if err := getSecretCmd.Run(); err != nil {
		fmt.Println("Secret gitea-tls-certs not found. Generating new certificates.") // TODO log
		if err := processCerts("gitea-http.gitea.svc.cluster.local"); err != nil {
			return err
		}
	}

	// create passwords
	adminGiteaPassword, err := randomPassword(passwordLength)
	if err != nil {
		return fmt.Errorf("failed to generate admin Gitea password: %v", err)
	}
	argocdGiteaPassword, err := randomPassword(passwordLength)
	if err != nil {
		return fmt.Errorf("failed to generate argocd Gitea password: %v", err)
	}
	appGiteaPassword, err := randomPassword(passwordLength)
	if err != nil {
		return fmt.Errorf("failed to generate app Gitea password: %v", err)
	}
	clusterGiteaPassword, err := randomPassword(passwordLength)
	if err != nil {
		return fmt.Errorf("failed to generate cluster Gitea password: %v", err)
	}

	// create secrets
	if err := createGiteaSecret("gitea-cred", "gitea_admin", adminGiteaPassword, "gitea"); err != nil {
		return err
	}
	if err := createGiteaSecret("argocd-gitea-credential", "argocd", argocdGiteaPassword, "gitea"); err != nil {
		return err
	}
	if err := createGiteaSecret("app-gitea-credential", "apporch", appGiteaPassword, "orch-platform"); err != nil {
		return err
	}
	if err := createGiteaSecret("cluster-gitea-credential", "clusterorch", clusterGiteaPassword, "orch-platform"); err != nil {
		return err
	}

	// install Helm chart
	if err := installHelmChart(); err != nil {
		return err
	}

	// create accounts
	if err := createGiteaAccount("argocd-gitea-credential", "argocd", argocdGiteaPassword, "argocd@orch-installer.com"); err != nil {
		return err
	}
	if err := createGiteaAccount("app-gitea-credential", "apporch", appGiteaPassword, "apporch@orch-installer.com"); err != nil {
		return err
	}
	if err := createGiteaAccount("cluster-gitea-credential", "clusterorch", clusterGiteaPassword, "clusterorch@orch-installer.com"); err != nil {
		return err
	}

	return nil
}

func installHelmChart() error {
	imageRegistry := os.Getenv("IMAGE_REGISTRY")
	if imageRegistry == "" {
		return fmt.Errorf("IMAGE_REGISTRY environment variable is not set")
	}

	helmCmd := exec.Command(
		"helm", "install", "gitea", "/tmp/gitea/gitea",
		"--values", "/tmp/gitea/values.yaml",
		"--set", "gitea.admin.existingSecret=gitea-cred",
		"--set", fmt.Sprintf("image.registry=%s", imageRegistry),
		"-n", "gitea", "--timeout", "15m0s", "--wait",
	)
	if output, err := helmCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to install Helm chart: %v\n%s", err, string(output))
	}
	return nil
}

func processCerts(args ...string) error {
	fmt.Println("Generating key...")

	// Check if openssl is available
	if err := exec.Command("openssl", "version").Run(); err != nil {
		return fmt.Errorf("OpenSSL not found")
	}

	tmpDir, err := os.MkdirTemp("", "certs")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	keyPath := filepath.Join(tmpDir, "infra-tls.key")
	crtPath := filepath.Join(tmpDir, "infra-tls.crt")

	// Generate private key
	if err := exec.Command("openssl", "genrsa", "-out", keyPath, "4096").Run(); err != nil {
		return fmt.Errorf("failed to generate key: %v", err)
	}

	fmt.Println("Generating certificate...")

	san, err := processSAN(args...)
	if err != nil {
		return fmt.Errorf("failed to process SAN: %v", err)
	}

	// Generate certificate
	reqCmd := exec.Command(
		"openssl", "req",
		"-key", keyPath,
		"-new", "-x509",
		"-days", "365",
		"-out", crtPath,
		"-subj", "/C=US/O=Orch Deploy/OU=Orchestrator",
		"-addext", san,
	)
	if output, err := reqCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to generate certificate: %v\n%s", err, string(output))
	}

	// Copy certificate to system CA directory
	destCrt := "/usr/local/share/ca-certificates/gitea_cert.crt"
	if err := copyFile(crtPath, destCrt); err != nil {
		return fmt.Errorf("failed to copy certificate: %v", err)
	}

	// Update CA certificates
	if err := exec.Command("update-ca-certificates").Run(); err != nil {
		return fmt.Errorf("failed to update CA certificates: %v", err)
	}

	// Create Kubernetes secret
	kubectlCmd := exec.Command(
		"kubectl", "create", "secret", "tls", "gitea-tls-certs", "-n", "gitea",
		"--cert="+crtPath,
		"--key="+keyPath,
	)
	if output, err := kubectlCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create k8s secret: %v\n%s", err, string(output))
	}

	return nil
}

// Helper to copy files
func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0644)
}

func processSAN(domains ...string) (string, error) {
	result := "subjectAltName=DNS:localhost"
	for _, domain := range domains {
		result += ",DNS:" + domain
	}
	return result, nil
}

func createGiteaSecret(secretName, accountName, password, namespace string) error {
	createCmd := exec.Command(
		"kubectl", "create", "secret", "generic", secretName,
		"-n", namespace,
		"--from-literal=username="+accountName,
		"--from-literal=password="+password,
		"--dry-run=client", "-o", "yaml",
	)

	// Pipe the output of kubectl create to kubectl apply
	applyCmd := exec.Command("kubectl", "apply", "-f", "-")
	pipe, err := createCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %v", err)
	}
	applyCmd.Stdin = pipe

	if err := createCmd.Start(); err != nil {
		return fmt.Errorf("failed to start create secret command: %v", err)
	}
	if err := applyCmd.Start(); err != nil {
		return fmt.Errorf("failed to start apply command: %v", err)
	}
	if err := createCmd.Wait(); err != nil {
		return fmt.Errorf("create secret command failed: %v", err)
	}
	if err := applyCmd.Wait(); err != nil {
		return fmt.Errorf("apply command failed: %v", err)
	}
	return nil
}

func createGiteaAccount(secretName, accountName, password, email string) error {
	// Get Gitea pod
	getPodsCmd := exec.Command("kubectl", "get", "pods", "-n", "gitea", "-l", "app=gitea", "-o", `jsonpath={.items[*].metadata.name}`)
	podsOut, err := getPodsCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get Gitea pods: %v", err)
	}
	giteaPods := strings.Fields(string(podsOut))
	if len(giteaPods) == 0 {
		return fmt.Errorf("no Gitea pods found")
	}
	giteaPod := giteaPods[0]

	// Check if user exists
	listCmd := exec.Command("kubectl", "exec", "-n", "gitea", giteaPod, "-c", "gitea", "--", "gitea", "admin", "user", "list")
	listOut, err := listCmd.Output()
	userExists := false
	if err == nil && bytes.Contains(listOut, []byte(accountName)) {
		userExists = true
	}

	if !userExists {
		fmt.Printf("Creating Gitea account for %s\n", accountName)
		createCmd := exec.Command(
			"kubectl", "exec", "-n", "gitea", giteaPod, "-c", "gitea", "--",
			"gitea", "admin", "user", "create",
			"--username", accountName,
			"--password", password,
			"--email", email,
			"--must-change-password=false",
		)
		if out, err := createCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create Gitea user: %v\n%s", err, string(out))
		}
	} else {
		fmt.Printf("Gitea account for %s already exists, updating password\n", accountName)
		changeCmd := exec.Command(
			"kubectl", "exec", "-n", "gitea", giteaPod, "-c", "gitea", "--",
			"gitea", "admin", "user", "change-password",
			"--username", accountName,
			"--password", password,
			"--must-change-password=false",
		)
		if out, err := changeCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to change Gitea user password: %v\n%s", err, string(out))
		}
	}

	// Generate access token
	tokenName := fmt.Sprintf("%s-%d", accountName, time.Now().Unix())
	tokenCmd := exec.Command(
		"kubectl", "exec", "-n", "gitea", giteaPod, "-c", "gitea", "--",
		"gitea", "admin", "user", "generate-access-token",
		"--scopes", "write:repository,write:user",
		"--username", accountName,
		"--token-name", tokenName,
	)
	tokenOut, err := tokenCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to generate access token: %v", err)
	}
	fields := strings.Fields(string(tokenOut))
	token := fields[len(fields)-1]

	// Create Kubernetes secret with the token
	secretCmd := exec.Command(
		"kubectl", "create", "secret", "generic", "gitea-"+accountName+"-token",
		"-n", "gitea",
		"--from-literal=token="+token,
	)
	if out, err := secretCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create token secret: %v\n%s", err, string(out))
	}

	return nil
}

// TODO I think this function exists elsewhere - use that existing one
func randomPassword(length int) (string, error) {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	password := make([]byte, length)
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", fmt.Errorf("failed to generate random number: %v", err)
		}
		password[i] = chars[num.Int64()]
	}
	return string(password), nil
}
