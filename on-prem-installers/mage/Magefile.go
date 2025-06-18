// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bitfield/script"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"

	"github.com/open-edge-platform/edge-manageability-framework/mage"
)

const (
	// RKE2 related constants.
	rke2Version                    = "v1.30.10+rke2r1"
	rke2ImagesPkg                  = "rke2.linux-amd64.tar.gz"
	rke2LibPkg                     = "rke2-images.linux-amd64.tar.zst"
	rke2CalicoLibPkg               = "rke2-images-calico.linux-amd64.tar.zst"
	rke2LibSHAFile                 = "sha256sum-amd64.txt"
	openEbsOperatorK8sTemplateFile = "openebs-operator.yaml"
	rke2ArtifactDownloadPath       = "assets/rke2"
	rke2CustomImageDownloadPath    = "assets/rke2/offline-images/"
	rke2ImagesURLFmt               = "https://github.com/rancher/rke2/releases/download/%s/rke2.linux-amd64.tar.gz"
	//nolint: all
	rke2LibURLFmt = "https://github.com/rancher/rke2/releases/download/%s/rke2-images.linux-amd64.tar.zst"

	//nolint: all
	rke2CNICalicoURLFmt = "https://github.com/rancher/rke2/releases/download/%s/rke2-images-calico.linux-amd64.tar.zst"
	//nolint: all
	openEbsOperatorK8sTemplate = "https://raw.githubusercontent.com/openebs/charts/gh-pages/versioned/3.9.0/openebs-operator.yaml"
	//nolint: all
	openEbsHostPathStorageK8sTemplate = "https://raw.githubusercontent.com/openebs/dynamic-localpv-provisioner/refs/heads/release/4.2/deploy/kubectl/hostpath-operator.yaml"

	deploymentTimeoutEnv     = "DEPLOYMENT_TIMEOUT"
	defaultDeploymentTimeout = "1200s" // timeout must be a valid string
)

// Removes build and deployment artifacts.
func Clean() error {
	buildFiles, err := filepath.Glob("secrets-config-*.tgz")
	if err != nil {
		return fmt.Errorf("glob search: %w", err)
	}

	for _, path := range append(
		[]string{
			"build",
			"dist",
			"edge-node-ca.crt",
			"jwt.txt",
			"orch-ca.crt",
			"vault-keys.json",
		},
		buildFiles...,
	) {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("clean %s: %w", path, err)
		}
	}

	return nil
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

// Namespace contains Lint targets.
type Lint mg.Namespace

// Lint markdown files using markdownlint-cli2 tool.
func (Lint) Markdown() error {
	return sh.RunV("markdownlint-cli2", "--config", "../tools/.markdownlint-cli2.yaml", "**/*.md")
}

// Namespace contains Build targets.
type Build mg.Namespace

// Deps ensures the required directories are created and checks for uncommitted changes.
func (Build) Deps() error {
	if err := os.MkdirAll("dist", os.ModePerm); err != nil {
		return fmt.Errorf("failed to create 'dist' directory")
	}

	// Check if there are any uncommitted changes since the DEB package version is derived from the latest git tag. If
	// there are uncommitted changes, the DEB package version might be incorrect.
	// TODO: Enforce and block from continuing in the future.
	if _, err := script.Exec("git diff --exit-code").Stdout(); err != nil {
		fmt.Printf("WARNING: unstaged changes present, commit or stash changes: %s ⚠️\n", err)
	}

	if _, err := script.Exec("git diff --cached --exit-code").Stdout(); err != nil {
		fmt.Printf("WARNING: unstaged changes present, commit or stash changes: %s ⚠️\n", err)
	}

	return nil
}

// Builds all the installers. Must run on Ubuntu 22.04.
func (b Build) All(ctx context.Context) error {
	mg.CtxDeps(ctx, Clean)

	mg.CtxDeps(
		ctx,
		b.OnpremKEInstaller,
		b.OSConfig,
		b.GiteaInstaller,
		b.ArgocdInstaller,
		b.OnPremOrchInstaller,
	)

	return nil
}

// Build the onprem-KE Installer package. By default builds online installer.
func (b Build) OnpremKEInstaller(ctx context.Context) error {
	mg.CtxDeps(
		ctx,
		b.Deps,
		mage.Deps.FPM,
	)

	return b.onpremKeInstaller()
}

// Build the OS-Config Installer package.
func (b Build) OSConfig(ctx context.Context) error {
	mg.CtxDeps(
		ctx,
		b.Deps,
		mage.Deps.FPM,
	)

	return b.osConfigInstaller()
}

// Build the Gitea Installer package.
func (b Build) GiteaInstaller(ctx context.Context) error {
	mg.CtxDeps(
		ctx,
		b.Deps,
		mage.Deps.FPM,
	)

	return b.giteaInstaller()
}

// Build the Argo-Cd Installer package.
func (b Build) ArgocdInstaller(ctx context.Context) error {
	mg.CtxDeps(
		ctx,
		b.Deps,
		mage.Deps.FPM,
	)

	return b.argoCdInstaller()
}

// Builds Orch Installer package.
func (b Build) OnPremOrchInstaller(ctx context.Context) error {
	mg.CtxDeps(
		ctx,
		b.Deps,
		mage.Deps.FPM,
	)

	return b.onPremOrchInstaller()
}

const (
	AWSRegion                         = "us-west-2"
	OpenEdgePlatformRegistryRepoURL   = "080137407410.dkr.ecr.us-west-2.amazonaws.com"
	OpenEdgePlatformRepository        = "edge-orch"
	RegistryRepoSubProj               = "common"
	OpenEdgePlatformContainerRegistry = OpenEdgePlatformRegistryRepoURL + "/" + OpenEdgePlatformRepository + "/" + RegistryRepoSubProj
	OpenEdgePlatformChartRegistry     = OpenEdgePlatformRegistryRepoURL + "/" + OpenEdgePlatformRepository + "/" + RegistryRepoSubProj + "/charts" //nolint: lll
	OpenEdgePlatformFilesRegistry     = OpenEdgePlatformRegistryRepoURL + "/" + OpenEdgePlatformRepository + "/" + RegistryRepoSubProj + "/files"  //nolint: lll
)

// Namespace contains Publish targets.
type Publish mg.Namespace

// Publish everything. Valid authentication with push access to the registry is required for this target.
func (Publish) All(ctx context.Context) error {
	mg.CtxDeps(ctx, Publish{}.DEBPackages)
	mg.CtxDeps(ctx, Publish{}.Files)
	return nil
}

// Publish DEB packages. Valid authentication with push access to the registry is required for this target.
func (Publish) DEBPackages(ctx context.Context) error {
	matches, err := filepath.Glob(filepath.Join("dist", "*.deb"))
	if err != nil {
		return fmt.Errorf("failed to list .deb files: %w", err)
	}

	// Strip dist from file paths since oras push requires the file name only or the artifact will include the entire
	// dist directory.
	var files []string
	for _, match := range matches {
		files = append(files, filepath.Base(match))
	}

	if len(files) == 0 {
		return fmt.Errorf("no .deb files found in dist directory")
	}

	// get branch name
	branchName, err := GetBranchName()
	if err != nil {
		return fmt.Errorf("failed get branch name: %w", err)
	}

	version, err := mage.GetDebVersion()
	if err != nil {
		return fmt.Errorf("failed to get DEB version: %w", err)
	}

	fmt.Printf("Version: %s\n", version)

	// Create ECR repository if it does not exist
	if err := mage.TryToCreateECRRepository(ctx, OpenEdgePlatformFilesRegistry); err != nil {
		fmt.Printf("failed to create ECR repository %s, ignoring: %v\n", OpenEdgePlatformFilesRegistry, err)
	}

	for _, file := range files {
		fmt.Printf("Processing file: %s\n", file)

		cmd := exec.CommandContext(
			ctx,
			"dpkg-deb", "--showformat=${Package}", "--show", file,
		)
		cmd.Dir = "dist"

		stdouterr, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to get name for %s: %w: %s", file, err, string(stdouterr))
		}

		name := string(stdouterr)
		if name == "" {
			return fmt.Errorf("failed to get name for %s: empty name", file)
		}

		var (
			// Set tags equal to the version and latest-<branch-name>-dev
			tags         = fmt.Sprintf("%s,latest-%s-dev", version, branchName)
			artifactName = fmt.Sprintf("%s/%s:%s", OpenEdgePlatformFilesRegistry, name, tags)
			repoName     = fmt.Sprintf("%s/%s/files/%s", OpenEdgePlatformRepository, RegistryRepoSubProj, name)
		)

		// If on main or a release branch, the version as declared in the VERSION file should be added
		if branchName == "main" ||
			strings.Contains(branchName, "pass-validation") ||
			strings.HasPrefix(branchName, "release") {
			tags = fmt.Sprintf("%s,latest-%s-dev", version, branchName)
			artifactName = fmt.Sprintf("%s/%s:%s", OpenEdgePlatformFilesRegistry, name, tags)
		}

		// Create ECR repository if it does not exist
		if err := mage.TryToCreateECRRepository(ctx, repoName); err != nil {
			fmt.Printf("failed to create ECR repository %s, ignoring: %v\n", repoName, err)
		}

		fmt.Println("Pushing to registry:", artifactName)

		cmd = exec.CommandContext(
			ctx,
			"oras",
			"push",
			artifactName,
			"--artifact-type", "application/vnd.intel.oep.deb", // TODO: Change to correct type
			file,
		)

		// Switch to dist/ directory to flatten the structure
		cmd.Dir = "dist"

		if stdouterr, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to push %s to registry: %w: %s", file, err, string(stdouterr))
		}
	}

	fmt.Printf("All DEB installer packages with tag %s pushed to the registry ✅\n", version)

	return nil
}

// Publish generic files. Valid authentication with push access to the registry is required for this target. Once
// published, the files will be available via the Release service with `oras pull $ARTIFACT_NAME e.g.,
// oras pull registry-rs.edgeorchestration.intel.com/edge-orch/common/files/functions.sh:3.0.0-dev-d5ed6ff
func (Publish) Files(ctx context.Context) error {
	version, err := mage.GetDebVersion()
	if err != nil {
		return fmt.Errorf("failed to get version: %w", err)
	}

	branchName, err := GetBranchName()
	if err != nil {
		return fmt.Errorf("failed get branch name: %w", err)
	}

	fmt.Println("Version: ", version)

	// Create ECR repository for sub-project if it does not exist
	repoName := fmt.Sprintf("%s/%s/files/on-prem", OpenEdgePlatformRepository, RegistryRepoSubProj)
	if err := mage.TryToCreateECRRepository(ctx, repoName); err != nil {
		fmt.Printf("failed to create ECR repository %s, ignoring: %v\n", repoName, err)
	}
	var (
		tags         = fmt.Sprintf("%s,latest-%s-dev", version, branchName)
		artifactName = fmt.Sprintf("%s/on-prem:%s", OpenEdgePlatformFilesRegistry, tags)
	)

	// If on main or a release branch, the version as declared in the VERSION file should be added
	if branchName == "main" ||
		strings.Contains(branchName, "pass-validation") ||
		strings.HasPrefix(branchName, "release") {
		tags = fmt.Sprintf("%s,latest-%s-dev", version, branchName)
		artifactName = fmt.Sprintf("%s/on-prem:%s", OpenEdgePlatformFilesRegistry, tags)
	}

	fmt.Println("Pushing to registry:", artifactName)

	matches, err := filepath.Glob(filepath.Join("onprem", "*.sh"))
	if err != nil {
		return fmt.Errorf("failed to list .sh files: %w", err)
	}

	// Strip onprem from file paths since oras push requires the file name only or the artifact will include the entire
	// dist directory.
	var files []string
	for _, match := range matches {
		files = append(files, filepath.Base(match))
	}

	cmdArgs := []string{
		"push",
		artifactName,
		"--artifact-type", "application/vnd.intel.orch.file",
	}

	cmdArgs = append(cmdArgs, files...)

	cmd := exec.CommandContext(
		ctx,
		"oras",
		cmdArgs...,
	)

	// Switch to onprem/ directory to flatten the structure
	cmd.Dir = "onprem"

	if stdouterr, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to push to registry: %w: %s", err, string(stdouterr))
	}
	fmt.Printf("All files pushed to the registry ✅\n")

	return nil
}

// Namespace contains deploy targets.
type Deploy mg.Namespace

// Rke2Cluster Deploys a local RKE2 Kubernetes cluster.
func (d Deploy) Rke2Cluster() error {
	return d.rke2Cluster()
}

// Namespace contains upgrade targets.
type Upgrade mg.Namespace

func (u Upgrade) Rke2Cluster() error {
	return u.rke2Cluster()
}

// Namespace contains undeploy targets.
type Undeploy mg.Namespace

// Deletes rke2 server clusters.
func (u Undeploy) Rke2Cluster() error {
	// TODO: Return nil if no cluster exists
	return u.rke2server()
}

// Namespace contains Gen targets.
type Gen mg.Namespace

// Generates Intel SHA256 Private Root Certificate Chain certificates.
func (Gen) IntelSHA256PrivateRootCertChain() error {
	file, err := os.Create("IntelSHA2RootChain-Base64.zip")
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()
	defer os.Remove(file.Name())

	resp, err := http.Get("https://certificates.intel.com/repository/certificates/IntelSHA2RootChain-Base64.zip")
	if err != nil {
		return fmt.Errorf("failed to get response: %w", err)
	}
	defer resp.Body.Close()

	if _, err = io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("failed to copy response body: %w", err)
	}

	dir := filepath.Join("terraform", "ca-certificates")

	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("failed to clean up existing directory: %w", err)
	}

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return sh.RunV("unzip", "-o", file.Name(), "-d", dir)
}

// Namespace contains registry targets.
type Registry mg.Namespace

// Namespace contains test targets.
type Test mg.Namespace
