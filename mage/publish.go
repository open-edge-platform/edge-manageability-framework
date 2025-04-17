// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bitfield/script"
	"github.com/magefile/mage/mg"
)

// Publish is a namespace for publishing artifacts.
type Publish mg.Namespace

const (
	AWSRegion                 = "us-west-2"
	InternalRegistryRepoURL   = "080137407410.dkr.ecr.us-west-2.amazonaws.com"
	PublicRegistryRepoURL     = "registry-rs.edgeorchestration.intel.com"
	RepositoryName            = "edge-orch"
	RegistryRepoSubProj       = "common"
	InternalContainerRegistry = InternalRegistryRepoURL + "/" + RepositoryName + "/" + RegistryRepoSubProj
	PublicContainerRegistry   = PublicRegistryRepoURL + "/" + RepositoryName + "/" + RegistryRepoSubProj
	InternalChartRegistry     = InternalRegistryRepoURL + "/" + RepositoryName + "/" + RegistryRepoSubProj + "/charts" //nolint: lll
	InternalFilesRegistry     = InternalRegistryRepoURL + "/" + RepositoryName + "/" + RegistryRepoSubProj + "/files"  //nolint: lll
	PublicFilesRegistry       = PublicRegistryRepoURL + "/" + RepositoryName + "/" + RegistryRepoSubProj + "/files"    //nolint: lll
)

// Builds and publishes Orchestrator application source tarballs to the registry.
func (Publish) SourceTarballs(ctx context.Context) error {
	defaultRepoVersion, err := os.ReadFile("VERSION")
	if err != nil {
		return fmt.Errorf("failed to read VERSION file: %w", err)
	}
	defaultRepoVersionStr := strings.TrimSpace(string(defaultRepoVersion))

	gitTagName, err := script.Exec("git tag --points-at HEAD").String()
	if err != nil {
		return fmt.Errorf("failed to get git tag name: %w", err)
	}
	gitTagName = strings.TrimSpace(gitTagName)

	branchName, err := GetBranchName()
	if err != nil {
		return fmt.Errorf("failed to get branch name: %w", err)
	}

	var tag string

	// Set the default tag using the repository version, branch name, and version with 'v' prefix
	tag = fmt.Sprintf(
		"%s,v%s-%s,latest,%s,v%s",
		defaultRepoVersionStr,
		defaultRepoVersionStr,
		gitShortHash(),
		branchName,
		defaultRepoVersionStr,
	)

	// If the repository version contains '-dev', only tag with the git short hash and branch-specific tags
	if strings.Contains(defaultRepoVersionStr, "-dev") {
		tag = fmt.Sprintf(
			"%s-%s,latest-%s-dev,v%s-%s",
			defaultRepoVersionStr,
			gitShortHash(),
			branchName,
			defaultRepoVersionStr,
			gitShortHash(),
		)
	}

	// If there is a git tag name, append it to the tag
	if gitTagName != "" {
		tag = fmt.Sprintf("%s,%s,v%s", tag, gitTagName, gitTagName)
	}

	for _, variant := range []string{"cloudFull", "onpremFull"} {
		if err := buildVariant(ctx, variant); err != nil {
			return fmt.Errorf("failed to build variant %s: %w", variant, err)
		}

		fileName := fmt.Sprintf("%s_edge-manageability-framework_%s.tgz", variant, defaultRepoVersionStr)
		if _, err := os.Stat(fileName); os.IsNotExist(err) {
			return fmt.Errorf("file %s does not exist: %w", fileName, err)
		}

		variantLC := strings.ToLower(variant)

		repoName := fmt.Sprintf("%s/common/files/orchestrator/%s", RepositoryName, variantLC)
		if err := TryToCreateECRRepository(ctx, repoName); err != nil {
			fmt.Printf("failed to create ECR repository %s, ignoring: %v\n", repoName, err)
		}

		artifactType := "application/vnd.intel.oep.orchestrator"

		fullPath := fmt.Sprintf("%s/%s:%s", InternalRegistryRepoURL, repoName, tag)
		fmt.Printf("Pushing artifact to %s\n", fullPath)

		if err := pushArtifact(
			ctx,
			InternalRegistryRepoURL,
			repoName,
			tag,
			fileName,
			artifactType,
		); err != nil {
			return fmt.Errorf("failed to push artifact %s: %w", fileName, err)
		}
	}

	fmt.Printf("Successfully published release with tag %s 🎉\n", tag)
	return nil
}

// Publishes the cloud installer release bundle to the registry.
func (Publish) CloudInstaller(ctx context.Context) error {
	defaultRepoVersion, err := os.ReadFile("VERSION")
	if err != nil {
		return fmt.Errorf("failed to read VERSION file: %w", err)
	}
	defaultRepoVersionStr := strings.TrimSpace(string(defaultRepoVersion))

	gitTagName, err := script.Exec("git tag --points-at HEAD").String()
	if err != nil {
		return fmt.Errorf("failed to get git tag name: %w", err)
	}
	gitTagName = strings.TrimSpace(gitTagName)

	branchName, err := script.Exec("git rev-parse --abbrev-ref HEAD").String()
	if err != nil {
		return fmt.Errorf("failed to get branch name: %w", err)
	}
	branchName = strings.TrimSpace(branchName)

	var tag string

	// Set the default tag using the repository version, branch name, and version with 'v' prefix
	tag = fmt.Sprintf("%s,latest,%s,v%s", defaultRepoVersionStr, branchName, defaultRepoVersionStr)

	// If the repository version contains '-dev', modify the tag to include the git short hash and branch-specific tags
	if strings.Contains(defaultRepoVersionStr, "-dev") {
		tag = fmt.Sprintf(
			"%s-%s,latest-%s-dev,v%s-%s",
			defaultRepoVersionStr,
			gitShortHash(),
			branchName,
			defaultRepoVersionStr,
			gitShortHash(),
		)
	}

	// If there is a git tag name, append it to the tag
	if gitTagName != "" {
		tag = fmt.Sprintf("%s,%s,v%s", tag, gitTagName, gitTagName)
	}

	fileName := "_build/" + ReleaseBundleName + ".tgz"
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		return fmt.Errorf("file %s does not exist: %w", fileName, err)
	}

	// Publish the installer image
	installerImageRepo := fmt.Sprintf("%s/common/%s", RepositoryName, InstallerImageName)
	if err := TryToCreateECRRepository(ctx, installerImageRepo); err != nil {
		fmt.Printf("failed to create ECR repository %s, ignoring: %v\n", installerImageRepo, err)
	}

	if err := (Installer{}).Publish(); err != nil {
		return fmt.Errorf("failed to publish installer: %w", err)
	}

	// Publish the installer release bundle
	repoName := fmt.Sprintf("%s/common/files/%s", RepositoryName, ReleaseBundleName)
	if err := TryToCreateECRRepository(ctx, repoName); err != nil {
		fmt.Printf("failed to create ECR repository %s, ignoring: %v\n", repoName, err)
	}

	artifactType := "application/vnd.intel.oep.orchestrator"

	fullPath := fmt.Sprintf("%s/%s:%s", InternalRegistryRepoURL, repoName, tag)
	fmt.Printf("Pushing artifact to %s\n", fullPath)

	if err := pushArtifact(
		ctx,
		InternalRegistryRepoURL,
		repoName,
		tag,
		fileName,
		artifactType,
	); err != nil {
		return fmt.Errorf("failed to push artifact %s: %w", fileName, err)
	}

	fmt.Printf("Successfully published release with tag %s 🎉\n", tag)
	return nil
}

// Publish release manifest files. Valid authentication with push access to the registry is required for this target. Once
// published, the files will be available via the Release service with `oras pull $ARTIFACT_NAME e.g.,
// oras pull registry-rs.edgeorchestration.intel.com/edge-orch/common/files/release-manifest:3.0.0-dev-d5ed6ff
func (Publish) ReleaseManifest(ctx context.Context) error {
	defaultRepoVersion, err := os.ReadFile("VERSION")
	if err != nil {
		return fmt.Errorf("failed to read VERSION file: %w", err)
	}
	defaultRepoVersionStr := strings.TrimSpace(string(defaultRepoVersion))

	branchName, err := script.Exec("git rev-parse --abbrev-ref HEAD").String()
	if err != nil {
		return fmt.Errorf("failed to get branch name: %w", err)
	}
	branchName = strings.TrimSpace(branchName)

	var tag string

	// Set the default tag using the repository version, branch name, and version with 'v' prefix
	tag = fmt.Sprintf("%s,latest,%s,v%s", defaultRepoVersionStr, branchName, defaultRepoVersionStr)

	// If the repository version contains '-dev', modify the tag to include the git short hash and branch-specific tags
	if strings.Contains(defaultRepoVersionStr, "-dev") {
		tag = fmt.Sprintf(
			"%s-%s,latest-%s-dev,v%s-%s",
			defaultRepoVersionStr,
			gitShortHash(),
			branchName,
			defaultRepoVersionStr,
			gitShortHash(),
		)
	}

	// Create ECR repository for sub-project if it does not exist
	manifestRepoName := fmt.Sprintf("%s/%s/files/release-manifest", RepositoryName, RegistryRepoSubProj)
	if err := TryToCreateECRRepository(ctx, manifestRepoName); err != nil {
		fmt.Printf("failed to create ECR repository %s, ignoring: %v\n", manifestRepoName, err)
	}
	var (
		artifactTag  = tag
		artifactName = fmt.Sprintf("%s/release-manifest:%s", InternalFilesRegistry, artifactTag)
	)

	fmt.Println("Pushing to registry:", artifactName)

	matches, err := filepath.Glob(filepath.Join("release-manifest", "*.yaml"))
	if err != nil {
		return fmt.Errorf("failed to list manifest files: %w", err)
	}

	// Strip release-manifest dir from file paths since oras push requires the
	// file name only or the artifact will include the entire dist directory.
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

	// Switch to release-manifest/ directory to flatten the structure
	cmd.Dir = "release-manifest"

	if stdouterr, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to push to registry: %w: %s", err, string(stdouterr))
	}
	fmt.Printf("All release manifest files are pushed to the registry ✅\n")

	return nil
}

func gitShortHash() string {
	hash, _ := script.Exec("git rev-parse --short HEAD").String()
	return strings.TrimSpace(hash)
}

func buildVariant(ctx context.Context, variant string) error {
	switch variant {
	case "cloudFull":
		return Tarball{}.CloudFull()

	case "onpremFull":
		return Tarball{}.OnpremFull()

	default:
		return fmt.Errorf("unknown variant: %s", variant)
	}
}

func pushArtifact(ctx context.Context, registry, repoName, tag, fileName, artifactType string) error {
	if stdouterr, err := exec.CommandContext(
		ctx,
		"oras",
		"push",
		fmt.Sprintf("%s/%s:%s", registry, repoName, tag),
		"--artifact-type", artifactType,
		fileName,
	).CombinedOutput(); err != nil {
		return fmt.Errorf("push image %s/%s:%s: %s: %w", registry, repoName, tag, string(stdouterr), err)
	}

	fmt.Printf("Image %s/%s:%s pushed\n", registry, repoName, tag)
	return nil
}

func TryToCreateECRRepository(ctx context.Context, repositoryName string) error {
	if stdouterr, err := exec.CommandContext(
		ctx,
		"aws",
		"ecr",
		"create-repository",
		"--region", AWSRegion,
		"--repository-name", repositoryName,
	).CombinedOutput(); err != nil {
		if strings.Contains(string(stdouterr), "already exists") {
			return nil
		}
		return fmt.Errorf("failed to create ECR repository %s: %s: %w", repositoryName, string(stdouterr), err)
	}
	return nil
}
