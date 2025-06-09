// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

//go:build mage

package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"strings"
	"time"
	"bufio"
    "os/user"
	"path/filepath"
	"regexp"
	"gopkg.in/yaml.v3"

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

    // Helper to get/set nested keys
func getNested(m map[string]interface{}, keys []string) (string, bool) {
    for i, k := range keys {
        if i == len(keys)-1 {
            if v, ok := m[k]; ok {
                if s, ok := v.(string); ok {
                    return s, true
                }
            }
            return "", false
        }
        if next, ok := m[k].(map[string]interface{}); ok {
            m = next
        } else {
            return "", false
        }
    }
    return "", false
}

func setNested(m map[string]interface{}, keys []string, value string) {
    for _, k := range keys[:len(keys)-1] {
        if _, ok := m[k]; !ok {
            m[k] = make(map[string]interface{})
        }
        m = m[k].(map[string]interface{})
    }
    m[keys[len(keys)-1]] = value
}

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
func (OnPrem) GeneratePassword() (string, error){
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
	fmt.Printf("%s\n", sb.String())
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
		cmd := exec.Command("kubectl", "get", "ns", namespace, "-o", "jsonpath={.status.phase}")
		out, err := cmd.Output()
		if err != nil {
			return err
		}
		if string(out) == "Active" {
			break
		}
		fmt.Printf("%s\n", string(out))
		fmt.Printf("Waiting for namespace %s to be created...\n", namespace)
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
	argoIP = os.Getenv("ARGO_IP")
	traefikIP = os.Getenv("TRAEFIK_IP")
	nginxIP = os.Getenv("NGINX_IP")

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

    // Read YAML file
    data, err := os.ReadFile(yamlPath)
    if err != nil {
        return err
    }
    var config map[string]interface{}
    if err := yaml.Unmarshal(data, &config); err != nil {
        return err
    }

    // Use setNested to update values
    clusterDomain := os.Getenv("CLUSTER_DOMAIN")
    if clusterDomain != "" {
        setNested(config, []string{"argo", "clusterDomain"}, clusterDomain)
    }

    sreTlsEnabled := os.Getenv("SRE_TLS_ENABLED")
    sreDestCaCert := os.Getenv("SRE_DEST_CA_CERT")
    if sreTlsEnabled == "true" {
        setNested(config, []string{"argo", "o11y", "sre", "tls", "enabled"}, "true")
        if sreDestCaCert != "" {
            setNested(config, []string{"argo", "o11y", "sre", "tls", "caSecretEnabled"}, "true")
        }
    } else {
        setNested(config, []string{"argo", "o11y", "sre", "tls", "enabled"}, "false")
    }

    if os.Getenv("SMTP_SKIP_VERIFY") == "true" {
        setNested(config, []string{"argo", "o11y", "alertingMonitor", "smtp", "insecureSkipVerify"}, "true")
    }

    setNested(config, []string{"postCustomTemplateOverwrite", "metallb-config", "ArgoIP"}, os.Getenv("ARGO_IP"))
    setNested(config, []string{"postCustomTemplateOverwrite", "metallb-config", "TraefikIP"}, os.Getenv("TRAEFIK_IP"))
    setNested(config, []string{"postCustomTemplateOverwrite", "metallb-config", "NginxIP"}, os.Getenv("NGINX_IP"))

    // Write YAML back
    out, err := yaml.Marshal(config)
    if err != nil {
        return err
    }
    return os.WriteFile(yamlPath, out, 0644)
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
		// fmt.Sprintf("onprem-config-installer:%s", os.Getenv("DEPLOY_VERSION")),
		fmt.Sprintf("onprem-ke-installer:%s", os.Getenv("DEPLOY_VERSION")),
		fmt.Sprintf("onprem-argocd-installer:%s", os.Getenv("DEPLOY_VERSION")),
		// fmt.Sprintf("onprem-gitea-installer:%s", os.Getenv("DEPLOY_VERSION")),
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

// validateAndSetIP checks and sets an IP in the YAML config using, prompting the user if needed.
func validateAndSetIP(yamlPath, yamlFile, ipVarName string) error {
    // Read YAML file
    data, err := os.ReadFile(yamlFile)
    if err != nil {
        return fmt.Errorf("failed to read %s: %w", yamlFile, err)
    }
    var config map[string]interface{}
    if err := yaml.Unmarshal(data, &config); err != nil {
        return fmt.Errorf("failed to unmarshal YAML: %w", err)
    }

    // Parse yamlPath to keys
    keys := strings.Split(strings.Trim(yamlPath, "."), ".")
    val, ok := getNested(config, keys)
    fmt.Printf("Value at %s in %s: %s\n", yamlPath, yamlFile, val)

    reader := bufio.NewReader(os.Stdin)
    ipRegex := regexp.MustCompile(`^\d{1,3}(\.\d{1,3}){3}$`)
    for !ok || val == "" || val == "null" {
        fmt.Printf("%s is not set to a valid value in the configuration file.\n", ipVarName)
        fmt.Printf("Please provide a value for %s: ", ipVarName)
        ipValue, _ := reader.ReadString('\n')
        ipValue = strings.TrimSpace(ipValue)
        if ipRegex.MatchString(ipValue) {
            os.Setenv(ipVarName, ipValue)
            setNested(config, keys, ipValue)
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

    // Write YAML back
    out, err := yaml.Marshal(config)
    if err != nil {
        return err
    }
    return os.WriteFile(yamlFile, out, 0644)
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
	tmpDir := os.Getenv("tmp_dir")
	cwd := os.Getenv("cwd")
	gitArchName := os.Getenv("git_arch_name")
	siConfigRepo := os.Getenv("si_config_repo")
	repoFile := os.Getenv("repo_file")
	// os.Getenv("tmp_dir")

	// Remove and recreate tmpDir
	exec.Command("rm", "-rf", tmpDir).Run()
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return fmt.Errorf("failed to create tmp dir: %w", err)
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
	// os.Exit(0)
	return nil
}

func (OnPrem) Usage() {
    prog := filepath.Base(os.Args[0])
    fmt.Fprintf(os.Stderr, `Purpose:
Install OnPrem Edge Orchestrator.

Usage:
%s [option...] [argument]

ex:
./%s
./%s -c <certificate string>

Options:
    -h, --help         Print this help message and exit
    -c, --cert         Path to Release Service/ArgoCD certificate
    -s, --sre          Path to SRE destination CA certificate (enables TLS for SRE Exporter)
    --skip-download    Skip downloading installer packages 
    -d, --notls        Disable TLS verification for SMTP endpoint
    -o, --override     Override production values with dev values
    -u, --url          Set the Release Service URL
    -t, --trace        Enable tracing
    -w, --write-config Write configuration to disk and exit
    -y, --yes          Assume yes for using existing configuration if it exists

Environment Variables:
    DOCKER_USERNAME    Docker.io username
    DOCKER_PASSWORD    Docker.io password

`, prog, prog, prog)
}

func (OnPrem) InstallOrchestrator() error {
	cwd := os.Getenv("cwd")
	debDirName := os.Getenv("deb_dir_name")
	orchInstallerProfile := os.Getenv("ORCH_INSTALLER_PROFILE")
	gitRepos := os.Getenv("GIT_REPOS")

	fmt.Println("Installing Edge Orchestrator Packages")

pattern := fmt.Sprintf("%s/%s/onprem-orch-installer_*_amd64.deb", cwd, debDirName)
matches, err := filepath.Glob(pattern)
if err != nil || len(matches) == 0 {
    return fmt.Errorf("no deb package found matching pattern: %s", pattern)
}
debFile := matches[0] // or handle multiple matches as needed

cmd := exec.Command(
    "sudo",
    "NEEDRESTART_MODE=a",
    "DEBIAN_FRONTEND=noninteractive",
    fmt.Sprintf("ORCH_INSTALLER_PROFILE=%s", orchInstallerProfile),
    fmt.Sprintf("GIT_REPOS=%s", gitRepos),
    "apt-get", "install", "-y",
    debFile,
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

// ParseArgs parses CLI arguments and sets environment variables accordingly.
func (OnPrem) ParseArgs() {
    args := os.Args[1:]
    for i := 0; i < len(args); i++ {
        arg := args[i]
        switch arg {
        case "-h", "--help":
            OnPrem{}.Usage()
            os.Exit(0)
        case "-s", "--sre_tls":
            os.Setenv("SRE_TLS_ENABLED", "true")
            if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
                certPath := args[i+1]
                data, err := os.ReadFile(certPath)
                if err != nil {
                    fmt.Fprintf(os.Stderr, "Failed to read SRE CA cert: %v\n", err)
                    os.Exit(1)
                }
                os.Setenv("SRE_DEST_CA_CERT", string(data))
                i++
            }
        case "--skip-download":
            os.Setenv("SKIP_DOWNLOAD", "true")
        case "-d", "--notls":
            os.Setenv("SMTP_SKIP_VERIFY", "true")
        case "-o", "--override":
            os.Setenv("ORCH_INSTALLER_PROFILE", "onprem-dev")
        case "-u", "--url":
            if i+1 < len(args) {
                os.Setenv("RELEASE_SERVICE_URL", args[i+1])
                i++
            } else {
                fmt.Fprintf(os.Stderr, "ERROR: %s requires an argument\n", arg)
                os.Exit(1)
            }
        case "-t", "--trace":
            os.Setenv("ENABLE_TRACE", "true")
            // No direct equivalent of set -x in Go
        case "-w", "--write-config":
            os.Setenv("WRITE_CONFIG", "true")
        case "-y", "--yes":
            os.Setenv("ASSUME_YES", "true")
        default:
            if strings.HasPrefix(arg, "-") {
                fmt.Fprintf(os.Stderr, "Unknown argument %s\n", arg)
                os.Exit(1)
            } else {
                break
            }
        }
    }
}

func (OnPrem) InstallRKE2() error {
    fmt.Println("Installing RKE2...")

    cwd := os.Getenv("cwd")
    debDirName := os.Getenv("deb_dir_name")
    installerPattern := fmt.Sprintf("%s/%s/onprem-ke-installer_*_amd64.deb", cwd, debDirName)
    matches, err := filepath.Glob(installerPattern)
    if err != nil || len(matches) == 0 {
        return fmt.Errorf("no RKE2 installer package found at %s", installerPattern)
    }
    debFile := matches[0]

    dockerUser := os.Getenv("DOCKER_USERNAME")
    dockerPass := os.Getenv("DOCKER_PASSWORD")

    var cmd *exec.Cmd
    if dockerUser != "" && dockerPass != "" {
        fmt.Println("Docker credentials provided. Installing RKE2 with Docker credentials")
        cmd = exec.Command(
            "sudo",
            "env",
            fmt.Sprintf("DOCKER_USERNAME=%s", dockerUser),
            fmt.Sprintf("DOCKER_PASSWORD=%s", dockerPass),
            "NEEDRESTART_MODE=a",
            "DEBIAN_FRONTEND=noninteractive",
            "apt-get", "install", "-y", debFile,
        )
    } else {
        cmd = exec.Command(
            "sudo",
            "NEEDRESTART_MODE=a",
            "DEBIAN_FRONTEND=noninteractive",
            "apt-get", "install", "-y", debFile,
        )
    }
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("failed to install RKE2: %v", err)
    }

    fmt.Println("OS level configuration installed and RKE2 Installed")

    user := os.Getenv("USER")
    kubeDir := fmt.Sprintf("/home/%s/.kube", user)
    kubeConfig := fmt.Sprintf("%s/config", kubeDir)

    if err := os.MkdirAll(kubeDir, 0755); err != nil {
        return fmt.Errorf("failed to create .kube directory: %v", err)
    }

    cpCmd := exec.Command("sudo", "cp", "/etc/rancher/rke2/rke2.yaml", kubeConfig)
    cpCmd.Stdout = os.Stdout
    cpCmd.Stderr = os.Stderr
    if err := cpCmd.Run(); err != nil {
        return fmt.Errorf("failed to copy kubeconfig: %v", err)
    }

    chownCmd := exec.Command("sudo", "chown", "-R", fmt.Sprintf("%s:%s", user, user), kubeDir)
    chownCmd.Stdout = os.Stdout
    chownCmd.Stderr = os.Stderr
    if err := chownCmd.Run(); err != nil {
        return fmt.Errorf("failed to chown .kube directory: %v", err)
    }

    chmodCmd := exec.Command("sudo", "chmod", "600", kubeConfig)
    chmodCmd.Stdout = os.Stdout
    chmodCmd.Stderr = os.Stderr
    if err := chmodCmd.Run(); err != nil {
        return fmt.Errorf("failed to chmod kubeconfig: %v", err)
    }

    return nil
}



func (OnPrem) Deploy() error {


	    setDefault := func(key, def string) {
        if os.Getenv(key) == "" {
            os.Setenv(key, def)
        }
    }

    setDefault("RELEASE_SERVICE_URL", "registry-rs.edgeorchestration.intel.com")
    setDefault("ORCH_INSTALLER_PROFILE", "onprem")
    setDefault("DEPLOY_VERSION", "v3.1.0")
    setDefault("GITEA_IMAGE_REGISTRY", "docker.io")

    // Variables
    cwd, err := os.Getwd()
    if err != nil {
        return err
    }
    os.Setenv("cwd", cwd)
    os.Setenv("deb_dir_name", "installers")
    os.Setenv("git_arch_name", "repo_archives")
    os.Setenv("argo_cd_ns", "argocd")
    os.Setenv("gitea_ns", "gitea")
    os.Setenv("archives_rs_path", "edge-orch/common/files/orchestrator")
    os.Setenv("si_config_repo", "edge-manageability-framework")
    os.Setenv("installer_rs_path", "edge-orch/common/files")

    tmpDir := filepath.Join(cwd, "repo_archives", "tmp")
    os.Setenv("tmp_dir", tmpDir)

    usr, err := user.Current()
    if err != nil {
        return err
    }
    kubeconfig := filepath.Join("/home", usr.Username, ".kube", "config")
    os.Setenv("KUBECONFIG", kubeconfig)

    os.Setenv("ASSUME_YES", "false")
    os.Setenv("SKIP_DOWNLOAD", "false")
    os.Setenv("ENABLE_TRACE", "false")
    os.Setenv("GIT_REPOS", filepath.Join(cwd, "repo_archives"))



    args := os.Args[1:]
    for i := 0; i < len(args); i++ {
        arg := args[i]
        switch arg {
        case "-h", "--help":
            OnPrem{}.Usage()
            os.Exit(0)
        case "-s", "--sre_tls":
            os.Setenv("SRE_TLS_ENABLED", "true")
            if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
                certPath := args[i+1]
                data, err := os.ReadFile(certPath)
                if err != nil {
                    fmt.Fprintf(os.Stderr, "Failed to read SRE CA cert: %v\n", err)
                    os.Exit(1)
                }
                os.Setenv("SRE_DEST_CA_CERT", string(data))
                i++
            }
        case "--skip-download":
            os.Setenv("SKIP_DOWNLOAD", "true")
        case "-d", "--notls":
            os.Setenv("SMTP_SKIP_VERIFY", "true")
        case "-o", "--override":
            os.Setenv("ORCH_INSTALLER_PROFILE", "onprem-dev")
        case "-u", "--url":
            if i+1 < len(args) {
                os.Setenv("RELEASE_SERVICE_URL", args[i+1])
                i++
            } else {
                fmt.Fprintf(os.Stderr, "ERROR: %s requires an argument\n", arg)
                os.Exit(1)
            }
        case "-t", "--trace":
            os.Setenv("ENABLE_TRACE", "true")
            // No direct equivalent of set -x in Go
        case "-w", "--write-config":
            os.Setenv("WRITE_CONFIG", "true")
        case "-y", "--yes":
            os.Setenv("ASSUME_YES", "true")
        default:
            if strings.HasPrefix(arg, "-") {
                fmt.Fprintf(os.Stderr, "Unknown argument %s\n", arg)
                os.Exit(1)
            } else {
                break
            }
        }
    }



	fmt.Println("Running On Premise Edge Orchestrator installers")

    // Print environment variables
	OnPrem{}.PrintEnvVariables()

    // Check & install script dependencies
	err = OnPrem{}.CheckOras()
    if  err != nil {
        return fmt.Errorf("failed to check oras: %v", err)
    }


    // Download packages
	err = OnPrem{}.DownloadPackages()
    if err != nil {
        return fmt.Errorf("failed to download packages: %v", err)
    }

    // Find repo file
    // cwd := os.Getenv("cwd")
    gitArchName := os.Getenv("git_arch_name")
    siConfigRepo := os.Getenv("si_config_repo")
    pattern := fmt.Sprintf("%s/%s/*%s*.tgz", cwd, gitArchName, siConfigRepo)
    matches, err := filepath.Glob(pattern)
    if err != nil || len(matches) == 0 {
        return fmt.Errorf("repo archive not found with pattern: %s", pattern)
    }
    repoFile := filepath.Base(matches[0])
    os.Setenv("repo_file", repoFile)

    // Write configuration to disk if the flag is set
    if os.Getenv("WRITE_CONFIG") == "true" {
		err = OnPrem{}.WriteConfigToDisk()
        if err != nil {
            return fmt.Errorf("failed to write config to disk: %v", err)
        }
    }

    // Config - interactive
	err = OnPrem{}.AllowConfigInRuntime()
    if  err != nil {
        return fmt.Errorf("failed to allow config in runtime: %v", err)
    }

    // Write out the configs that have explicit overrides
	err = OnPrem{}.WriteConfigsUsingOverrides() 
    if err != nil {
        return fmt.Errorf("failed to write configs using overrides: %v", err)
    }

    // Validate the configuration file, and set missing values
	err = OnPrem{}.ValidateConfig()
    if  err != nil {
        return fmt.Errorf("failed to validate config: %v", err)
    }
	
	
	
    // tmpDir := os.Getenv("tmp_dir")

    // Find the repo file
    pattern = fmt.Sprintf("%s/%s/*%s*.tgz", cwd, gitArchName, siConfigRepo)
    matches, err = filepath.Glob(pattern)
    if err != nil || len(matches) == 0 {
        return fmt.Errorf("repo archive not found with pattern: %s", pattern)
    }
    repoFile = filepath.Base(matches[0])

    // Change to tmpDir
    if err := os.Chdir(tmpDir); err != nil {
        return fmt.Errorf("failed to cd to tmpDir: %v", err)
    }

    // Create tar archive
    tarCmd := exec.Command("tar", "-zcf", repoFile, "./edge-manageability-framework")
    tarCmd.Stdout = os.Stdout
    tarCmd.Stderr = os.Stderr
    if err := tarCmd.Run(); err != nil {
        return fmt.Errorf("failed to create tar archive: %v", err)
    }

    // Move the archive to the destination
    destPath := fmt.Sprintf("%s/%s/%s", cwd, gitArchName, repoFile)
    mvCmd := exec.Command("mv", "-f", repoFile, destPath)
    mvCmd.Stdout = os.Stdout
    mvCmd.Stderr = os.Stderr
    if err := mvCmd.Run(); err != nil {
        return fmt.Errorf("failed to move archive: %v", err)
    }

    // Change back to cwd
    if err := os.Chdir(cwd); err != nil {
        return fmt.Errorf("failed to cd back to cwd: %v", err)
    }

    // Remove tmpDir
    rmCmd := exec.Command("rm", "-rf", tmpDir)
    rmCmd.Stdout = os.Stdout
    rmCmd.Stderr = os.Stderr
    if err := rmCmd.Run(); err != nil {
        return fmt.Errorf("failed to remove tmpDir: %v", err)
    }

    // Run OS Configuration installer and K8s Installer
	err = OnPrem{}.InstallRKE2()
    if err != nil {
        return fmt.Errorf("failed to install RKE2: %v", err)
    }
	
	
	fmt.Println("Installing Gitea & ArgoCD...")

    // cwd := os.Getenv("cwd")
    debDirName := os.Getenv("deb_dir_name")
    installerPattern := fmt.Sprintf("%s/%s/onprem-argocd-installer_*_amd64.deb", cwd, debDirName)
    matches, err = filepath.Glob(installerPattern)
    if err != nil || len(matches) == 0 {
        return fmt.Errorf("no ArgoCD installer package found at %s", installerPattern)
    }
    debFile := matches[0]

    cmd := exec.Command(
        "sudo",
        "NEEDRESTART_MODE=a",
        "DEBIAN_FRONTEND=noninteractive",
        "apt-get", "install", "-y", debFile,
    )
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("failed to install ArgoCD: %v", err)
    }

    // Wait for Gitea namespace creation
    giteaNS := os.Getenv("gitea_ns")
    if giteaNS == "" {
        giteaNS = "gitea"
    }
	err = OnPrem{}.WaitForNamespaceCreation(giteaNS)
    if  err != nil {
        return fmt.Errorf("failed to wait for Gitea namespace: %v", err)
    }

    fmt.Println("sleep 30s to allow Gitea to start")
    time.Sleep(30 * time.Second)
    err = OnPrem{}.WaitForPodsRunning(giteaNS);
    if err != nil {
        return fmt.Errorf("failed to wait for Gitea pods: %v", err)
    }
    fmt.Println("Gitea Installed")

    // Wait for ArgoCD namespace creation
    argoCDNS := os.Getenv("argo_cd_ns")
    if argoCDNS == "" {
        argoCDNS = "argocd"
    }
	err = OnPrem{}.WaitForNamespaceCreation(argoCDNS);
    if  err != nil {
        return fmt.Errorf("failed to wait for ArgoCD namespace: %v", err)
    }




    // Sleep 30 seconds to allow ArgoCD to start
    fmt.Println("Sleeping 30s to allow ArgoCD to start...")
    time.Sleep(30 * time.Second)

    // Wait for ArgoCD pods to be running
    argoCDNamespace := os.Getenv("argo_cd_ns")
    if argoCDNamespace == "" {
        argoCDNamespace = "argocd"
    }
	err = OnPrem{}.WaitForPodsRunning(argoCDNamespace)
    if err != nil {
        return fmt.Errorf("failed to wait for ArgoCD pods: %v", err)
    }
    fmt.Println("ArgoCD installed")

    // Create namespaces for ArgoCD
	err = OnPrem{}.CreateNamespaces()
    if err != nil {
        return fmt.Errorf("failed to create namespaces: %v", err)
    }

    // Create SRE secrets
	err = OnPrem{}.CreateSreSecrets()
    if err != nil {
        return fmt.Errorf("failed to create SRE secrets: %v", err)
    }

    // Create SMTP secrets
	err = OnPrem{}.CreateSmtpSecrets()
    if  err != nil {
        return fmt.Errorf("failed to create SMTP secrets: %v", err)
    }



    // Generate passwords
    harborPassword, err := OnPrem{}.GenerateHarborPassword()
    if err != nil {
        return fmt.Errorf("failed to generate Harbor password: %v", err)
    }
    keycloakPassword, err := OnPrem{}.GeneratePassword()
    if err != nil {
        return fmt.Errorf("failed to generate Keycloak password: %v", err)
    }
    postgresPassword, err := OnPrem{}.GeneratePassword()
    if err != nil {
        return fmt.Errorf("failed to generate Postgres password: %v", err)
    }


    OnPrem{}.CreateHarborSecret("orch-harbor", harborPassword)
	OnPrem{}.CreateHarborPassword("orch-harbor", harborPassword)
    OnPrem{}.CreateKeycloakPassword("orch-platform", keycloakPassword)
    OnPrem{}.CreatePostgresPassword("orch-database", postgresPassword)
    
	OnPrem{}.InstallOrchestrator()

    return nil
}