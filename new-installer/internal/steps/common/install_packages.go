// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps_common

import (
	"context"
	"path/filepath"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	"go.uber.org/zap"
)

const (
	KubectlVersion    = "v1.32.5"
	DefaultBinaryPath = ".deploy/bin"
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
	return runtimeState, nil
}

func (s *InstallPackagesStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, s.installKubectl(ctx)
}

func (s *InstallPackagesStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

func (s *InstallPackagesStep) installKubectl(ctx context.Context) *internal.OrchInstallerError {
	url := "https://dl.k8s.io/release/" + KubectlVersion + "/bin/linux/amd64/kubectl"
	outputPath := filepath.Join(s.rootPath, DefaultBinaryPath, "kubectl")
	input := steps.ShellUtilityInput{
		Command: []string{"curl", "-Lo", outputPath, url},
		Timeout: 30 * 60, // 30 minutes
	}
	if _, err := s.ShellUtility.Run(ctx, input); err != nil {
		return &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  "Failed to download kubectl: " + err.Error(),
		}
	}
	s.logger.Infof("Successfully downloaded kubectl to %s", outputPath)
	return nil
}
