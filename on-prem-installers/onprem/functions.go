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

	"bufio"

	"path/filepath"
	"regexp"

	"github.com/magefile/mage/mg"
)

var orchNamespaceList = []string{
	"onprem",
	"orch-boots",
	"orch-database",
	"orch-platform",
	"orch-app",
	"orch-cluster",
	"orch-infra",
	"orch-sre",
	"orch-ui",
	"orch-secret",
	"orch-gateway",
	"orch-harbor",
	"cattle-system",
}

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

// GeneratePassword generates a random 100-character alphanumeric password.
func (OnPrem) GenerateHarborPassword() (string, error) {
	const length = 100
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	var sb strings.Builder

	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		sb.WriteByte(chars[num.Int64()])
	}
	return sb.String(), nil
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
func DownloadArtifacts(cwd, dirName, rsURL, rsPath string, artifacts ...string) error {
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

func (OnPrem) CreateNamespaces() error {
	for _, ns := range orchNamespaceList {
		cmd := exec.Command("kubectl", "create", "ns", ns, "--dry-run=client", "-o", "yaml")
		apply := exec.Command("kubectl", "apply", "-f", "-")
		pipe, err := cmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("failed to get stdout pipe: %w", err)
		}
		apply.Stdin = pipe
		apply.Stdout = nil
		apply.Stderr = nil
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start kubectl create: %w", err)
		}
		if err := apply.Run(); err != nil {
			return fmt.Errorf("failed to apply namespace: %w", err)
		}
		if err := cmd.Wait(); err != nil {
			return fmt.Errorf("kubectl create wait failed: %w", err)
		}
	}
	return nil
}

func (OnPrem) CreateSreSecrets() error {
	if os.Getenv("SRE_USERNAME") == "" {
		os.Setenv("SRE_USERNAME", "sre")
	}
	if os.Getenv("SRE_PASSWORD") == "" {
		if os.Getenv("ORCH_DEFAULT_PASSWORD") == "" {
			os.Setenv("SRE_PASSWORD", "123")
		} else {
			os.Setenv("SRE_PASSWORD", os.Getenv("ORCH_DEFAULT_PASSWORD"))
		}
	}
	if os.Getenv("SRE_DEST_URL") == "" {
		os.Setenv("SRE_DEST_URL", "http://sre-exporter-destination.orch-sre.svc.cluster.local:8428/api/v1/write")
	}
	// SRE_DEST_CA_CERT is not set by default

	namespace := "orch-sre"
	sreUsername := os.Getenv("SRE_USERNAME")
	srePassword := os.Getenv("SRE_PASSWORD")
	sreDestURL := os.Getenv("SRE_DEST_URL")
	sreDestCACert := os.Getenv("SRE_DEST_CA_CERT")

	// Delete existing secrets
	secrets := []string{
		"basic-auth-username",
		"basic-auth-password",
		"destination-secret-url",
		"destination-secret-ca",
	}
	for _, secret := range secrets {
		exec.Command("kubectl", "-n", namespace, "delete", "secret", secret, "--ignore-not-found").Run()
	}

	// Create basic-auth-username secret
	secret1 := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: basic-auth-username
  namespace: %s
stringData:
  username: %s
`, namespace, sreUsername)
	if err := applySecret(secret1); err != nil {
		return err
	}

	// Create basic-auth-password secret
	secret2 := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: basic-auth-password
  namespace: %s
stringData:
  password: "%s"
`, namespace, srePassword)
	if err := applySecret(secret2); err != nil {
		return err
	}

	// Create destination-secret-url secret
	secret3 := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: destination-secret-url
  namespace: %s
stringData:
  url: %s
`, namespace, sreDestURL)
	if err := applySecret(secret3); err != nil {
		return err
	}

	// Create destination-secret-ca secret if SRE_DEST_CA_CERT is set
	if sreDestCACert != "" {
		// Indent each line of the CA cert by 4 spaces
		indented := "    " + strings.ReplaceAll(sreDestCACert, "\n", "\n    ")
		secret4 := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: destination-secret-ca
  namespace: %s
stringData:
  ca.crt: |
%s
`, namespace, indented)
		if err := applySecret(secret4); err != nil {
			return err
		}
	}

	return nil
}

// // Helper function to apply a secret using kubectl
// func applySecret(secret string) error {
// 	cmd := exec.Command("kubectl", "apply", "-f", "-")
// 	cmd.Stdin = strings.NewReader(secret)
// 	cmd.Stdout = os.Stdout
// 	cmd.Stderr = os.Stderr
// 	return cmd.Run()
// }

func (OnPrem) PrintEnvVariables() {
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("         Environment Variables")
	fmt.Println("========================================")
	fmt.Printf("%-25s: %s\n", "RELEASE_SERVICE_URL", os.Getenv("RELEASE_SERVICE_URL"))
	fmt.Printf("%-25s: %s\n", "ORCH_INSTALLER_PROFILE", os.Getenv("ORCH_INSTALLER_PROFILE"))
	fmt.Printf("%-25s: %s\n", "DEPLOY_VERSION", os.Getenv("DEPLOY_VERSION"))
	fmt.Println("========================================")
	fmt.Println()
}

func (OnPrem) AllowConfigInRuntime() error {
	enableTrace := os.Getenv("ENABLE_TRACE") == "true"
	cwd := os.Getenv("cwd")
	// cwd, _ := os.Getwd()
	gitArchName := os.Getenv("git_arch_name")
	siConfigRepo := os.Getenv("si_config_repo")
	assumeYes := os.Getenv("ASSUME_YES") == "true"

	tmpDir := filepath.Join(cwd, gitArchName, "tmp")
	configRepoPath := filepath.Join(tmpDir, siConfigRepo)

	// Disable tracing if enabled (not implemented in Go, just print)
	if enableTrace {
		fmt.Println("Tracing is enabled. Temporarily disabling tracing")
	}

	// Check if config already exists
	if _, err := os.Stat(configRepoPath); err == nil {
		fmt.Printf("Configuration already exists at %s.\n", configRepoPath)
		if assumeYes {
			fmt.Println("Assuming yes to use existing configuration.")
			return nil
		}
		reader := bufio.NewReader(os.Stdin)
		for {
			fmt.Print("Do you want to overwrite the existing configuration? (yes/no): ")
			yn, _ := reader.ReadString('\n')
			yn = strings.TrimSpace(yn)
			switch strings.ToLower(yn) {
			case "y", "yes":
				os.RemoveAll(configRepoPath)
				break
			case "n", "no":
				fmt.Println("Using existing configuration.")
				return nil
			default:
				fmt.Println("Please answer yes or no.")
			}
		}
	}

	// Untar edge-manageability-framework repo
	repoFile := ""
	files, _ := filepath.Glob(filepath.Join(cwd, gitArchName, fmt.Sprintf("*%s*.tgz", siConfigRepo)))
	if len(files) > 0 {
		repoFile = filepath.Base(files[0])
	}
	os.RemoveAll(tmpDir)
	os.MkdirAll(tmpDir, 0755)
	cmd := exec.Command("tar", "-xf", filepath.Join(cwd, gitArchName, repoFile), "-C", tmpDir)
	fmt.Println("cmd:", cmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	// Prompt for Docker.io credentials
	reader := bufio.NewReader(os.Stdin)
	dockerUsername := os.Getenv("DOCKER_USERNAME")
	dockerPassword := os.Getenv("DOCKER_PASSWORD")
	for {
		if dockerUsername == "" && dockerPassword == "" {
			fmt.Print("Would you like to provide Docker credentials? (Y/n): ")
			yn, _ := reader.ReadString('\n')
			yn = strings.TrimSpace(yn)
			if strings.ToLower(yn) == "y" || yn == "" {
				fmt.Print("Enter Docker Username: ")
				dockerUsername, _ = reader.ReadString('\n')
				dockerUsername = strings.TrimSpace(dockerUsername)
				os.Setenv("DOCKER_USERNAME", dockerUsername)
				fmt.Print("Enter Docker Password: ")
				dockerPassword, _ = reader.ReadString('\n')
				dockerPassword = strings.TrimSpace(dockerPassword)
				os.Setenv("DOCKER_PASSWORD", dockerPassword)
				break
			} else if strings.ToLower(yn) == "n" {
				fmt.Println("The installation will proceed without using Docker credentials.")
				os.Unsetenv("DOCKER_USERNAME")
				os.Unsetenv("DOCKER_PASSWORD")
				break
			} else {
				fmt.Println("Please answer yes or no.")
			}
		} else {
			fmt.Println("Setting Docker credentials.")
			os.Setenv("DOCKER_USERNAME", dockerUsername)
			os.Setenv("DOCKER_PASSWORD", dockerPassword)
			break
		}
	}

	if dockerUsername != "" && dockerPassword != "" {
		fmt.Println("Docker credentials are set.")
	} else {
		fmt.Println("Docker credentials are not valid. The installation will proceed without using Docker credentials.")
		os.Unsetenv("DOCKER_USERNAME")
		os.Unsetenv("DOCKER_PASSWORD")
	}

	// Prompt for IP addresses for Argo, Traefik and Nginx services
	fmt.Println("Provide IP addresses for Argo, Traefik and Nginx services.")
	ipRegex := regexp.MustCompile(`^\d{1,3}(\.\d{1,3}){3}$`)
	var argoIP, traefikIP, nginxIP string
	for {
		if argoIP == "" {
			fmt.Print("Enter Argo IP: ")
			argoIP, _ = reader.ReadString('\n')
			argoIP = strings.TrimSpace(argoIP)
			os.Setenv("ARGO_IP", argoIP)
		}
		if traefikIP == "" {
			fmt.Print("Enter Traefik IP: ")
			traefikIP, _ = reader.ReadString('\n')
			traefikIP = strings.TrimSpace(traefikIP)
			os.Setenv("TRAEFIK_IP", traefikIP)
		}
		if nginxIP == "" {
			fmt.Print("Enter Nginx IP: ")
			nginxIP, _ = reader.ReadString('\n')
			nginxIP = strings.TrimSpace(nginxIP)
			os.Setenv("NGINX_IP", nginxIP)
		}
		if ipRegex.MatchString(argoIP) && ipRegex.MatchString(traefikIP) && ipRegex.MatchString(nginxIP) {
			fmt.Println("IP addresses are valid.")
			break
		} else {
			fmt.Println("Inputted values are not valid IPs. Please input correct IPs without any masks.")
			argoIP, traefikIP, nginxIP = "", "", ""
			os.Unsetenv("ARGO_IP")
			os.Unsetenv("TRAEFIK_IP")
			os.Unsetenv("NGINX_IP")
		}
	}

	// Wait for SI to confirm that they have made changes
	for {
		proceed := os.Getenv("PROCEED")
		if proceed != "" {
			break
		}
		fmt.Printf(`Edit config values.yaml files with custom configurations if necessary!!!
The files are located at:
%s/%s/orch-configs/profiles/<profile>.yaml
%s/%s/orch-configs/clusters/$ORCH_INSTALLER_PROFILE.yaml
Enter 'yes' to confirm that configuration is done in order to progress with installation
('no' will exit the script) !!!

Ready to proceed with installation? `, tmpDir, siConfigRepo, tmpDir, siConfigRepo)
		yn, _ := reader.ReadString('\n')
		yn = strings.TrimSpace(yn)
		switch strings.ToLower(yn) {
		case "y", "yes":
			break
		case "n", "no":
			os.Exit(1)
		default:
			fmt.Println("Please answer yes or no.")
			continue
		}
		break
	}

	// Re-enable tracing if needed
	if enableTrace {
		fmt.Println("Tracing is enabled. Re-enabling tracing")
	}

	return nil
}

func (OnPrem) CreateAzureSecret() error {
	namespace := "orch-secret"
	azureRefreshToken := os.Getenv("AZUREAD_REFRESH_TOKEN")

	// Delete the existing secret if it exists
	exec.Command("kubectl", "-n", namespace, "delete", "secret", "azure-ad-creds", "--ignore-not-found").Run()

	// Define the secret YAML
	secret := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: azure-ad-creds
  namespace: %s
stringData:
  refresh_token: %s
`, namespace, azureRefreshToken)

	// Apply the secret using kubectl
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(secret)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (OnPrem) CreateSmtpSecrets() error {
	if os.Getenv("SMTP_ADDRESS") == "" {
		os.Setenv("SMTP_ADDRESS", "smtp.serveraddress.com")
	}
	if os.Getenv("SMTP_PORT") == "" {
		os.Setenv("SMTP_PORT", "587")
	}
	// Firstname Lastname <email@example.com> format expected
	if os.Getenv("SMTP_HEADER") == "" {
		os.Setenv("SMTP_HEADER", "foo bar <foo@bar.com>")
	}
	if os.Getenv("SMTP_USERNAME") == "" {
		os.Setenv("SMTP_USERNAME", "uSeR")
	}
	if os.Getenv("SMTP_PASSWORD") == "" {
		os.Setenv("SMTP_PASSWORD", "T@123sfD")
	}

	namespace := "orch-infra"
	smtpAddress := os.Getenv("SMTP_ADDRESS")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpHeader := os.Getenv("SMTP_HEADER")
	smtpUsername := os.Getenv("SMTP_USERNAME")
	smtpPassword := os.Getenv("SMTP_PASSWORD")

	// Delete existing secrets
	exec.Command("kubectl", "-n", namespace, "delete", "secret", "smtp", "--ignore-not-found").Run()
	exec.Command("kubectl", "-n", namespace, "delete", "secret", "smtp-auth", "--ignore-not-found").Run()

	// Create smtp secret
	smtpSecret := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: smtp
  namespace: %s
type: Opaque
stringData:
  smartHost: %s
  smartPort: "%s"
  from: %s
  authUsername: %s
`, namespace, smtpAddress, smtpPort, smtpHeader, smtpUsername)
	if err := applySecret(smtpSecret); err != nil {
		return err
	}

	// Create smtp-auth secret
	smtpAuthSecret := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: smtp-auth
  namespace: %s
type: kubernetes.io/basic-auth
stringData:
  password: %s
`, namespace, smtpPassword)
	if err := applySecret(smtpAuthSecret); err != nil {
		return err
	}

	return nil
}

func (OnPrem) WriteConfigsUsingOverrides() error {
	tmpDir := os.Getenv("tmp_dir")
	siConfigRepo := os.Getenv("si_config_repo")
	profile := os.Getenv("ORCH_INSTALLER_PROFILE")
	yamlPath := fmt.Sprintf("%s/%s/orch-configs/clusters/%s.yaml", tmpDir, siConfigRepo, profile)

	clusterDomain := os.Getenv("CLUSTER_DOMAIN")
	if clusterDomain != "" {
		fmt.Println("CLUSTER_DOMAIN is set. Updating clusterDomain in the YAML file...")
		cmd := exec.Command("yq", "-i", fmt.Sprintf(".argo.clusterDomain=\"%s\"", clusterDomain), yamlPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
		fmt.Printf("Update complete. clusterDomain is now set to: %s\n", clusterDomain)
	}

	sreTlsEnabled := os.Getenv("SRE_TLS_ENABLED")
	sreDestCaCert := os.Getenv("SRE_DEST_CA_CERT")
	if sreTlsEnabled == "true" {
		cmd := exec.Command("yq", "-i", ".argo.o11y.sre.tls.enabled|=true", yamlPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
		if sreDestCaCert != "" {
			cmd := exec.Command("yq", "-i", ".argo.o11y.sre.tls.caSecretEnabled|=true", yamlPath)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return err
			}
		}
	} else {
		cmd := exec.Command("yq", "-i", ".argo.o11y.sre.tls.enabled|=false", yamlPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	if os.Getenv("SMTP_SKIP_VERIFY") == "true" {
		cmd := exec.Command("yq", "-i", ".argo.o11y.alertingMonitor.smtp.insecureSkipVerify|=true", yamlPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	// Override MetalLB address pools
	cmds := []*exec.Cmd{
		exec.Command("yq", "-i", ".postCustomTemplateOverwrite.metallb-config.ArgoIP|=strenv(ARGO_IP)", yamlPath),
		exec.Command("yq", "-i", ".postCustomTemplateOverwrite.metallb-config.TraefikIP|=strenv(TRAEFIK_IP)", yamlPath),
		exec.Command("yq", "-i", ".postCustomTemplateOverwrite.metallb-config.NginxIP|=strenv(NGINX_IP)", yamlPath),
	}
	for _, cmd := range cmds {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	return nil
}

// DownloadPackages cleans up and downloads .deb and .git packages with retry logic.
func (OnPrem) DownloadPackages() error {
	skipDownload := os.Getenv("SKIP_DOWNLOAD") == "true"
	cwd := os.Getenv("cwd")
	debDirName := os.Getenv("deb_dir_name")
	gitArchName := os.Getenv("git_arch_name")
	releaseServiceURL := os.Getenv("RELEASE_SERVICE_URL")
	installerRSPath := os.Getenv("installer_rs_path")
	archivesRSPath := os.Getenv("archives_rs_path")

	// These lists should be set by your set_artifacts_version logic
	installerList := []string{
		fmt.Sprintf("onprem-config-installer:%s", os.Getenv("DEPLOY_VERSION")),
		fmt.Sprintf("onprem-ke-installer:%s", os.Getenv("DEPLOY_VERSION")),
		fmt.Sprintf("onprem-argocd-installer:%s", os.Getenv("DEPLOY_VERSION")),
		fmt.Sprintf("onprem-gitea-installer:%s", os.Getenv("DEPLOY_VERSION")),
		fmt.Sprintf("onprem-orch-installer:%s", os.Getenv("DEPLOY_VERSION")),
	}
	gitArchiveList := []string{
		fmt.Sprintf("onpremfull:%s", os.Getenv("DEPLOY_VERSION")),
	}

	if !skipDownload {
		// Cleanup .deb packages
		exec.Command("sudo", "rm", "-rf", fmt.Sprintf("%s/%s", cwd, debDirName)).Run()

		retryCount := 0
		maxRetries := 10
		retryDelay := 15 * time.Second

		// Download .deb packages with retry
		for {
			err := DownloadArtifacts(cwd, debDirName, releaseServiceURL, installerRSPath, installerList...)
			if err == nil {
				break
			}
			retryCount++
			if retryCount >= maxRetries {
				fmt.Printf("Failed to download deb artifacts after %d attempts.\n", maxRetries)
				return err
			}
			fmt.Printf("Download failed. Retrying in %d seconds... (%d/%d)\n", int(retryDelay.Seconds()), retryCount, maxRetries)
			time.Sleep(retryDelay)
		}

		exec.Command("sudo", "chown", "-R", "_apt:root", debDirName).Run()

		// Cleanup .git packages
		exec.Command("sudo", "rm", "-rf", fmt.Sprintf("%s/%s", cwd, gitArchName)).Run()

		retryCount = 0
		// Download .git packages with retry
		for {
			err := DownloadArtifacts(cwd, gitArchName, releaseServiceURL, archivesRSPath, gitArchiveList...)
			if err == nil {
				break
			}
			retryCount++
			if retryCount >= maxRetries {
				fmt.Printf("Failed to download git artifacts after %d attempts.\n", maxRetries)
				return err
			}
			fmt.Printf("Download failed. Retrying in %d seconds... (%d/%d)\n", int(retryDelay.Seconds()), retryCount, maxRetries)
			time.Sleep(retryDelay)
		}
	} else {
		fmt.Println("Skipping packages download")
		exec.Command("sudo", "chown", "-R", "_apt:root", debDirName).Run()
	}
	return nil
}

// validateAndSetIP checks and sets an IP in the YAML config using yq, prompting the user if needed.
func validateAndSetIP(yamlPath, yamlFile, ipVarName string) error {
	// Read current value from YAML
	cmd := exec.Command("yq", yamlPath, yamlFile)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to read %s from %s: %w", yamlPath, yamlFile, err)
	}
	val := strings.TrimSpace(string(out))
	fmt.Printf("Value at %s in %s: %s\n", yamlPath, yamlFile, val)

	if val == "" || val == "null" {
		reader := bufio.NewReader(os.Stdin)
		ipRegex := regexp.MustCompile(`^\d{1,3}(\.\d{1,3}){3}$`)
		for {
			fmt.Printf("%s is not set to a valid value in the configuration file.\n", ipVarName)
			fmt.Printf("Please provide a value for %s: ", ipVarName)
			ipValue, _ := reader.ReadString('\n')
			ipValue = strings.TrimSpace(ipValue)
			if ipRegex.MatchString(ipValue) {
				os.Setenv(ipVarName, ipValue)
				cmd := exec.Command("yq", "-i", fmt.Sprintf("%s|=strenv(%s)", yamlPath, ipVarName), yamlFile)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					return fmt.Errorf("failed to set %s in %s: %w", ipVarName, yamlFile, err)
				}
				fmt.Printf("%s has been set to: %s\n", ipVarName, ipValue)
				break
			} else {
				os.Unsetenv(ipVarName)
				fmt.Print("Invalid IP address. Would you like to provide a valid value? (Y/n): ")
				yn, _ := reader.ReadString('\n')
				yn = strings.TrimSpace(yn)
				if strings.ToLower(yn) == "n" {
					return fmt.Errorf("Exiting as a valid value for %s has not been provided.", ipVarName)
				}
			}
		}
	}
	return nil
}

// ValidateConfig validates the IP addresses for Argo, Traefik, and Nginx services in the config.
func (OnPrem) ValidateConfig() error {
	tmpDir := os.Getenv("tmp_dir")
	siConfigRepo := os.Getenv("si_config_repo")
	profile := os.Getenv("ORCH_INSTALLER_PROFILE")
	yamlFile := fmt.Sprintf("%s/%s/orch-configs/clusters/%s.yaml", tmpDir, siConfigRepo, profile)

	if err := validateAndSetIP(".postCustomTemplateOverwrite.metallb-config.ArgoIP", yamlFile, "ARGO_IP"); err != nil {
		return err
	}
	if err := validateAndSetIP(".postCustomTemplateOverwrite.metallb-config.TraefikIP", yamlFile, "TRAEFIK_IP"); err != nil {
		return err
	}
	if err := validateAndSetIP(".postCustomTemplateOverwrite.metallb-config.NginxIP", yamlFile, "NGINX_IP"); err != nil {
		return err
	}
	return nil
}

func (OnPrem) WriteConfigToDisk() error {
	cwd := os.Getenv("cwd")
	gitArchName := os.Getenv("git_arch_name")
	siConfigRepo := os.Getenv("si_config_repo")
	tmpDir := filepath.Join(cwd, gitArchName, "tmp")
	os.Setenv("tmp_dir", tmpDir)

	// Remove and recreate tmpDir
	exec.Command("rm", "-rf", tmpDir).Run()
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return fmt.Errorf("failed to create tmp dir: %w", err)
	}

	// Find the repo archive file
	findCmd := exec.Command("find", filepath.Join(cwd, gitArchName), "-name", fmt.Sprintf("*%s*.tgz", siConfigRepo), "-type", "f", "-printf", "%f\n")
	out, err := findCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to find repo archive: %w", err)
	}
	repoFile := strings.TrimSpace(string(out))
	if repoFile == "" {
		return fmt.Errorf("repo archive not found")
	}

	// Extract the archive
	tarCmd := exec.Command("tar", "-xf", filepath.Join(cwd, gitArchName, repoFile), "-C", tmpDir)
	tarCmd.Stdout = os.Stdout
	tarCmd.Stderr = os.Stderr
	if err := tarCmd.Run(); err != nil {
		return fmt.Errorf("failed to extract repo archive: %w", err)
	}

	// Apply config overrides
	if err := (OnPrem{}).WriteConfigsUsingOverrides(); err != nil {
		return fmt.Errorf("failed to apply config overrides: %w", err)
	}

	fmt.Printf("Configuration files have been written to disk at %s/%s\n", tmpDir, siConfigRepo)
	os.Exit(0)
	return nil
}

func (OnPrem) InstallOrchestrator() error {
	cwd := os.Getenv("cwd")
	debDirName := os.Getenv("deb_dir_name")
	orchInstallerProfile := os.Getenv("ORCH_INSTALLER_PROFILE")
	gitRepos := os.Getenv("GIT_REPOS")

	fmt.Println("Installing Edge Orchestrator Packages")

	cmd := exec.Command(
		"sudo", "env",
		"NEEDRESTART_MODE=a",
		"DEBIAN_FRONTEND=noninteractive",
		fmt.Sprintf("ORCH_INSTALLER_PROFILE=%s", orchInstallerProfile),
		fmt.Sprintf("GIT_REPOS=%s", gitRepos),
		"apt-get", "install", "-y",
		fmt.Sprintf("%s/%s/onprem-orch-installer_*_amd64.deb", cwd, debDirName),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install Edge Orchestrator: %w", err)
	}

	fmt.Println("Edge Orchestrator getting installed, wait for SW to deploy...")

	fmt.Printf(`
Edge Orchestrator SW is being deployed, please wait for all applications to deploy...
To check the status of the deployment run 'kubectl get applications -A'.
Installation is completed when 'root-app' Application is in 'Healthy' and 'Synced' state.
Once it is completed, you might want to configure DNS for UI and other services by running generate_fqdn script and following instructions
`)

	return nil
}
