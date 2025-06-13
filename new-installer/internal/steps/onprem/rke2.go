// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package onprem

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"os/user"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
)

const (
	rke2Version          = "v1.30.10+rke2r1"
	rke2Binary           = "rke2.linux-amd64.tar.gz"
	rke2ImagesPkg        = "rke2-images.linux-amd64.tar.zst"
	rke2CalicoImagePkg   = "rke2-images-calico.linux-amd64.tar.zst"
	rke2LibSHAFile       = "sha256sum-amd64.txt"
	rke2DownloadBasePath = "https://github.com/rancher/rke2/releases/download"
)

type Rke2Step struct {
	RootPath               string
	KeepGeneratedFiles     bool
	orchConfigReaderWriter config.OrchConfigReaderWriter
	StepLabels             []string
}

func CreateRke2Step(rootPath string, keepGeneratedFiles bool, orchConfigReaderWriter config.OrchConfigReaderWriter) *Rke2Step {
	return &Rke2Step{
		RootPath:               rootPath,
		KeepGeneratedFiles:     keepGeneratedFiles,
		orchConfigReaderWriter: orchConfigReaderWriter,
	}
}

func (s *Rke2Step) Name() string {
	return "Rke2InfraStep"
}

func (s *Rke2Step) Labels() []string {
	return s.StepLabels
}

func (s *Rke2Step) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

func (s *Rke2Step) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	// no-op for now
	fmt.Println("PreStep for Rke2Step is a no-op")
	return runtimeState, nil
}

func (s *Rke2Step) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if runtimeState.Action == "install" {
		fmt.Println("Running RKE2 installation step")

		var dockerUsername, dockerPassword string
		var err error
		dockerUsername = config.Onprem.DockerUsername
		dockerPassword = config.Onprem.DockerToken
		currentUser, err := user.Current()
		if err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorMsg:  fmt.Sprintf("failed to get current user: %s", err),
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
			}
		}

		// if err := downloadRKE2(); err != nil {
		// 	return runtimeState, &internal.OrchInstallerError{
		// 		ErrorMsg:  fmt.Sprintf("failed to download RKE2: %s", err),
		// 		ErrorCode: internal.OrchInstallerErrorCodeInternal,
		// 	}
		// }
		// fmt.Println("RKE2 images and install script downloaded successfully")

		if err := installRKE2(INSTALLERS_DIR, dockerUsername, dockerPassword, currentUser.Username); err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorMsg:  fmt.Sprintf("failed to install RKE2: %s", err),
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
			}
		}
		fmt.Println("RKE2 installation completed successfully")
	}
	return runtimeState, nil
}

func (s *Rke2Step) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, prevStepError
}

func downloadRKE2() error {
	fmt.Println("Downloading RKE2 images and install script...")
	imagesDir := "./installers/"
	if err := os.MkdirAll(imagesDir, 0o755); err != nil {
		return fmt.Errorf("failed to create images directory: %w", err)
	}

	rke2VersionEscaped := url.QueryEscape(rke2Version)

	for _, image := range []string{
		rke2Binary,
		rke2ImagesPkg,
		rke2CalicoImagePkg,
		rke2LibSHAFile,
	} {
		rke2DownloadURL := fmt.Sprintf("%s/%s/%s", rke2DownloadBasePath, rke2VersionEscaped, image)
		fmt.Printf("Downloading RKE2 from %s\n", rke2DownloadURL)
		cmd := exec.Command("curl", "-L", rke2DownloadURL, "-o", fmt.Sprintf("%s/%s", imagesDir, image))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to download RKE2 image %s: %w", image, err)
		}
	}

	// Download the RKE2 install script
	installScriptURL := "https://get.rke2.io"
	fmt.Printf("Downloading RKE2 install script from %s\n", installScriptURL)
	cmd := exec.Command("curl", "-sfL", installScriptURL, "-o", fmt.Sprintf("%s/%s", imagesDir, "install.sh"))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to download install script: %w", err)
	}

	fmt.Println("RKE2 images and install script downloaded successfully")
	return nil
}

func installRKE2(debDirName, dockerUsername, dockerPassword, currentUser string) error {
	fmt.Println("Installing RKE2...")
	var cmd *exec.Cmd
	if dockerUsername != "" && dockerPassword != "" {
		fmt.Println("Docker credentials provided. Installing RKE2 with Docker credentials")
		cmd = exec.Command("sudo", "env",
			fmt.Sprintf("DOCKER_USERNAME=%s", dockerUsername),
			fmt.Sprintf("DOCKER_PASSWORD=%s", dockerPassword),
			"NEEDRESTART_MODE=a", "DEBIAN_FRONTEND=noninteractive",
			"apt-get", "install", "-y",
			fmt.Sprintf("%s/onprem-ke-installer_%s_amd64.deb", debDirName, ORCH_VERSION),
		)
	} else {
		cmd = exec.Command("sudo",
			"NEEDRESTART_MODE=a", "DEBIAN_FRONTEND=noninteractive",
			"apt-get", "install", "-y",
			fmt.Sprintf("%s/onprem-ke-installer_%s_amd64.deb", debDirName, ORCH_VERSION),
		)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install RKE2: %w", err)
	}
	fmt.Println("OS level configuration installed and RKE2 Installed")

	kubeDir := fmt.Sprintf("/home/%s/.kube", currentUser)
	if err := os.MkdirAll(kubeDir, 0o700); err != nil {
		return fmt.Errorf("failed to create kube dir: %w", err)
	}
	if err := exec.Command("sudo", "cp", "/etc/rancher/rke2/rke2.yaml", fmt.Sprintf("%s/config", kubeDir)).Run(); err != nil {
		return fmt.Errorf("failed to copy kube config: %w", err)
	}
	if err := exec.Command("sudo", "chown", "-R", fmt.Sprintf("%s:%s", currentUser, currentUser), kubeDir).Run(); err != nil {
		return fmt.Errorf("failed to chown kube dir: %w", err)
	}
	if err := exec.Command("sudo", "chmod", "600", fmt.Sprintf("%s/config", kubeDir)).Run(); err != nil {
		return fmt.Errorf("failed to chmod kube config: %w", err)
	}
	os.Setenv("KUBECONFIG", fmt.Sprintf("/home/%s/.kube/config", currentUser))
	return nil
}
