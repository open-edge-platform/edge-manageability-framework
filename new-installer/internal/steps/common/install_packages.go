// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps_common

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	"go.uber.org/zap"
)

const (
	PythonVenvPath  = ".deploy/venv"
	SshuttleVersion = "1.3.1"
)

type InstallPackagesStep struct {
	ShellUtility steps.ShellUtility
	rootPath     string
	logger       *zap.SugaredLogger
}

var installPackageStepLabels = []string{"common", "install-packages"}

func CreateInstallPackagesStep(rootPath string, shellUtility steps.ShellUtility) *InstallPackagesStep {

	return &InstallPackagesStep{
		ShellUtility: shellUtility,
		rootPath:     rootPath,
		logger:       internal.Logger(),
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
	if s.ShellUtility == nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  "Shell utility is not initialized.",
		}
	}
	if !commandExists(ctx, s.ShellUtility, "sudo") {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  "sudo command is not available. Please install sudo.",
		}
	}
	if !commandExists(ctx, s.ShellUtility, "python3") {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  "apt-get command is not available.",
		}
	}
	return runtimeState, nil
}

func (s *InstallPackagesStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if err := s.initPythonVenvDir(ctx); err != nil {
		return runtimeState, err
	}
	if err := s.installSshuttle(ctx); err != nil {
		return runtimeState, err
	}
	return runtimeState, nil
}

func (s *InstallPackagesStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

func (s *InstallPackagesStep) initPythonVenvDir(ctx context.Context) *internal.OrchInstallerError {
	// Create the Python virtual environment directory if it doesn't exist
	venvPath := filepath.Join(s.rootPath, PythonVenvPath)
	_, err := os.Stat(venvPath)
	if os.IsNotExist(err) {
		s.logger.Infof("Creating Python virtual environment at %s", venvPath)
		_, venvErr := s.ShellUtility.Run(ctx, steps.ShellUtilityInput{
			Command:         []string{"python3", "-m", "venv", venvPath},
			Timeout:         60,
			SkipError:       false,
			RunInBackground: false,
		})
		if venvErr != nil {
			return &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorMsg:  fmt.Sprintf("Failed to create Python virtual environment: %v", err),
			}
		}
	}
	return nil
}

func (s *InstallPackagesStep) installSshuttle(ctx context.Context) *internal.OrchInstallerError {
	s.logger.Infof("installing sshuttle to %s", filepath.Join(s.rootPath, PythonVenvPath))
	script := fmt.Sprintf(`source %s/bin/activate && pip3 install sshuttle==%s`, filepath.Join(s.rootPath, PythonVenvPath), SshuttleVersion)
	_, err := s.ShellUtility.Run(ctx, steps.ShellUtilityInput{
		Command:         []string{"bash", "-c", script},
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
	s.logger.Info("sshuttle installed successfully")
	return nil
}
