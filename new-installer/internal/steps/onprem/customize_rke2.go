// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package onprem

import (
	"context"
	"fmt"
	"io"
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

		for _, entry := range [][]string{
			{"audit-policy.yaml", "/etc/rancher/rke2", "audit-policy.yaml"},
			{"rke2-config.yaml", "/etc/rancher/rke2", "config.yaml"},
			{"rke2-coredns-config.yaml", "/var/lib/rancher/rke2/server/manifests", "rke2-coredns-config.yaml"},
		} {
			if err := copyAssetFile(s.AssetsDir, entry[0], entry[1], entry[2]); err != nil {
				return runtimeState, &internal.OrchInstallerError{
					ErrorMsg:  fmt.Sprintf("failed install %s into %s: %s", entry[0], entry[1], err),
					ErrorCode: internal.OrchInstallerErrorCodeInternal,
				}
			}
		}
	}

	return runtimeState, nil
}

func (s *CustomizeRKE2Step) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, prevStepError
}

func copyAssetFile(assetsDir, srcFile, destDir, destFile string) error {
	srcPath := filepath.Join(assetsDir, srcFile)
	if _, err := os.Stat(srcPath); err != nil {
		return fmt.Errorf("file does not exist at path: %s", srcPath)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destDir, err)
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", srcPath, err)
	}
	defer src.Close()

	destPath := filepath.Join(destDir, destFile)
	dest, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", destPath, err)
	}
	defer dest.Close()

	if _, err := io.Copy(dest, src); err != nil {
		return fmt.Errorf("failed to copy %s to %s: %w", srcFile, destPath, err)
	}

	fmt.Printf("Copied %s to %s\n", srcFile, destPath)
	return nil
}
