// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package onprem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
)

type CustomizeRKE2Step struct {
	RootPath               string
	KeepGeneratedFiles     bool
	orchConfigReaderWriter config.OrchConfigReaderWriter
	AssetsDir              string
	StepLabels             []string
}

func CreateCustomizeRKE2Step(rootPath string, keepGeneratedFiles bool, orchConfigReaderWriter config.OrchConfigReaderWriter, assetsDir string) *CustomizeRKE2Step {
	return &CustomizeRKE2Step{
		RootPath:               rootPath,
		KeepGeneratedFiles:     keepGeneratedFiles,
		orchConfigReaderWriter: orchConfigReaderWriter,
		AssetsDir:              assetsDir,
	}
}

func (s *CustomizeRKE2Step) Name() string {
	return "Rke2InfraStep"
}

func (s *CustomizeRKE2Step) Labels() []string {
	return s.StepLabels
}

func (s *CustomizeRKE2Step) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

func (s *CustomizeRKE2Step) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

func (s *CustomizeRKE2Step) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if runtimeState.Action == "install" {
		// Perform customization for RKE2 installation
		// This could include copying configuration files, setting up directories, etc.
		// For now, we will just print a message indicating the step is running
		fmt.Println("Running RKE2 customization step")

		if err := copyAssetFile(s.AssetsDir, "audit-policy.yaml", "/etc/rancher/rke2"); err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorMsg:  fmt.Sprintf("failed install audit-policy.yaml into /etc/rancher/rke2: %s", err),
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
			}
		}
	}

	return runtimeState, nil
}

func (s *CustomizeRKE2Step) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, prevStepError
}

func copyAssetFile(assetsDir, file, destDir string) error {
	path := filepath.Join(assetsDir, file)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist at path: %s", path)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %s", destDir, err)
	}

	destPath := filepath.Join(destDir, file)
	if err := os.Rename(path, destPath); err != nil {
		return fmt.Errorf("failed to copy asset file %s to %s: %s", file, destDir, err)
	}
	fmt.Printf("successfully copied to %s to %s\n", file, destDir)

	return nil
}
