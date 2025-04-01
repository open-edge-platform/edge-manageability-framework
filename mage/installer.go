// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/magefile/mage/sh"
)

var targets = []string{"cloudFull"}

func getVersionRevision(version string) (string, error) {
	if strings.Contains(version, "-dev") {
		c, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
		if err != nil {
			return "", err
		}
		commit := strings.TrimSpace(string(c))
		return fmt.Sprintf("%s-%s", version, commit), nil
	}
	return version, nil
}

func getVersionTags(version string) ([]string, error) {
	var tags []string

	versionRevision, err := getVersionRevision(version)
	if err != nil {
		return []string{}, err
	}
	tags = append(tags, fmt.Sprintf("v%s", versionRevision))
	tags = append(tags, fmt.Sprintf("v%s-latest", version))

	return tags, nil
}

func getFullImageNames(target string) ([]string, error) {
	repository := os.Getenv("DOCKER_REPOSITORY")
	registry := os.Getenv("DOCKER_REGISTRY")
	var registryPath string
	if repository == "" && registry == "" {
		registryPath = IntelTiberContainerRegistry
	} else {
		registryPath = path.Join(registry, repository)
	}

	version, err := getVersionFromFile()
	if err != nil {
		return []string{}, err
	}

	imageTags, err := getVersionTags(version)
	if err != nil {
		return []string{}, err
	}

	imageNames := []string{}
	for _, tag := range imageTags {
		imageName := fmt.Sprintf("orchestrator-installer-%s:%s", strings.ToLower(target), tag)
		fullImageName := path.Join(registryPath, imageName)
		imageNames = append(imageNames, fullImageName)
	}

	return imageNames, nil
}

func buildDockerImage(target string, podConfigTarball string) error {
	http_proxy := os.Getenv("http_proxy")
	if http_proxy == "" {
		http_proxy = os.Getenv("HTTP_PROXY")
	}
	https_proxy := os.Getenv("https_proxy")
	if https_proxy == "" {
		https_proxy = os.Getenv("HTTPS_PROXY")
	}
	no_proxy := os.Getenv("no_proxy")
	if no_proxy == "" {
		no_proxy = os.Getenv("NO_PROXY")
	}

	version, err := getVersionFromFile()
	if err != nil {
		return fmt.Errorf("failed to parse VERSION file: %w", err)
	}
	deployTarball := fmt.Sprintf("%s_edge-manageability-framework_%s.tgz", target, version)

	imageNames, err := getFullImageNames(target)
	if err != nil {
		return fmt.Errorf("failed to get full installer image names: %w", err)
	}
	imageNameArgs := []string{}
	for _, name := range imageNames {
		imageNameArgs = append(imageNameArgs, "-t", name)
	}

	dockerArgs := []string{
		"build",
		"--build-arg", "http_proxy=" + http_proxy,
		"--build-arg", "https_proxy=" + https_proxy,
		"--build-arg", "no_proxy=" + no_proxy,
		"--build-arg", "DEPLOY_TARBALL=" + deployTarball,
		"--build-arg", "POD_CONFIGS_TARBALL=" + podConfigTarball,
		"--build-arg", "DEPLOY_TYPE=" + target,
	}
	dockerArgs = append(dockerArgs, imageNameArgs...)
	dockerArgs = append(dockerArgs, "installer")

	if err := sh.RunV("docker", dockerArgs...); err != nil {
		return fmt.Errorf("failed to build installer image: %w", err)
	}

	return nil
}

func cleanDockerImages(target string) error {
	imageNames, err := getFullImageNames(target)
	if err != nil {
		return fmt.Errorf("could not get full docker image names: %w", err)
	}

	for _, imageName := range imageNames {
		fmt.Println("docker rmi " + imageName)
		if err := sh.RunV("docker", "rmi", imageName); err != nil {
			fmt.Println("warning: error removing image. skipping...")
		}
	}
	return nil
}

func publishDockerImage(target string) error {
	imageNames, err := getFullImageNames(target)
	if err != nil {
		return fmt.Errorf("could not get full docker image names: %w", err)
	}

	for _, imageName := range imageNames {
		fmt.Println("docker push " + imageName)
		if err := sh.RunV("docker", "push", imageName); err != nil {
			return fmt.Errorf("failed to push docker images: %w", err)
		}
	}
	return nil
}

// Build the Cloud Installer images.
func (Installer) build() error {
	var t Tarball

	fmt.Println("Building Installer images")
	podConfigsDir := "./pod-configs"

	// Create tarballs for edge-manageability-framework, orch-configs for each deploy type
	os.Setenv("TARBALL_DIR", "installer/")

	if err := t.CloudFull(); err != nil {
		return fmt.Errorf("failed to build cloudFull tarball: %w", err)
	}

	// TBD: Refactor/simplify pod-configs inclusion with the compbined repo
	// Create tarball for pod-configs
	if err := sh.RunV("make", "-C", podConfigsDir, "package"); err != nil {
		return fmt.Errorf("failed to build pod-configs deployment package: %w", err)
	}

	version, err := getVersionFromFile()
	if err != nil {
		return fmt.Errorf("failed to parse VERSION file: %w", err)
	}
	versionRevision, err := getVersionRevision(version)
	if err != nil {
		return fmt.Errorf("failed to get revision: %w", err)
	}

	filename := fmt.Sprintf("emf-pod-configs-%s.tar.gz", versionRevision)
	if err := sh.RunV("cp", "-v", path.Join(podConfigsDir, filename), "installer/"); err != nil {
		return fmt.Errorf("unable to copy pod-configs deployment package: %w", err)
	}

	for _, target := range targets {
		if err := buildDockerImage(target, filename); err != nil {
			return fmt.Errorf("failed to build cloud-installer image: %w", err)
		}
	}
	return nil
}

// Clean Cloud Installer images.
func (Installer) clean() error {
	for _, target := range targets {
		if err := cleanDockerImages(target); err != nil {
			return fmt.Errorf("failed to clean release images: %w", err)
		}
	}
	return nil
}

func (Installer) publish() error {
	fmt.Println("Publishing Installer images")

	for _, target := range targets {
		if err := publishDockerImage(target); err != nil {
			return fmt.Errorf("failed to publish release images: %w", err)
		}
	}
	return nil
}

const (
	InstallerImageName = "orchestrator-installer-cloudfull"
	ReleaseBundleName  = "cloud-orchestrator-installer"
)

func (i Installer) bundle() error {
	fmt.Println("Bundling Installer images")

	repository := os.Getenv("DOCKER_REPOSITORY")
	registry := os.Getenv("DOCKER_REGISTRY")
	var registryPath string
	if repository == "" && registry == "" {
		registryPath = IntelTiberContainerRegistry
	} else {
		registryPath = path.Join(registry, repository)
	}

	version, err := getVersionFromFile()
	if err != nil {
		return fmt.Errorf("failed to parse VERSION file: %w", err)
	}

	versionRevision, err := getVersionRevision(version)
	if err != nil {
		return fmt.Errorf("failed to get revision: %w", err)
	}
	versionTag := fmt.Sprintf("v%s", versionRevision)

	// Build the release bundle tarball
	buildDir := path.Join(getDeployDir(), "_build", ReleaseBundleName)
	if err := os.RemoveAll(buildDir); err != nil {
		return fmt.Errorf("failed to clean deployment build directory: %w", err)
	}
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return fmt.Errorf("failed to create deployment build directory: %w", err)
	}

	buildImage := path.Join(getDeployDir(), "_build", ReleaseBundleName+".tgz")
	fmt.Printf("build %s from %s\n", buildImage, buildDir)

	if err := sh.RunV("cp", path.Join(getDeployDir(), "tools/deploy", "doc/README.md"), buildDir); err != nil {
		return fmt.Errorf("failed to bundle README file: %w", err)
	}

	if err := sh.RunV("cp", path.Join(getDeployDir(), "tools/deploy", "start-orchestrator-install.sh"), buildDir); err != nil {
		return fmt.Errorf("failed to bundle cloud installer startup script: %w", err)
	}

	bundleVersionFile := path.Join(buildDir, "VERSION")
	if err := os.WriteFile(bundleVersionFile, []byte(versionTag), 0o644); err != nil {
		return fmt.Errorf("failed to bundle VERSION file: %w", err)
	}

	publishImage := path.Join(registryPath, InstallerImageName+":"+versionTag)
	publicImage := path.Join(PublicTiberContainerRegistry, InstallerImageName+":"+versionTag)
	fmt.Println("install image:")
	fmt.Printf("  - publish : %s\n", publishImage)
	fmt.Printf("  = public  : %s\n", publicImage)

	// Verify the publish image exists
	if err := sh.RunV("docker", "inspect", publishImage); err != nil {
		fmt.Println("warning: installer image not found. building...")
		err = i.build()
		if err != nil {
			return fmt.Errorf("failed to build cloud installer image: %w", err)
		}
	}

	if err := sh.RunV("docker", "tag", publishImage, publicImage); err != nil {
		return fmt.Errorf("failed to tag cloud installer image: %w", err)
	}
	fmt.Println("exporting installer images...")
	if err := sh.RunV("docker", "save", "-o", path.Join(buildDir, InstallerImageName+".tar"), publicImage); err != nil {
		return fmt.Errorf("failed to export cloud installer image: %w", err)
	}

	// Clean up the build time public image retag
	if err := sh.RunV("docker", "rmi", publicImage); err != nil {
		return fmt.Errorf("failed to clean exported cloud installer image: %w", err)
	}

	fmt.Printf("building release bundle: %s.tgz\n", ReleaseBundleName)
	if err := sh.RunV("tar", "-czvf", buildImage, "-C", path.Join(getDeployDir(), "_build"), ReleaseBundleName); err != nil {
		return fmt.Errorf("failed to create cloud installer release bundle: %w", err)
	}

	return nil
}
