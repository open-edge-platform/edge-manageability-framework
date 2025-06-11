// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps_common

import (
	"context"
	"runtime"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
)

type InstallPackagesStep struct {
	ShellUtility steps.ShellUtility
}

var installPackageStepLabels = []string{"common", "install-packages"}

func CreateInstallPackagesStep(shellUtility steps.ShellUtility) *InstallPackagesStep {
	return &InstallPackagesStep{
		ShellUtility: shellUtility,
	}
}

func (s *InstallPackagesStep) Name() string {
	return "InstallPackagesStep"
}

func (s *InstallPackagesStep) Labels() []string {
	return installPackageStepLabels
}

func (s *InstallPackagesStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

func (s *InstallPackagesStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	os := runtime.GOOS
	if os != "linux" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  "Unsupported operating system. Only Linux is supported.",
		}
	}
	if s.ShellUtility == nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  "Shell utility is not initialized.",
		}
	}

	// Check if sudo exists
	if !commandExists(ctx, s.ShellUtility, "sudo") {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  "sudo command is not available. Please install sudo.",
		}
	}
	// Check if apt-get exists
	if !commandExists(ctx, s.ShellUtility, "apt-get") {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  "apt-get command is not available.",
		}
	}
	return runtimeState, nil
}

func (s *InstallPackagesStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, s.installSshuttle(ctx)
}

func (s *InstallPackagesStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

func (s *InstallPackagesStep) installSshuttle(ctx context.Context) *internal.OrchInstallerError {
	logger := internal.Logger()
	_, err := s.ShellUtility.Run(ctx, steps.ShellUtilityInput{
		Command:         []string{"which", "sshuttle"},
		Timeout:         60,
		SkipError:       false,
		RunInBackground: false,
	})
	if err == nil {
		return nil // sshuttle is already installed
	}
	_, err = s.ShellUtility.Run(ctx, steps.ShellUtilityInput{
		Command:         []string{"sudo", "apt-get", "install", "-y", "sshuttle"},
		Timeout:         60,
		SkipError:       false,
		RunInBackground: false,
	})
	if err != nil {
		return &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  "Failed to install sshuttle using apt-get",
		}
	}
	logger.Info("sshuttle installed successfully")
	return nil
}
