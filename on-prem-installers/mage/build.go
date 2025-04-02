// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bitfield/script"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

func GetBranchName() (string, error) {
	branchName := os.Getenv("BRANCH_NAME")
	if branchName == "" {
		output, err := script.Exec("git rev-parse --abbrev-ref HEAD").String()
		if err != nil {
			return "", fmt.Errorf("get branch name: %w", err)
		}
		branchName = strings.TrimSpace(output)
	}

	// If branch name contains slashes, replace them with hyphens
	branchName = strings.ReplaceAll(branchName, "/", "-")

	return branchName, nil
}

func GetRepoVersion() (string, error) {
	contents, err := os.ReadFile("VERSION")
	if err != nil {
		return "", fmt.Errorf("read version from 'VERSION' file: %w", err)
	}
	return strings.TrimSpace(string(contents)), nil
}

func GetDebVersion() (string, error) {
	repoVersion, err := GetRepoVersion()
	if err != nil {
		return "", fmt.Errorf("read version from 'VERSION' file: %w", err)
	}

	// get branch name
	branchName, err := GetBranchName()
	if err != nil {
		return "", fmt.Errorf("get branch name: %w", err)
	}

	// check if release branch
	isReleaseBranch := branchName == "main" ||
		strings.Contains(branchName, "pass-validation") ||
		strings.HasPrefix(branchName, "release")

	// If release version on release branches, return the version as is
	if isReleaseBranch && !strings.Contains(repoVersion, "-dev") {
		return repoVersion, nil // e.g., 3.0.0, 3.0.0-rc1, 3.0.0-n20250306
	}

	stdout, err := exec.Command("git", "rev-parse", "--short", "HEAD").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("get current git hash: %w", err)
	}
	shortHash := strings.TrimSpace(string(stdout))

	// else, return the version with short hash appended
	return fmt.Sprintf("%s-%s", repoVersion, shortHash), nil // e.g., 3.0.0-dev-7d763f9, 3.0.0-rc1-7d763f9 (in pre-merge CI)
}

// Returns the DEB package version
func (Build) DEBVersion() error {
	version, err := GetDebVersion()
	if err != nil {
		return err
	}

	fmt.Println(version)

	return nil
}

const (
	giteaPath             = "assets/gitea"
	giteaChartVersion     = "10.4.0"
	argocdPath            = "assets/argo-cd"
	argocdHelmVersion     = "7.4.4"
	deployOnlineDirectory = "onprem-ke-installer"
)

func compile(path, output string) error {
	return sh.RunWithV(map[string]string{
		"CGO_ENABLED": "0",
		"GOARCH":      "amd64",
		"GOOS":        "linux",
	},
		"go",
		"build",
		"-ldflags", "-s -w -extldflags=-static",
		"-o", output,
		path,
	)
}

func (Build) onpremKeInstaller() error {
	fmt.Println("Fetch dependencies and compile onprem-ke-installer executable")

	deployFilePath := deployOnlineDirectory

	mg.SerialDeps(
		Clean,
		// Must compile after everything is fetched in order to package dependencies
		mg.F(
			compile,
			filepath.Join(".", "cmd", deployFilePath, "main.go"),
			filepath.Join(".", "dist", "bin", deployFilePath),
		),
	)

	debVersion, err := GetDebVersion()
	if err != nil {
		return fmt.Errorf("get DEB version: %w", err)
	}

	fmt.Println("Build onprem-ke-installer package 📦")
	return sh.RunV(
		"fpm",
		"-s", "dir",
		"-t", "deb",
		"--name", "onprem-ke-installer",
		"-p", "./dist/",
		"--version", debVersion,
		"--architecture", "amd64",
		"--description", "Installs Intel onprem-ke",
		"--url", "https://github.com/open-edge-platform/edge-manageability-framework/on-prem-installers",
		"--maintainer", "Intel Corporation",
		"--after-install", "./cmd/onprem-ke-installer/after-install.sh",
		"--after-remove", "./cmd/onprem-ke-installer/after-remove.sh",
		"--after-upgrade", "./cmd/onprem-ke-installer/after-upgrade.sh",
		"./dist/bin/onprem-ke-installer=/usr/bin/onprem-ke-installer",
		"./cmd/onprem-ke-installer/onprem-ke-installer.1=/usr/share/man/man1/onprem-ke-installer.1",
		"./rke2=/tmp/onprem-ke-installer",
	)
}

func (Build) osConfigInstaller() error {
	fmt.Println("Fetch dependencies and compile configuration installer executable")
	mg.SerialDeps(
		Clean,
		// Must compile after everything is fetched in order to package dependencies
		mg.F(
			compile,
			filepath.Join(".", "cmd", "onprem-config-installer", "main.go"),
			filepath.Join(".", "dist", "bin", "onprem-config-installer"),
		),
	)

	debVersion, err := GetDebVersion()
	if err != nil {
		return fmt.Errorf("get DEB version: %w", err)
	}

	fmt.Println("Build onprem-config-installer package 📦")
	return sh.RunV(
		"fpm",
		"-s", "dir",
		"-t", "deb",
		"--name", "onprem-config-installer",
		"-p", "./dist",
		"-d", "jq,libpq5,apparmor,lvm2,mosquitto,net-tools,ntp,openssh-server", //nolint:misspell
		"-d", "software-properties-common,tpm2-abrmd,tpm2-tools,unzip",
		"--version", debVersion,
		"--architecture", "amd64",
		"--description", "OS Configuration Powered By Intel",
		"--url", "https://github.com/open-edge-platform/edge-manageability-framework/on-prem-installers",
		"--maintainer", "Intel Corporation",
		"--after-install", "./cmd/onprem-config-installer/after-install.sh",
		"--after-remove", "./cmd/onprem-config-installer/after-remove.sh",
		"./dist/bin/onprem-config-installer=/usr/bin/onprem-config-installer",
		"./cmd/onprem-config-installer/onprem-config-installer.1=/usr/share/man/man1/onprem-config-installer.1",
	)
}

func (Build) giteaInstaller() error {
	cmd := "helm repo add gitea-charts https://dl.gitea.com/charts/ --force-update"
	if _, err := script.Exec(cmd).Stdout(); err != nil {
		return err
	}

	if err := os.RemoveAll(filepath.Join(giteaPath, "gitea")); err != nil {
		return err
	}

	cmd = fmt.Sprintf("helm fetch gitea-charts/gitea --version %v --untar --untardir %v", giteaChartVersion, giteaPath)
	if _, err := script.Exec(cmd).Stdout(); err != nil {
		return err
	}

	debVersion, err := GetDebVersion()
	if err != nil {
		return fmt.Errorf("get DEB version: %w", err)
	}

	fmt.Println("Build gitea package 📦")
	if err := sh.RunV(
		"fpm",
		"-s", "dir",
		"-t", "deb",
		"--name", "onprem-gitea-installer",
		"-p", "./dist/",
		"--version", debVersion,
		"--architecture", "amd64",
		"--description", "Installs Gitea",
		"--url", "https://github.com/go-gitea/gitea",
		"--maintainer", "Intel Corporation",
		"--after-install", "cmd/onprem-gitea/after-install.sh",
		"--after-remove", "cmd/onprem-gitea/after-remove.sh",
		"--after-upgrade", "cmd/onprem-gitea/after-upgrade.sh",
		giteaPath+"=/tmp",
	); err != nil {
		return err
	}

	return os.RemoveAll(filepath.Join(giteaPath, "gitea"))
}

func (Build) argoCdInstaller() error {
	cmd := "helm repo add argo-helm https://argoproj.github.io/argo-helm --force-update"
	if _, err := script.Exec(cmd).Stdout(); err != nil {
		return err
	}

	if err := os.RemoveAll(filepath.Join(argocdPath, "argo-cd")); err != nil {
		return nil
	}

	cmd = fmt.Sprintf("helm fetch argo-helm/argo-cd --version %v --untar --untardir %v", argocdHelmVersion, argocdPath)
	if _, err := script.Exec(cmd).Stdout(); err != nil {
		return err
	}

	debVersion, err := GetDebVersion()
	if err != nil {
		return fmt.Errorf("get DEB version: %w", err)
	}

	fmt.Println("Build argocd-installer package 📦")
	if err := sh.RunV(
		"fpm",
		"-s", "dir",
		"-t", "deb",
		"--name", "onprem-argocd-installer",
		"-p", "./dist",
		"--version", debVersion,
		"--architecture", "amd64",
		"--description", "Installs argo-cd on the on-prem orchestrator",
		"--url", "https://github.com/argoproj/argo-cd",
		"--maintainer", "Intel Corporation",
		"--after-install", filepath.Join("cmd/onprem-argo-cd", "after-install.sh"),
		"--after-remove", filepath.Join("cmd/onprem-argo-cd", "after-remove.sh"),
		"--after-upgrade", filepath.Join("cmd/onprem-argo-cd", "after-upgrade.sh"),
		argocdPath+"=/tmp/",
	); err != nil {
		return err
	}

	return os.RemoveAll(filepath.Join(argocdPath, "argo-cd"))
}

func (Build) onPremOrchInstaller() error {
	if err := downloadTeaBinary(); err != nil {
		return err
	}

	// Statically compile the Orch Installer binary
	mg.SerialDeps(
		Clean,
		mg.F(
			compile,
			filepath.Join(".", "cmd", "onprem-orch-installer", "main.go"),
			filepath.Join(".", "dist", "bin", "onprem-orch-installer"),
		),
	)
	fmt.Println("Statically compile mage")
	if _, err := script.NewPipe().Exec("mage -compile ./assets/mage").Stdout(); err != nil {
		return fmt.Errorf("statically compiling mage: %w", err)
	}

	debVersion, err := GetDebVersion()
	if err != nil {
		return fmt.Errorf("get DEB version: %w", err)
	}

	fmt.Println("Build on-prem orch-installer package 📦")
	return sh.RunV(
		"fpm",
		"-s", "dir",
		"-t", "deb",
		"--name", "onprem-orch-installer",
		"-p", "./dist/",
		"--version", debVersion,
		"--architecture", "amd64",
		"--description", "Installs on-prem Orchestrator",
		"--url", "https://github.com/open-edge-platform/edge-manageability-framework",
		"--maintainer", "Intel Corporation",
		"--after-install", "./cmd/onprem-orch-installer/after-install.sh",
		"--after-remove", "./cmd/onprem-orch-installer/after-remove.sh",
		"./dist/bin/onprem-orch-installer=/usr/bin/orch-installer",
		"./assets/tea=/usr/bin/tea",
		"./cmd/onprem-orch-installer/generate_fqdn=/usr/bin/generate_fqdn",
	)
}

func downloadTeaBinary() error {
	const usrBinTea = "./assets/tea"

	const teaVersion = "0.9.2"

	err := downloadFile(usrBinTea, "https://dl.gitea.com/tea/"+teaVersion+"/tea-"+teaVersion+"-linux-amd64")
	if err != nil {
		return fmt.Errorf("failed to download tea binary - %w", err)
	}

	if err := os.Chmod(usrBinTea, 0700); err != nil { //nolint:gofumpt
		return fmt.Errorf("failed to chmod tea binary - %w", err)
	}

	return nil
}
