// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package onprem

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
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
