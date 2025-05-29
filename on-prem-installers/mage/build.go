// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitfield/script"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/open-edge-platform/edge-manageability-framework/mage"
)

func GetBranchName() (string, error) {
	branchName := os.Getenv("BRANCH_NAME")
	if branchName == "" {
		output, err := script.Exec("git rev-parse --abbrev-ref HEAD").String()
		if err != nil {
			return "", fmt.Errorf("failed to execute git command to get branch name: %w", err)
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
		return "", fmt.Errorf("failed to read 'VERSION' file: %w", err)
	}
	return strings.TrimSpace(string(contents)), nil
}

// Returns the DEB package version
func (Build) DEBVersion() error {
	version, err := mage.GetDebVersion()
	if err != nil {
		return fmt.Errorf("failed to get DEB version: %w", err)
	}

	fmt.Println(version)

	return nil
}

const (
	giteaPath             = "assets/gitea"
	giteaChartVersion     = "10.4.0"
	argocdPath            = "assets/argo-cd"
	argocdHelmVersion     = "8.0.0"
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
	fmt.Println("Compile onprem-ke-installer executable")

	deployFilePath := deployOnlineDirectory

	mg.SerialDeps(
		mg.F(
			compile,
			filepath.Join(".", "cmd", deployFilePath, "main.go"),
			filepath.Join(".", "dist", "bin", deployFilePath),
		),
	)

	debVersion, err := mage.GetDebVersion()
	if err != nil {
		return fmt.Errorf("failed to get DEB version for onprem-ke-installer: %w", err)
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

func (Build) argoCdInstaller() error {
	// ArgoCD helm installation
	cmd := "helm repo add argo-helm https://argoproj.github.io/argo-helm --force-update"
	if _, err := script.Exec(cmd).Stdout(); err != nil {
		return fmt.Errorf("failed to add argo helm repo: %w", err)
	}

	if err := os.RemoveAll(filepath.Join(argocdPath, "argo-cd")); err != nil {
		return err
	}

	cmd = fmt.Sprintf("helm fetch argo-helm/argo-cd --version %v --untar --untardir %v", argocdHelmVersion, argocdPath)
	if _, err := script.Exec(cmd).Stdout(); err != nil {
		return fmt.Errorf("failed to fetch argo-cd chart: %w", err)
	}

	// Gitea helm installation
	cmd = "helm repo add gitea-charts https://dl.gitea.com/charts/ --force-update"
	if _, err := script.Exec(cmd).Stdout(); err != nil {
		return fmt.Errorf("failed to add gitea helm repo: %w", err)
	}

	if err := os.RemoveAll(filepath.Join(giteaPath, "gitea")); err != nil {
		return err
	}

	cmd = fmt.Sprintf("helm fetch gitea-charts/gitea --version %v --untar --untardir %v", giteaChartVersion, giteaPath)
	if _, err := script.Exec(cmd).Stdout(); err != nil {
		return fmt.Errorf("failed to fetch gitea chart: %w", err)
	}

	debVersion, err := mage.GetDebVersion()
	if err != nil {
		return fmt.Errorf("failed to get DEB version for argoCdInstaller: %w", err)
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
		giteaPath+"=/tmp/",
	); err != nil {
		return err
	}

	return os.RemoveAll(filepath.Join(argocdPath, "argo-cd"))
}

func (Build) onPremOrchInstaller() error {
	if err := downloadTeaBinary(); err != nil {
		return fmt.Errorf("failed to download tea binary: %w", err)
	}

	mg.SerialDeps(
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

	debVersion, err := mage.GetDebVersion()
	if err != nil {
		return fmt.Errorf("failed to get DEB version for onPremOrchInstaller: %w", err)
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
		return fmt.Errorf("failed to download tea binary: %w", err)
	}

	if err := os.Chmod(usrBinTea, 0700); err != nil { //nolint:gofumpt
		return fmt.Errorf("failed to set permissions for tea binary: %w", err)
	}

	return nil
}
