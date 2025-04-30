// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/bitfield/script"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"gopkg.in/yaml.v3"
)

const (
	kindOrchClusterName      = "kind" // TODO: Keep for backwards compatibility until all Mage is moved to root
	deploymentTimeoutEnv     = "DEPLOYMENT_TIMEOUT"
	defaultDeploymentTimeout = "1200s" // timeout must be a valid string
	argoVersion              = "7.4.4"
	argoRetryCount           = 30
	argoRetryInterval        = 30
	giteaVersion             = "10.6.0"
)

var (
	edgeClusterName      = "demo-cluster"
	defaultClusterDomain = "kind.internal"
)

var argoNamespaces = []string{
	"dev",
	"argocd",
	"gitea",
	"orch-platform", // used when creating a secret for gitea
	"orch-sre",      // used when creating a secret for kindAll
	"orch-harbor",   // used when creating a secret for integration
	"orch-infra",    // used when creating a secret for mailpit
}

// FIXME: Ideally this could be extracted from the cluster configuration and aligned with auth secrets - out of scope for now
var giteaRepos = []string{
	"https://gitea-http.gitea.svc.cluster.local/argocd/edge-manageability-framework",
}

// Public GitHub repositories can be useful for specific development workflows.
var githubRepos = []string{
	"https://github.com/open-edge-platform/edge-manageability-framework",
	"https://github.com/open-edge-platform/orch-utils",
}

var globalAsdf = []string{
	"kubectl",
}

// autoCert is a package-level variable initialized during startup.
var autoCert = func() bool {
	value := os.Getenv("AUTO_CERT")
	return value == "1"
}()

var coderEnv = func() bool {
	value, exists := os.LookupEnv("CODER_WORKSPACE_NAME")
	return exists && value != ""
}()

// serviceDomain is a package-level variable initialized during startup.
var serviceDomain = func() string {
	sd := os.Getenv("E2E_SVC_DOMAIN")
	// retrieve svcdomain from configmap
	// if it does not exist there, then use the defaultservicedomain
	if sd == "" {
		domain, err := LookupOrchestratorDomain()
		if err != nil {
			// Could not locate in the configmap, also attempt to build the domain if this is an autocert (coder) deployment
			if autoCert && coderEnv {
				// retrieve the subdomain name
				subDomain := os.Getenv("ORCH_DOMAIN")
				if subDomain == "" {
					fmt.Printf("error retrieving the orchestrator domain from autocert aws lookup\n")
					return defaultClusterDomain
				}
				return subDomain
			}
			return defaultClusterDomain
		}

		if len(domain) > 0 {
			sd = domain
		} else {
			sd = defaultClusterDomain
		}
	}

	return sd
}()

func updateEdgeName() {
	name, exists := os.LookupEnv("EDGE_CLUSTER_NAME")
	if exists {
		edgeClusterName = name
	}
}

// Install ASDF plugins.
func AsdfPlugins() error {
	// Check if ASDF is installed
	if _, err := exec.LookPath("asdf"); err != nil {
		return fmt.Errorf("asdf is not installed: %w", err)
	}
	// Install remaining tools
	if _, err := script.File(".tool-versions").Column(1).
		MatchRegexp(regexp.MustCompile(`^[^\#]`)).ExecForEach("asdf plugin add {{.}}").Stdout(); err != nil {
		return fmt.Errorf("error running 'asdf plugin add': %w", err)
	}
	if _, err := script.File(".tool-versions").MatchRegexp(regexp.MustCompile(`^[^\#]`)).FilterLine(func(line string) string {
		// Split the line into parts
		parts := strings.Fields(line)
		if len(parts) < 2 {
			fmt.Printf("invalid line format: %s\n", line)
			return line
		}
		tool := parts[0]
		version := parts[1]

		// Run the asdf install command
		cmd := fmt.Sprintf("asdf install %s %s", tool, version)
		if _, err := script.Exec(cmd).Stdout(); err != nil {
			fmt.Printf("error running '%s': %v\n", cmd, err)
		}
		return line
	}).Stdout(); err != nil {
		return fmt.Errorf("error running 'asdf install': %w", err)
	}
	if _, err := script.Exec("asdf current").Stdout(); err != nil {
		return fmt.Errorf("error running 'asdf current': %w", err)
	}
	// Set plugins listed in globalAsdf as global
	for _, name := range globalAsdf {
		if _, err := script.File(".tool-versions").MatchRegexp(regexp.MustCompile(name)).Column(2).
			ExecForEach(fmt.Sprintf("asdf set --home %s {{.}}", name)).Stdout(); err != nil {
			return fmt.Errorf("error seting plugins listed in globalAsdf as global: %w", err)
		}
	}
	fmt.Printf("asdf plugins updated🔌\n")
	return nil
}

// Cleans up the local environment by removing all generated files and directories 🧹
func Clean(ctx context.Context) error {
	for _, path := range []string{
		// Keep list sorted in ascending order for easier maintenance
		"cloudFull_edge-manageability-framework_*.tgz",
		"COMMIT_ID",
		"onpremFull_edge-manageability-framework_*.tgz",
		"edge-manageability-framework",
	} {
		matches, err := filepath.Glob(path)
		if err != nil {
			return fmt.Errorf("failed to glob %s: %w", path, err)
		}

		for _, match := range matches {
			fmt.Printf("Cleaning path: %s\n", match)

			if err := os.RemoveAll(match); err != nil {
				return fmt.Errorf("failed to remove %s: %w", match, err)
			}
		}
	}

	fmt.Println("Cleanup completed successfully 🧹")

	return nil
}

// Namespace contains Undeploy targets.
type Undeploy mg.Namespace

// Undeploy Deletes all local KinD clusters.
func (Undeploy) Kind() error {
	clusters, err := sh.Output("kind", "get", "clusters")
	if err != nil {
		return err
	}

	// No kind clusters found.
	if clusters == "" {
		return nil
	}

	clusterList := strings.Split(clusters, "\n")
	for _, clusterName := range clusterList {
		if err := sh.RunV("kind", "-v", strconv.Itoa(verboseLevel), "delete", "cluster",
			"--name", clusterName); err != nil {
			return err
		}
	}

	return nil
}

// Deletes ENiC and cluster, input required: mage undeploy:edgeCluster <org-name> <project-name>
func (Undeploy) EdgeCluster(orgName, projectName string) error {
	updateEdgeName()

	ctx := context.TODO()
	if err := (TenantUtils{}).GetProject(ctx, orgName, projectName); err != nil {
		return fmt.Errorf("failed to get project %s: %w", projectName, err)
	}

	edgeInfraUser, _, err := getEdgeAndApiUsers(ctx, orgName)
	if err != nil {
		return fmt.Errorf("failed to get edge user: %w", err)
	}

	edgeMgrUser = edgeInfraUser
	project = projectName

	projectId, err := projectId(projectName)
	if err != nil {
		return fmt.Errorf("failed to get project id: %w", err)
	}
	fleetNamespace = projectId

	if err := cleanUpEnic(); err != nil {
		return fmt.Errorf("failed to cleanup enic: %w", err)
	}

	fmt.Println("\nENiC cluster deleted 😊")

	return nil
}

type Deps mg.Namespace

func (Deps) Terraform() error {
	terraformDir := "terraform"

	dirs, err := os.ReadDir(terraformDir)
	if err != nil {
		return fmt.Errorf("failed to read terraform directory: %w", err)
	}

	for _, dir := range dirs {
		if dir.IsDir() {
			if err := sh.RunV(
				"terraform",
				"-chdir="+filepath.Join(terraformDir, dir.Name()),
				"init",
				"--upgrade",
			); err != nil {
				return fmt.Errorf("terraform init failed for directory %s: %w", dir.Name(), err)
			}
		}
	}

	return nil
}

// Checks if running on Ubuntu Linux.
func (Deps) EnsureUbuntu() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("%s OS not supported. Submit a PR? 🤔", runtime.GOOS)
	}

	contents, err := os.ReadFile("/etc/issue")
	if err != nil {
		return fmt.Errorf("read system identification file: %w", err)
	}
	if !bytes.Contains(contents, []byte("Ubuntu")) {
		return fmt.Errorf("ubuntu is the only supported distro at this time. Submit a PR? 🤔")
	}

	fmt.Println("Running on Ubuntu 🐧")

	return nil
}

// Installs FPM (Effing Package Management) for creating OS packages.
func (d Deps) FPM(ctx context.Context) error {
	mg.SerialCtxDeps(
		ctx,
		d.EnsureUbuntu,
	)

	// Check if FPM is already installed and its version is 1.16.0
	if output, err := exec.Command("fpm", "--version").Output(); err == nil {
		if strings.TrimSpace(string(output)) == "1.16.0" {
			fmt.Println("FPM version 1.16.0 is already installed ✅")
			return nil
		}
		fmt.Println("A different version of FPM is installed. Updating to version 1.16.0...")
	}

	// Check if Ruby is already installed
	if _, err := exec.LookPath("ruby"); err == nil {
		fmt.Println("Ruby is already installed ✅")
	} else {
		// Ruby is just needed for this target and is not necessarily needed for general development.
		if err := sh.RunV("sudo", "apt-get", "update", "--assume-yes"); err != nil {
			return fmt.Errorf("failed to update apt-get: %w", err)
		}
		if err := sh.RunV("sudo", "apt-get", "install", "--assume-yes", "ruby-full"); err != nil {
			return fmt.Errorf("failed to install ruby: %w", err)
		}
	}

	// Install or update FPM to version 1.16.0
	return sh.RunV("sudo", "gem", "install", "fpm", "--version", "1.16.0")
}

// Installs libvirt dependencies. This is required for running an Orchestrator and Edge Node using KVM. Only Ubuntu
// 22.04 is supported.
func (d Deps) Libvirt(ctx context.Context) error {
	mg.CtxDeps(ctx, d.EnsureUbuntu)

	return sh.RunV(filepath.Join("tools", "setup-libvirt.bash"))
}

// Destroys the edge network deployed locally.
func (u Undeploy) EdgeNetwork(ctx context.Context) error {
	mg.SerialCtxDeps(
		ctx,
		Deps{}.EnsureUbuntu,
		Deps{}.Terraform,
		u.EdgeNetworkDNS,
	)

	return sh.RunV(
		"terraform",
		"-chdir="+filepath.Join("terraform", "edge-network"),
		"destroy",
		"-var=dns_resolvers=[]",
		"--auto-approve",
	)
}

// Removes the edge network DNS resolver as the default resolver on the host machine.
func (Undeploy) EdgeNetworkDNS(ctx context.Context) error {
	mg.CtxDeps(
		ctx,
		Deps{}.EnsureUbuntu,
	)

	if _, err := os.Stat("/etc/systemd/resolved.conf.bak"); err != nil {
		fmt.Printf("Backup of resolved.conf not found. Was `mage deploy:edgeNetworkDNS` ever run?")
		return nil
	}

	if err := sh.RunV("sudo", "mv", "/etc/systemd/resolved.conf.bak", "/etc/systemd/resolved.conf"); err != nil {
		return fmt.Errorf("failed to restore backup of resolved.conf: %w", err)
	}

	if err := sh.RunV("sudo", "systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %w", err)
	}

	if err := sh.RunV("sudo", "systemctl", "restart", "systemd-resolved"); err != nil {
		return fmt.Errorf("failed to restart systemd-resolved: %w", err)
	}

	fmt.Println("Edge network DNS integration removed successfully 🧑‍🔧")

	return nil
}

// Destroys the edge storage pool deployed locally.
func (Undeploy) EdgeStoragePool(ctx context.Context) error {
	mg.SerialCtxDeps(
		ctx,
		Deps{}.EnsureUbuntu,
		Deps{}.Terraform,
	)

	return sh.RunV(
		"terraform",
		"-chdir="+filepath.Join("terraform", "edge-storage-pool"),
		"destroy",
		"--auto-approve",
	)
}

// Destroys the on-premise Orchestrator, edge network, and edge storage pool deployed locally.
func (u Undeploy) OnPrem(ctx context.Context) error {
	mg.SerialCtxDeps(
		ctx,
		Deps{}.EnsureUbuntu,
		Deps{}.Terraform,
	)

	// Check if TF_VAR_FILE is defined, if not, set it to a default value
	if os.Getenv("TF_VAR_FILE") == "" {
		os.Setenv("TF_VAR_FILE", "terraform.tfvars")
	}

	tfvarsFile := os.Getenv("TF_VAR_FILE")

	if err := sh.RunV(
		"terraform",
		"-chdir="+filepath.Join("terraform", "orchestrator"),
		"destroy",
		"--var-file="+tfvarsFile,
		fmt.Sprintf("--parallelism=%d", runtime.NumCPU()), // Set parallelism to the number of CPUs on the machine
		"--auto-approve",
	); err != nil {
		return fmt.Errorf("terraform destroy failed: %w", err)
	}

	// HACK: Sometimes the destroy command fails to remove the VM, so we need to undefine it manually
	if err := sh.RunV("sudo", "virsh", "undefine", "orch-tf"); err != nil {
		fmt.Printf("virsh undefine failed: %v\n", err)
	}

	mg.CtxDeps(
		ctx,
		u.EdgeNetwork,
		u.EdgeStoragePool,
	)

	fmt.Println("Orchestrator deployment destroyed 🗑️")

	return nil
}

// Destroys any local Virtual Edge Nodes.
func (Undeploy) VEN(ctx context.Context) error {
	mg.CtxDeps(
		ctx,
		Deps{}.EnsureUbuntu,
	)

	tempDir, err := os.MkdirTemp("", "ven-clone")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	fmt.Printf("Temporary directory created: %s\n", tempDir)

	if err := os.Chdir(tempDir); err != nil {
		return fmt.Errorf("failed to change directory to temporary directory: %w", err)
	}

	if err := sh.RunV("git", "clone", "https://github.com/open-edge-platform/virtual-edge-node", "ven"); err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	if err := os.Chdir("ven"); err != nil {
		return fmt.Errorf("failed to change directory to 'ven': %w", err)
	}

	if err := sh.RunV("git", "checkout", "vm-provisioning/1.0.7"); err != nil {
		return fmt.Errorf("failed to checkout specific commit: %w", err)
	}

	if err := os.Setenv("LIBVIRT_DEFAULT_URI", "qemu:///system"); err != nil {
		return fmt.Errorf("failed to set LIBVIRT_DEFAULT_URI: %w", err)
	}

	if err := os.Chdir("vm-provisioning"); err != nil {
		return fmt.Errorf("failed to change directory to 'ven': %w", err)
	}

	if err := sh.RunV("sudo", filepath.Join("scripts", "destroy_vm.sh")); err != nil {
		return fmt.Errorf("failed to destroy virtual machine: %w", err)
	}

	return nil
}

type Deploy mg.Namespace

// TBD: Replace with default and custom driven by external config / config UI

// Deploy kind cluster, Argo CD, and all Orchestrator services.
func (d Deploy) KindAll() error {
	return d.all("dev")
}

// Deploy kind cluster, Argo CD, and all Orchestrator services except o11y and kyverno.
func (d Deploy) KindMinimal() error {
	return d.all("dev-minimal")
}

// Deploy kind cluster, Argo CD, and Orchestrator services with customized settings.
func (d Deploy) KindCustom() error {
	targetEnv, err := Config{}.createCluster()
	if err != nil {
		return fmt.Errorf("failed to create cluster: %w", err)
	}

	return d.all(targetEnv)
}

// Deploy kind cluster, Argo CD, and Orchestrator services with preset settings.
func (d Deploy) KindPreset(clusterPreset string) error {
	targetEnv, err := Config{}.usePreset(clusterPreset)
	if err != nil {
		return fmt.Errorf("failed to apply cluster preset: %w", err)
	}

	return d.all(targetEnv)
}

// Deploy kind cluster and Argo CD.
func (d Deploy) Kind(targetEnv string) error {
	return d.kind(targetEnv)
}

func (d Deploy) Gitea(targetEnv string) error {
	err := (Config{}).renderTargetConfigTemplate(targetEnv, "orch-configs/templates/bootstrap/gitea.tpl", ".deploy/bootstrap/gitea.yaml")
	if err != nil {
		return fmt.Errorf("failed to render gitea configuration: %w", err)
	}

	giteaBootstrapValues := []string{".deploy/bootstrap/gitea.yaml"}
	return d.gitea(giteaBootstrapValues, targetEnv)
}

func (d Deploy) StartGiteaProxy() error {
	err := d.StopGiteaProxy()
	if err != nil {
		return fmt.Errorf("failed to stop Gitea proxy: %w", err)
	}

	portForwardCmd, err := d.startGiteaPortForward()
	if err != nil {
		return fmt.Errorf("failed to start Gitea port forwarding: %w", err)
	}

	// Save the PID to a .gitea-proxy file
	pid := portForwardCmd.Process.Pid
	pidFile := ".gitea-proxy"
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0o644); err != nil {
		return fmt.Errorf("failed to write PID to %s: %w", pidFile, err)
	}
	fmt.Printf("Gitea proxy PID saved to %s\n", pidFile)
	return nil
}

func (d Deploy) StopAllKubectlProxies() error {
	// List all kubectl port-forward processes
	cmd := exec.Command("pgrep", "-af", "kubectl port-forward")
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("failed to list kubectl port-forward processes: %v\n", err)
		return nil
	}
	fmt.Println("kubectl port-forward processes:")
	fmt.Println(string(output))

	// Stop all kubectl port-forward processes
	cmd = exec.Command("pkill", "-f", "kubectl port-forward")
	if err := cmd.Run(); err != nil {
		fmt.Printf("failed to stop kubectl port-forward processes: %v", err)
	}

	cmd = exec.Command("pgrep", "-af", "kubectl port-forward")
	output, err = cmd.Output()
	if err != nil {
		fmt.Printf("failed to list kubectl port-forward processes: %v\n", err)
		return nil
	}
	fmt.Println("Any remaining kubectl port-forward processes after pkill:")
	fmt.Println(string(output))
	return nil
}

func (d Deploy) StopGiteaProxy() error {
	pidFile := ".gitea-proxy"

	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		fmt.Println("Gitea proxy PID file not found, nothing to stop")
		return nil
	}
	defer func() {
		if err := os.Remove(pidFile); err != nil {
			fmt.Printf("failed to remove PID file %s: %v\n", pidFile, err)
		}
	}()

	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		fmt.Printf("failed to read PID file %s\n", pidFile)
		return nil
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		fmt.Printf("failed to parse PID from %s\n", pidFile)
		return nil
	}

	portForwardCmd := exec.Command("kill", "-9", strconv.Itoa(pid))
	if err := portForwardCmd.Run(); err != nil {
		fmt.Printf("failed to stop Gitea port forwarding process: %d", pid)
	} else {
		fmt.Printf("Gitea proxy with PID %d stopped\n", pid)
	}

	return nil
}

// Deploy Argo CD in kind cluster.
func (d Deploy) Argocd(targetEnv string) error {
	err := (Config{}).renderTargetConfigTemplate(targetEnv, "orch-configs/templates/bootstrap/argocd.tpl", ".deploy/bootstrap/argocd.yaml")
	if err != nil {
		return fmt.Errorf("failed to render argocd proxy configuration: %w", err)
	}

	// Set argoBootstrapValues to the default set for a kind dev environment
	argoBootstrapValues := []string{
		".deploy/bootstrap/argocd.yaml",
	}

	return d.argocd(argoBootstrapValues, targetEnv)
}

// Deploy the edge network locally.
func (d Deploy) EdgeNetwork(ctx context.Context) error {
	mg.CtxDeps(
		ctx,
		Deps{}.EnsureUbuntu,
		Deps{}.Terraform,
	)

	dnsResolversBytes, err := exec.Command("resolvectl", "dns").Output()
	if err != nil {
		return fmt.Errorf("failed to get DNS resolvers: %w", err)
	}
	dnsResolversOutput := strings.TrimSpace(string(dnsResolversBytes))

	// Extract IP addresses from the output. There can be multiple lines, so we split by newline. Each line can have
	// multiple IP addresses, so we split by space and parse to determine if it's a valid IP address. Finally, we store
	// the valid IP addresses in a set to ensure uniqueness.
	dnsResolvers := map[string]struct{}{}

	for _, line := range strings.Split(dnsResolversOutput, "\n") {
		for _, field := range strings.Fields(line) {
			if net.ParseIP(field) != nil {
				dnsResolvers[field] = struct{}{}
			}
		}
	}

	// Convert the set to a slice of strings that are comma separated and wrapped in double quotes
	var resolvers []string
	for ip := range dnsResolvers {
		resolvers = append(resolvers, fmt.Sprintf("\"%s\"", ip))
	}
	resolversStr := strings.Join(resolvers, ",")

	fmt.Printf("Using DNS resolvers: %s\n", resolversStr)

	// Pass parent context to the command to allow for cancellation
	cmd := exec.CommandContext(
		ctx,
		"terraform",
		"-chdir="+filepath.Join("terraform", "edge-network"),
		"apply",
		fmt.Sprintf("-var=dns_resolvers=[%s]", resolversStr),
		fmt.Sprintf("--parallelism=%d", runtime.NumCPU()), // Set parallelism to the number of CPUs on the machine
		"--auto-approve")

	// Stream the output to stdout and stderr
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("terraform apply failed: %w", err)
	}

	return nil
}

// Sets the edge network DNS resolver as the default resolver on the host machine.
func (d Deploy) EdgeNetworkDNS(ctx context.Context) error {
	mg.SerialCtxDeps(
		ctx,
		Deps{}.EnsureUbuntu,
		Undeploy{}.EdgeNetworkDNS,
	)

	output, err := exec.CommandContext(
		ctx,
		"terraform",
		"-chdir="+filepath.Join("terraform", "edge-network"),
		"output",
		"-json",
	).Output()
	if err != nil {
		return fmt.Errorf("failed to get terraform output: %w", err)
	}

	var terraformOutput struct {
		NetworkSubnetCIDRs struct {
			Value []string `json:"value"`
		} `json:"network_subnet_cidrs"`
	}
	if err := json.Unmarshal(output, &terraformOutput); err != nil {
		return fmt.Errorf("failed to unmarshal terraform output: %w", err)
	}
	if len(terraformOutput.NetworkSubnetCIDRs.Value) == 0 {
		return fmt.Errorf("no network subnet CIDRs found in terraform output")
	}

	ip, _, err := net.ParseCIDR(terraformOutput.NetworkSubnetCIDRs.Value[0])
	if err != nil {
		return fmt.Errorf("failed to parse CIDR: %w", err)
	}

	// The bridge IP is the first address in the subnet e.g., if the subnet is 192.168.99.0/24, the bridge IP is
	// 192.168.99.1
	ipv4 := ip.To4()
	if ipv4 == nil {
		return fmt.Errorf("IP address is not IPv4: %s", ip)
	}
	ipv4[3] = 1
	bridgeIP := ipv4

	fmt.Printf("Using bridge IP: %s\n", bridgeIP.String())

	// Check if backup of resolved.conf already exists
	if _, err := os.Stat("/etc/systemd/resolved.conf.bak"); err == nil {
		return fmt.Errorf("backup of resolved.conf already exists. Did you forget to run `mage undeploy:edgeNetwork`?")
	}

	// Create a backup of the existing resolved.conf file
	if err := sh.RunV("sudo", "cp", "/etc/systemd/resolved.conf", "/etc/systemd/resolved.conf.bak"); err != nil {
		return fmt.Errorf("failed to create backup of resolved.conf: %w", err)
	}

	contents := fmt.Sprintf(`[Resolve]
DNS=%s
DNSStubListener=yes
`, bridgeIP.String())

	cmd := exec.Command("sudo", "tee", "/etc/systemd/resolved.conf")
	cmd.Stdin = strings.NewReader(contents)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to write edge-network-resolved.sh: %w", err)
	}

	if err := sh.RunV("sudo", "systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %w", err)
	}

	if err := sh.RunV("sudo", "systemctl", "restart", "systemd-resolved"); err != nil {
		return fmt.Errorf("failed to restart systemd-resolved: %w", err)
	}

	fmt.Println("Edge network DNS successfully set as default DNS resolver 🧑‍🔧")

	return nil
}

func (Deploy) EdgeStoragePool(ctx context.Context) error {
	mg.CtxDeps(
		ctx,
		Deps{}.EnsureUbuntu,
		Deps{}.Terraform,
	)

	// Pass parent context to the command to allow for cancellation
	cmd := exec.CommandContext(
		ctx,
		"terraform",
		"-chdir="+filepath.Join("terraform", "edge-storage-pool"),
		"apply",
		fmt.Sprintf("--parallelism=%d", runtime.NumCPU()), // Set parallelism to the number of CPUs on the machine
		"--auto-approve")

	// Stream the output to stdout and stderr
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("terraform apply failed: %w", err)
	}

	return nil
}

// Deploy orchestrator locally using edge-manageability-framework revision defined by local git HEAD.
func (d Deploy) OnPrem(ctx context.Context) error {
	// Ensure the required dependencies are installed first
	mg.SerialCtxDeps(
		ctx,
		Deps{}.EnsureUbuntu,
		Deps{}.Libvirt,
		Deps{}.Terraform,
		Undeploy{}.OnPrem,
	)

	// Create the underlying network and storage pool
	mg.CtxDeps(
		ctx,
		d.EdgeNetwork,
		d.EdgeStoragePool,
	)

	// TODO: Build DEB packages so they can be copied into the Orchestrator

	// Check if TF_VAR_FILE is defined, if not, set it to a default value
	if os.Getenv("TF_VAR_FILE") == "" {
		os.Setenv("TF_VAR_FILE", "terraform.tfvars")
	}

	tfvarsFile := os.Getenv("TF_VAR_FILE")

	// Pass parent context to the command to allow for cancellation
	cmd := exec.CommandContext(
		ctx,
		"terraform",
		"-chdir="+filepath.Join("terraform", "orchestrator"),
		"apply",
		"--var-file="+tfvarsFile,
		fmt.Sprintf("--parallelism=%d", runtime.NumCPU()), // Set parallelism to the number of CPUs on the machine
		"--auto-approve")

	// Stream the output to stdout and stderr
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("terraform apply failed: %w", err)
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	kubeconfigPath := filepath.Join(dir, "terraform", "orchestrator", "files", "kubeconfig")

	fmt.Printf("Orchestrator deployment started 🚀\n")

	fmt.Printf(`
This generally takes ~35 minutes to complete. You can check the status of the deployment by running:

export KUBECONFIG=%s
mage deploy:waitUntilComplete

Once the deployment is complete, you can access the K8s cluster with kubectl:

kubectl get pods -A

In order to access the Orchestrator, you will need to install the Orchestrator TLS certificate into your local machine's
trust store and configure DNS to resolve the orchestrator domain to the kind cluster IP. Execute:

mage deploy:OrchCA deploy:EdgeNetworkDNS

Congrats! You should now be able to access the Orchestrator on this host at https://web-ui.cluster.onprem 🎉
`, kubeconfigPath)

	return nil
}

// OnboardingFlow defines the onboarding flow type for the edge node.
type OnboardingFlow string

var (
	InteractiveOnboardingFlow    OnboardingFlow = "io"
	NonInteractiveOnboardingFlow OnboardingFlow = "nio"
)

func (flow OnboardingFlow) IsValid() bool {
	return flow == InteractiveOnboardingFlow || flow == NonInteractiveOnboardingFlow
}

// Deploy a local Virtual Edge Node using libvirt. An Orchestrator must be running locally.
func (d Deploy) VEN(ctx context.Context, flow string) error {
	serialNumber, err := d.VENWithFlow(ctx, flow)
	if err != nil {
		return fmt.Errorf("failed to deploy virtual Edge Node: %w", err)
	}

	fmt.Printf("Successfully deployed virtual Edge Node with serial number: %s\n", serialNumber)

	return nil
}

// VENWithFlow deploys a local Virtual Edge Node using libvirt and returns the serial number of the deployed node.
func (d Deploy) VENWithFlow(ctx context.Context, flow string) (string, error) { //nolint:gocyclo,maintidx
	mg.CtxDeps(
		ctx,
		Deps{}.EnsureUbuntu,
	)

	if !OnboardingFlow(flow).IsValid() {
		return "", fmt.Errorf("invalid onboarding flow: %s", flow)
	}

	if serviceDomain == "" {
		return "", fmt.Errorf("cluster service domain name is not set")
	}

	fmt.Printf("Using Orchestrator domain: %s\n", serviceDomain)

	password, err := GetDefaultOrchPassword()
	if err != nil {
		return "", fmt.Errorf("failed to get default Orchestrator password: %w", err)
	}

	tempDir, err := os.MkdirTemp("", "ven-clone")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	fmt.Printf("Temporary directory created: %s\n", tempDir)

	if err := os.Chdir(tempDir); err != nil {
		return "", fmt.Errorf("failed to change directory to temporary directory: %w", err)
	}

	if err := sh.RunV("git", "clone", "https://github.com/open-edge-platform/virtual-edge-node", "ven"); err != nil {
		return "", fmt.Errorf("failed to clone repository: %w", err)
	}

	if err := os.Chdir("ven"); err != nil {
		return "", fmt.Errorf("failed to change directory to 'ven': %w", err)
	}

	if err := sh.RunV("git", "checkout", "vm-provisioning/1.0.7"); err != nil {
		return "", fmt.Errorf("failed to checkout specific commit: %w", err)
	}

	if err := os.Chdir("vm-provisioning"); err != nil {
		return "", fmt.Errorf("failed to change directory to 'vm-provisioning': %w", err)
	}

	if err := sh.RunV("sudo", "mkdir", "-p", "/etc/apparmor.d/disable/"); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	for _, link := range []string{
		"/etc/apparmor.d/usr.sbin.libvirtd",
		"/etc/apparmor.d/usr.lib.libvirt.virt-aa-helper",
	} {
		if err := sh.RunV("sudo", "ln", "-sf", link, "/etc/apparmor.d/disable/"); err != nil {
			return "", fmt.Errorf("failed to create symlink: %w", err)
		}
	}

	if err := sh.RunV("sudo", "apparmor_parser", "-R", "/etc/apparmor.d/usr.sbin.libvirtd"); err != nil {
		fmt.Printf("failed to remove apparmor profile: %v\n", err)
	}

	if err := sh.RunV("sudo", "apparmor_parser", "-R", "/etc/apparmor.d/usr.lib.libvirt.virt-aa-helper"); err != nil {
		fmt.Printf("failed to remove apparmor profile: %v\n", err)
	}

	if err := sh.RunV("sudo", "systemctl", "restart", "libvirtd"); err != nil {
		return "", fmt.Errorf("failed to restart libvirtd: %w", err)
	}

	if err := sh.RunV("sudo", "systemctl", "reload", "apparmor"); err != nil {
		return "", fmt.Errorf("failed to reload apparmor: %w", err)
	}

	tmpl, err := template.New("config").Parse(`
CLUSTER='{{.ServiceDomain}}'

# IO Flow Configurations
ONBOARDING_USERNAME='{{.OnboardingUsername}}'
ONBOARDING_PASSWORD='{{.OnboardingPassword}}'
# NIO Flow Configurations
PROJECT_NAME='{{.ProjectName}}'
PROJECT_API_USER='{{.ProjectApiUser}}'
PROJECT_API_PASSWORD='{{.ProjectApiPassword}}'

# VM Resources
RAM_SIZE='{{.RamSize}}'
NO_OF_CPUS='{{.NoOfCpus}}'
SDA_DISK_SIZE='{{.SdaDiskSize}}'
LIBVIRT_DRIVER='{{.LibvirtDriver}}'

USERNAME_LINUX='{{.UsernameLinux}}'
PASSWORD_LINUX='{{.PasswordLinux}}'
CI_CONFIG='{{.CiConfig}}'

# Optional: Advance Settings
BRIDGE_NAME='{{.BridgeName}}'
INTF_NAME='{{.IntfName}}'
VM_NAME='{{.VmName}}'
POOL_NAME='{{.PoolName}}'
STANDALONE=0
`)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	data := struct {
		ServiceDomain      string
		OnboardingUsername string
		OnboardingPassword string
		ProjectName        string
		ProjectApiUser     string
		ProjectApiPassword string
		RamSize            string
		NoOfCpus           string
		SdaDiskSize        string
		LibvirtDriver      string
		UsernameLinux      string
		PasswordLinux      string
		CiConfig           string
		BridgeName         string
		IntfName           string
		VmName             string
		PoolName           string
	}{
		ServiceDomain:      serviceDomain,
		OnboardingUsername: "sample-project-onboarding-user",
		OnboardingPassword: password,
		ProjectName:        "sample-project",
		ProjectApiUser:     "sample-project-api-user",
		ProjectApiPassword: password,
		RamSize:            "8192",
		NoOfCpus:           "4",
		SdaDiskSize:        "110G",
		LibvirtDriver:      "kvm",
		UsernameLinux:      "user",
		PasswordLinux:      "user",
		CiConfig:           "true",
		BridgeName:         "edge",
		IntfName:           "virbr1",
		VmName:             "",
		PoolName:           "edge",
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	if err := os.WriteFile("config", buf.Bytes(), os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to write config file: %w", err)
	}

	for i := 0; i < 60; i++ {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://tinkerbell-nginx.%s/tink-stack/keys/Full_server.crt", serviceDomain), nil)
		if err != nil {
			fmt.Printf("Failed to create request: %v\n", err)
			time.Sleep(10 * time.Second)
			continue
		}

		client := &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}

		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Request failed: %v\n", err)
			time.Sleep(10 * time.Second)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			out, err := os.Create(filepath.Join("certs", "Full_server.crt"))
			if err != nil {
				fmt.Printf("Failed to create file: %v\n", err)
				return "", fmt.Errorf("failed to create file: %w", err)
			}
			defer out.Close()

			_, err = io.Copy(out, resp.Body)
			if err != nil {
				fmt.Printf("Failed to write to file: %v\n", err)
				return "", fmt.Errorf("failed to write to file: %w", err)
			}

			fmt.Println("Successfully retrieved and saved the certificate.")
			break
		} else {
			fmt.Printf("Unexpected status code, will retry: %d\n", resp.StatusCode)
			time.Sleep(10 * time.Second)
			continue
		}
	}
	var outputChmodBuf bytes.Buffer
	chmodCmd := exec.CommandContext(ctx, "sudo", "chmod", "755",
		filepath.Join("scripts", "update_provider_defaultos.sh"),
		filepath.Join("scripts", "create_vm.sh"),
		filepath.Join("scripts", "host_status_check.sh"),
		filepath.Join("scripts", "nio_configs.sh"),
		filepath.Join("scripts", "destroy_vm.sh"),
	)

	chmodCmd.Stdout = io.MultiWriter(os.Stdout, &outputChmodBuf)
	chmodCmd.Stderr = io.MultiWriter(os.Stderr, &outputChmodBuf)

	if err := chmodCmd.Run(); err != nil {
		return "", fmt.Errorf("failed to chmod: %w", err)
	}

	if err := sh.RunV(filepath.Join("scripts", "update_provider_defaultos.sh"), "microvisor"); err != nil {
		return "", fmt.Errorf("failed to update provider default OS: %w", err)
	}

	var outputBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, "sudo", filepath.Join("scripts", "create_vm.sh"), "1", fmt.Sprintf("-%s", flow))
	cmd.Stdout = io.MultiWriter(os.Stdout, &outputBuf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &outputBuf)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to create virtual machine: %w", err)
	}

	matches, err := searchForSubstring(outputBuf.Bytes(), "serial=")
	if err != nil {
		return "", fmt.Errorf("failed to search file for serial number: %w", err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("failed to find serial number in ci-console.log")
	}

	// Remove trailing commas or spaces
	serialNumber := strings.Split(matches[0], "serial=")[1]
	serialNumber = strings.TrimSpace(strings.ReplaceAll(serialNumber, ",", ""))

	// Search for "Secure Boot Status MATCH" in the output
	matches_sb, err := searchForSubstring(outputBuf.Bytes(), "Secure Boot Status MATCH")
	if err != nil {
		return "", fmt.Errorf("failed to search file for Secure Boot Status MATCH: %w", err)
	}
	if len(matches_sb) == 0 {
		return "", fmt.Errorf("failed to find Secure Boot Status MATCH check in ci-console.log")
	}

	// Add the new command to execute host_statue with the serial number
	hostStatueCmd := exec.CommandContext(ctx, filepath.Join("scripts", "host_status_check.sh"), serialNumber)
	hostStatueCmd.Stdout = os.Stdout
	hostStatueCmd.Stderr = os.Stderr

	if err := hostStatueCmd.Run(); err != nil {
		return "", fmt.Errorf("failed to execute host_statue command: %w", err)
	}
	return serialNumber, nil
}

func searchForSubstring(contents []byte, substring string) ([]string, error) {
	var matchingLines []string
	for _, line := range bytes.Split(contents, []byte{'\n'}) {
		if bytes.Contains(line, []byte(substring)) {
			matchingLines = append(matchingLines, string(line))
		}
	}
	return matchingLines, nil
}

// Deploy the Orchestrator using edge-manageability-framework revision defined by the targetEnv config.
// Should be used for environments based on merged edge-manageability-framework version, e.g. integration, validation, demo, prod.
func (d Deploy) Orch(targetEnv string) error {
	return d.orch(targetEnv)
}

// Deploy Orchestrator using edge-manageability-framework revision defined by local git HEAD.
// Should be used for environments based on local edge-manageability-framework version, e.g. dev, coder, fast-pipeline.
func (d Deploy) OrchLocal(targetEnv string) error {
	return d.orchLocal(targetEnv)
}

func (d Deploy) OrchCA() error {
	return d.orchCA()
}

// Deploys ENiC Edge cluster with sample-project project, input required: mage deploy:edgeCluster <targetEnv>
func (d Deploy) EdgeCluster(targetEnv string) error {
	updateEdgeName()

	projectName := "sample-project"
	orgName := "sample-org"

	ctx := context.TODO()
	if err := (TenantUtils{}).GetProject(ctx, orgName, projectName); err != nil {
		return fmt.Errorf("failed to get project %s: %w", projectName, err)
	}

	_, apiUser, err := getEdgeAndApiUsers(ctx, orgName)
	if err != nil {
		return fmt.Errorf("failed to get api user: %w", err)
	}

	os.Setenv("ORCH_PROJECT", projectName)
	os.Setenv("ORCH_ORG", orgName)
	os.Setenv("ORCH_USER", apiUser)

	projectId, err := projectId(projectName)
	if err != nil {
		return fmt.Errorf("failed to get project id: %w", err)
	}

	fleetNamespace = projectId

	labels := []string{
		"color=blue",
	}
	return d.deployEnicCluster(targetEnv, strings.Join(labels, ","))
}

// Deploys ENiC Edge cluster, input required: mage deploy:edgeClusterWithProject <targetEnv> <org-name> <project-name>
func (d Deploy) EdgeClusterWithProject(targetEnv string, orgName string, projectName string) error {
	updateEdgeName()

	ctx := context.TODO()
	if err := (TenantUtils{}).GetProject(ctx, orgName, projectName); err != nil {
		return fmt.Errorf("failed to get project %s: %w", projectName, err)
	}

	edgeInfraUser, apiUser, err := getEdgeAndApiUsers(ctx, orgName)
	if err != nil {
		return fmt.Errorf("failed to get edge and api users: %w", err)
	}

	edgeMgrUser = edgeInfraUser
	project = projectName

	os.Setenv("ORCH_PROJECT", projectName)
	os.Setenv("ORCH_ORG", orgName)
	os.Setenv("ORCH_USER", apiUser)

	projectId, err := projectId(projectName)
	if err != nil {
		return fmt.Errorf("failed to get project id: %w", err)
	}

	fleetNamespace = projectId

	labels := []string{
		"color=blue",
	}
	return d.deployEnicCluster(targetEnv, strings.Join(labels, ","))
}

// Deploys ENiC Edge cluster with sample-project project: mage deploy:edgeClusterWithLabels <targetEnv> <labels, color=blue,city=hillsboro>
func (d Deploy) EdgeClusterWithLabels(targetEnv string, labels string) error {
	updateEdgeName()
	projectName := "sample-project"
	orgName := "sample-org"

	ctx := context.TODO()
	if err := (TenantUtils{}).GetProject(ctx, orgName, projectName); err != nil {
		return fmt.Errorf("failed to get project %s: %w", projectName, err)
	}

	_, apiUser, err := getEdgeAndApiUsers(ctx, orgName)
	if err != nil {
		return fmt.Errorf("failed to get api user: %w", err)
	}

	os.Setenv("ORCH_PROJECT", projectName)
	os.Setenv("ORCH_ORG", orgName)
	os.Setenv("ORCH_USER", apiUser)

	projectId, err := projectId(projectName)
	if err != nil {
		return fmt.Errorf("failed to get project id: %w", err)
	}

	fleetNamespace = projectId

	return d.deployEnicCluster(targetEnv, labels)
}

func (d Deploy) AddKyvernoPolicy() error {
	return d.addKyvernoPolicy()
}

// Deploying VictoriaMetrics, <usage with argument "apply" when deploying, "delete" when deleting>
func (d Deploy) VictoriaMetrics(cmd string) error {
	return d.victoriaMetrics(cmd)
}

type Argo mg.Namespace

// Gets initial Argo login credential.
func (a Argo) InitSecret() error {
	return a.initSecret()
}

// Login ArgoCD CLI.
func (a Argo) Login() error {
	return a.login()
}

// Add public GitHub Orchestrator platform source repos to ArgoCD.
func (a Argo) AddGithubRepos() error {
	gitUser := os.Getenv("GIT_USER")
	if gitUser == "" {
		return fmt.Errorf("must set environment variable GIT_USER")
	}

	gitToken := os.Getenv("GIT_TOKEN")
	if gitToken == "" {
		return fmt.Errorf("must set environment variable GIT_TOKEN")
	}

	err := a.login()
	if err != nil {
		return fmt.Errorf("failed to login to ArgoCD: %w", err)
	}

	return a.repoAdd(gitUser, gitToken, githubRepos)
}

// Add repositories to ArgoCD from .mage-local.yaml configuration.
func (a Argo) AddLocalRepos() error {
	_, err := os.Stat(".mage-local.yaml")
	if err != nil {
		fmt.Println("No .mage-local.yaml found, using default repositories")
		return nil
	}

	fmt.Println("Local repositories file .mage-local.yaml found, adding repositories to ArgoCD")
	yamlFile, err := os.ReadFile(".mage-local.yaml")
	if err != nil {
		return fmt.Errorf("error reading .mage-local.yaml: %w", err)
	}

	var localMageSettings struct {
		EnableGithubRepos bool `yaml:"enableGithubRepos"`

		LocalRepos []struct {
			Url   string `yaml:"url"`
			User  string `yaml:"user"`
			Token string `yaml:"token"`
		} `yaml:"localRepos"`
	}

	if err := yaml.Unmarshal(yamlFile, &localMageSettings); err != nil {
		return fmt.Errorf("error parsing .mage-local.yaml: %w", err)
	}

	if localMageSettings.EnableGithubRepos {
		fmt.Println("Adding Github repositories")
		if err := a.AddGithubRepos(); err != nil {
			return fmt.Errorf("failed to add Github repositories: %w", err)
		}
	} else {
		// If Github repositories are not enabled, login to ArgoCD here to avoid duplicate login
		err := a.login()
		if err == nil {
			return fmt.Errorf("failed to login to ArgoCD: %w", err)
		}
	}

	// Add any specified local repositories to ArgoCD
	if len(localMageSettings.LocalRepos) > 0 {
		fmt.Println("Adding local repositories")
		for _, repo := range localMageSettings.LocalRepos {
			if repo.Url == "" {
				continue
			}
			// If the user value is empty, set it to the default value
			if repo.User == "" {
				repo.User = "$GIT_USER"
			}
			// If the token value is empty, set it to the default value
			if repo.Token == "" {
				repo.Token = "$GIT_TOKEN"
			}
			// If the user value starts with a '$' sign, replace it with the environment value
			if strings.HasPrefix(repo.User, "$") {
				envVar := strings.TrimPrefix(repo.User, "$")
				repo.User = os.Getenv(envVar)
				if repo.User == "" {
					return fmt.Errorf("user %s required by %s repo is not set", envVar, repo.Url)
				}
			}
			// If the token value starts with a '$' sign, replace it with the environment value
			if strings.HasPrefix(repo.Token, "$") {
				envVar := strings.TrimPrefix(repo.Token, "$")
				repo.Token = os.Getenv(envVar)
				if repo.Token == "" {
					return fmt.Errorf("token %s required by %s repo is not set", envVar, repo.Url)
				}
			}

			fmt.Printf("Adding local repository %s\n", repo.Url)
			repoUrlList := []string{repo.Url}
			err := a.repoAdd(repo.User, repo.Token, repoUrlList)
			if err != nil {
				return fmt.Errorf("failed to add local repository %s: %w", repo.Url, err)
			}
		}
	}

	return nil
}

// Lists all ArgoCD Applications, sorted by syncWave.
func (a Argo) AppSeq() error {
	return a.appSeq()
}

type Router mg.Namespace

// Restarts router to pass through external TLS connections to Argo service in k8s.
func (r Router) Restart() error {
	if err := r.stop(); err != nil {
		return err
	}
	return r.start("", "", "")
}

// Starts router to pass through external TLS connections to Argo service in k8s.
func (r Router) Start() error {
	return r.start("", "", "")
}

// StartSandbox Start router for external domain to kind.internal for orchestrator-sandbox.
// 3 arguments are required:
// * externalDomain - the name of the external domain e.g. sandbox-1.orchestrator-sandbox.one-edge.intel.com.
// * sandboxKeyFile An "rsa" Private Key (unencrypted)
// * sandboxCertFile An x509 Certificate that matches the Private Key and the external Domain and has not expired.
func (r Router) StartSandbox(externalDomain string, sandboxKeyFile string, sandboxCertFile string) error {
	return r.start(externalDomain, sandboxKeyFile, sandboxCertFile)
}

// Stops router for external connections to Argo service in k8s.
func (r Router) Stop() error {
	return r.stop()
}

type Lint mg.Namespace

// Lint everything.
func (l Lint) All() error {
	if err := l.golang(); err != nil {
		return err
	}
	if err := l.helm(); err != nil {
		return err
	}
	if err := l.yaml(); err != nil {
		return err
	}
	return nil
}

// Lint golang files.
func (l Lint) Golang() error {
	return l.golang()
}

// Lint helm templates.
func (l Lint) Helm() error {
	return l.helm()
}

// Lint helm values.
func (l Lint) Yaml() error {
	return l.yaml()
}

type Database mg.Namespace

// Retrieves the admin password for local postgres database.
func (d Database) GetPassword() error {
	pass, err := d.getPassword()
	fmt.Println(pass)
	return err
}

// Starts an interactive psql client and connects to local postgres database.
func (d Database) PSQL() error {
	return d.psql()
}

// Namespace contains Vault targets.
type Vault mg.Namespace

// Retrieves vault keys and root token after deployment and writes vault keys to a local file.
func (v Vault) Keys() error {
	return v.keys()
}

// Unseals vault with the keys provided in a k8s secret.
func (v Vault) Unseal() error {
	return v.unseal()
}

// Namespace contains test targets.
type Test mg.Namespace

// Test Go source files.
func (t Test) Go() error {
	return t.golang()
}

// Test to make sure all k8s Deployments are Ready.
func (t Test) Deployment() error {
	return t.deployment()
}

// Test to make sure all k8s Pods are Ready.
func (t Test) Pods() error {
	return t.pods()
}

// Test end-to-end functionality of full Orchestrator.
func (t Test) E2e(ctx context.Context) error {
	mg.SerialCtxDeps(
		ctx,
		Gen{}.OrchCA,
		Deploy{}.OrchCA,
		Router{}.Stop,
		Router{}.Start,
		Test{}.createTestProject,
		DevUtils{}.CreateDefaultUser,
	)

	return t.e2e()
}

// Test end-to-end functionality of Tenancy Services via Api-Gw.
func (t Test) E2eTenancyApiGw(ctx context.Context) error {
	mg.SerialCtxDeps(
		ctx,
		Gen{}.OrchCA,
		Deploy{}.OrchCA,
		Router{}.Stop,
		Router{}.Start,
	)

	return t.e2eTenancyApiGw()
}

// Test end-to-end functionality of Observability.
func (t Test) E2eObservability(ctx context.Context) error {
	mg.SerialCtxDeps(
		ctx,
		Gen{}.OrchCA,
		Deploy{}.OrchCA,
		Router{}.Stop,
		Router{}.Start,
	)

	return t.e2eObservability()
}

// Test end-to-end functionality of Orchestrator observability.
func (t Test) E2eOrchObservability(ctx context.Context) error {
	mg.SerialCtxDeps(
		ctx,
		Gen{}.OrchCA,
		Deploy{}.OrchCA,
		Router{}.Stop,
		Router{}.Start,
	)

	return t.e2eOrchObservability()
}

// Test end-to-end functionality of Edgenode observability.
func (t Test) E2eEnObservability(ctx context.Context) error {
	mg.SerialCtxDeps(
		ctx,
		Gen{}.OrchCA,
		Deploy{}.OrchCA,
		Router{}.Stop,
		Router{}.Start,
	)

	return t.e2eEnObservability()
}

// Test end-to-end functionality of observability alerts.
func (t Test) E2eAlertsObservability(ctx context.Context) error {
	mg.SerialCtxDeps(
		ctx,
		Gen{}.OrchCA,
		Deploy{}.OrchCA,
		Router{}.Stop,
		Router{}.Start,
	)

	return t.e2eAlertsObservability()
}

// Test end-to-end functionality of observability alerts (extended, includes long-duration tests).
func (t Test) E2eAlertsObservabilityExtended(ctx context.Context) error {
	mg.SerialCtxDeps(
		ctx,
		Gen{}.OrchCA,
		Deploy{}.OrchCA,
		Router{}.Stop,
		Router{}.Start,
	)

	return t.e2eAlertsObservabilityExtended()
}

// Test end-to-end functionality of SRE Exporter.
func (t Test) E2eSreObservability(ctx context.Context) error {
	mg.SerialCtxDeps(
		ctx,
		Gen{}.OrchCA,
		Deploy{}.OrchCA,
		Router{}.Stop,
		Router{}.Start,
	)

	return t.e2eSreObservability()
}

// Test end-to-end functionality of SRE Exporter with No Enic deployed.
func (t Test) E2eSreObservabilityNoEnic(ctx context.Context) error {
	mg.SerialCtxDeps(
		ctx,
		Gen{}.OrchCA,
		Deploy{}.OrchCA,
		Router{}.Stop,
		Router{}.Start,
	)

	return t.e2eSreObservabilityNoEnic()
}

// Test end-to-end functionality of Tenancy Services via Nexus Client.
func (t Test) E2eTenancy(ctx context.Context) error {
	mg.SerialCtxDeps(
		ctx,
		Gen{}.OrchCA,
		Deploy{}.OrchCA,
		Router{}.Stop,
		Router{}.Start,
	)

	return t.tenancyTestsViaClient(ctx)
}

func (t Test) CreateTestProject(ctx context.Context) error {
	mg.SerialCtxDeps(
		ctx,
		Gen{}.OrchCA,
		Deploy{}.OrchCA,
		Router{}.Stop,
		Router{}.Start,
	)

	return t.createTestProject(ctx)
}

// Test end-to-end functionality of full Orchestrator.
func (t Test) E2eOnPrem(ctx context.Context) error {
	mg.SerialCtxDeps(
		ctx,
		Gen{}.OrchCA,
		Deploy{}.OrchCA,
		Test{}.createTestProject,
		DevUtils{}.CreateDefaultUser,
	)

	return t.e2e()
}

// Perform stress test of Traefik endpoint in Orchestrator.
func (t Test) Stress(ctx context.Context) error {
	mg.SerialCtxDeps(
		ctx,
		Gen{}.OrchCA,
		Deploy{}.OrchCA,
		Router{}.Stop,
		Router{}.Start,
	)

	return t.stress()
}

// ClusterOrchSmokeTest Run cluster orch smoke test.
func (t Test) ClusterOrchSmokeTest(ctx context.Context) error {
	mg.SerialCtxDeps(
		ctx,
		Gen{}.OrchCA,
		Deploy{}.OrchCA,
	)

	return t.clusterOrchSmoke()
}

// Test end-to-end functionality of Autocert deployment.
func (t Test) E2eAutocert(ctx context.Context) error {
	mg.SerialCtxDeps(
		ctx,
		Gen{}.OrchCA,
		Deploy{}.OrchCA,
		Router{}.Stop,
		Router{}.Start,
	)

	return t.e2eAutocert()
}

// Test by label.
func (t Test) E2eByLabel(label string) error {
	return t.e2eByLabel(label)
}

// Test FlexCore deployment using gnodeb_proxy.
func (t Test) FlexCore() error {
	return t.flexCore()
}

// Namespace contains Gen targets.
type Gen mg.Namespace

// Returns domain assigned to orchestrator if using auto cert management
func (g Gen) OrchestratorDomain() error {
	return g.orchestratorDomain()
}

// Creates fullchain CA bundle for Orchestrator. Used for integration testing with autocert deployment
func (g Gen) LEOrchestratorCABundle() error {
	return g.orchCABundle("orchestrator-ca-bundle.crt")
}

// RegistryCacheCert generates the Registry cache x509 certificate file for a specified target environment.
func (Gen) RegistryCacheCert(targetEnv string) error {
	cacheRegistry, _ := (Config{}).getDockerCache(targetEnv)
	cacheRegistry = strings.TrimSpace(cacheRegistry)
	cacheRegistryCert, _ := (Config{}).getDockerCacheCert(targetEnv)
	cacheRegistryCert = strings.TrimSpace(cacheRegistryCert)

	if cacheRegistry != "" && cacheRegistryCert != "" {
		if err := os.WriteFile(filepath.Join("mage", "registry-cache-ca.crt"), []byte(cacheRegistryCert), 0o644); err != nil {
			return fmt.Errorf("failed to write cache registry certificate to file: %w", err)
		}
		fmt.Printf("Cache registry certificate written to: %s\n", filepath.Join("mage", "registry-cache-ca.crt"))
	}

	return nil
}

// Hostfile Generates entries to be added/modified in a REMOTE hostfile via IP specified by the user.
func (g Gen) Hostfile(ip string) error {
	return g.hostfile(ip, true)
}

// Hostfile Generates entries to be added/modified in a local hostfile (using Traefik svc ExternalIP).
func (g Gen) HostfileTraefik() error {
	return g.hostfileTraefik()
}

// GetHostSNICollection Generates rules to be added/modified to traefik.yml.
func (g Gen) GetHostSNICollection() error {
	hostSNICollection, err := g.GethostSNICollection()
	println(hostSNICollection)
	return err
}

// OrchCA Saves Orchestrator's CA certificate to `orch-ca.crt` so it can be imported to trust store for web access.
func (g Gen) OrchCA() error {
	return g.orchCA("orch-ca.crt")
}

// DockerImageManifest Generates Manifest of Docker Images after deploying Orchestrator to kind cluster.
func (g Gen) DockerImageManifest() error {
	return g.dockerImageManifest()
}

// Create a Release manifest file.
func (g Gen) ReleaseManifest(manifestFilename string) error {
	return g.releaseManifest(manifestFilename)
}

// Print the Release manifest details to stdout.
func (g Gen) DumpReleaseManifest() error {
	return g.dumpReleaseManifest()
}

// Create a Release image manifest file.
func (g Gen) ReleaseImageManifest(manifestFilename string) error {
	return g.releaseImageManifest(manifestFilename)
}

// Print Release image manifest details to stdout.
func (g Gen) DumpReleaseImageManifest() error {
	return g.dumpReleaseImageManifest()
}

// Create a Release image manifest with local charts
func (g Gen) LocalReleaseImageManifest(manifestFilename string) error {
	return g.localReleaseImageManifest(manifestFilename)
}

// Create a document showing firewall configurationt.
func (g Gen) FirewallDoc() error {
	return g.firewallDoc()
}

type Config mg.Namespace

func (c Config) CreateCluster() error {
	_, err := c.createCluster()
	if err != nil {
		return fmt.Errorf("failed to create cluster: %w", err)
	}
	return nil
}

// Create a cluster deployment configuration from a cluster values file.
func (c Config) UsePreset(clusterPresetFile string) error {
	_, err := c.usePreset(clusterPresetFile)
	if err != nil {
		return fmt.Errorf("failed to render preset to cluster: %w", err)
	}
	return nil
}

// Create a cluster values file using the cluster configuration interface.
func (c Config) CreatePreset() error {
	return c.createPreset()
}

// Clean out generated cluster configuration files.
func (c Config) Clean() error {
	return c.clean()
}

// Render a Cluster configuration.
func (c Config) Debug(targetEnv string) error {
	return c.debug(targetEnv)
}

// Namespace contains Use targets.
type Use mg.Namespace

// Current Show current kubectl context.
func (Use) Current() error {
	if _, err := script.Exec("kubectl config current-context").Stdout(); err != nil {
		return err
	}
	return nil
}

// Switch kubectl context to Orchestrator.
func (Use) Orch() error {
	if _, err := script.Exec(fmt.Sprintf("kubectl config use-context kind-%s", kindOrchClusterName)).
		Stdout(); err != nil {
		return err
	}
	return nil
}

// EdgeCluster Switch kubectl context to ENiC Edge Cluster.
func (Use) EdgeCluster() error {
	if _, err := script.Exec(fmt.Sprintf("kubectl config use-context %s-admin", edgeClusterName)).
		Stdout(); err != nil {
		return err
	}
	return nil
}

// EdgeClusterWithName Switch kubectl context to named Edge Cluster (for multiple edges).
func (Use) EdgeClusterWithName(edgeName string) error {
	if _, err := script.Exec(fmt.Sprintf("kubectl config use-context %s", edgeName)).
		Stdout(); err != nil {
		return err
	}
	return nil
}

// EdgeClusterGetNames Show all kubectl contexts.
func (Use) EdgeClusterGetNames() error {
	if _, err := script.Exec("kubectl config get-contexts").
		Stdout(); err != nil {
		return err
	}
	return nil
}

// Namespace contains registry targets.
type Registry mg.Namespace

// Namespace contains App targets.
type App mg.Namespace

// Upload sample applications to Catalog.
func (a App) Upload() error {
	return a.upload()
}

// Deploys Wordpress via Orchestrator using public charts and images.
func (a App) Wordpress() error {
	return a.wordpress()
}

// Deploys Wordpress via Orchestrator from the private Harbor registry.
func (a App) WordpressFromPrivateRegistry() error {
	return a.wordpressFromPrivateRegistry()
}

// Deploys iPerf-Web VM via Orchestrator from the private Harbor registry.
func (a App) IperfWebVM() error {
	return a.iperfWebVM()
}

// Deploys NGINX via Orchestrator using public charts and images.
func (a App) Nginx() error {
	return a.nginx()
}

type Tarball mg.Namespace

// OnpremFull Creates a Tarball of artifacts for OnPrem deployment of Full orchestrator
func (t Tarball) OnpremFull() error {
	return t.setupCollectors("onpremFull", []string{"onprem", "onprem-explicit-proxy", "onprem-1k"})
}

// OnpremFullIntel Creates a Tarball of artifacts for OnPrem deployment of orchestrator inside Intel
func (t Tarball) OnpremFullIntel() error {
	return t.setupCollectors("onpremFullIntel", []string{"onprem-dev", "onprem-dev-explicit-proxy"})
}

// CloudFull Creates a Tarball of artifacts for Cloud deployment of Full orchestrator
func (t Tarball) CloudFull() error {
	return t.setupCollectors("cloudFull", []string{"example", "example-staging"})
}

type Installer mg.Namespace

// Builds the Installer images. DOCKER_REGISTRY and DOCKER_REPOSITORY environment variables can override default image repo path.
func (i Installer) Build() error {
	return i.build()
}

// Creates the Installer release artifact. DOCKER_REGISTRY and DOCKER_REPOSITORY environment variables can override default installer image repo path.
func (i Installer) Bundle() error {
	return i.bundle()
}

// Cleans the Installer images. DOCKER_REGISTRY and DOCKER_REPOSITORY environment variables can override default image repo path.
func (i Installer) Clean() error {
	return i.clean()
}

// Publishes the Installer Docker images, using DOCKER_REGISTRY and DOCKER_REPOSITORY in environment
func (i Installer) Publish() error {
	return i.publish()
}

type CoUtils mg.Namespace

type DevUtils mg.Namespace

type LogUtils mg.Namespace

type Version mg.Namespace

// Get the Release Tag for the current source version
func (Version) GetVersionTag() error {
	tag, err := getDeployTag()
	if err != nil {
		return fmt.Errorf("failed to get deploy tag: %w", err)
	}
	fmt.Println(tag)
	return nil
}

// Checks that the Version in the VERSION file and in the Argo Charts is in syn
func (v Version) CheckVersion() error {
	return v.checkVersion()
}

// Reads the Version from the Version file and updates the Argo Charts
func (v Version) SetVersion() error {
	return v.setVersion()
}

type TenantUtils mg.Namespace
