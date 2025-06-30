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
	"text/template"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
)

type RKE2CustomizeStep struct {
	RootPath               string
	KeepGeneratedFiles     bool
	OrchConfigReaderWriter config.OrchConfigReaderWriter
	AssetsDir              string
	StepLabels             []string
}

func CreateRKE2CustomizeStep(rootPath string, keepGeneratedFiles bool, orchConfigReaderWriter config.OrchConfigReaderWriter, assetsDir string) *RKE2CustomizeStep {
	return &RKE2CustomizeStep{
		RootPath:               rootPath,
		KeepGeneratedFiles:     keepGeneratedFiles,
		OrchConfigReaderWriter: orchConfigReaderWriter,
		AssetsDir:              assetsDir,
	}
}

func (s *RKE2CustomizeStep) Name() string {
	return "CustomizeRKE2Step"
}

func (s *RKE2CustomizeStep) Labels() []string {
	return s.StepLabels
}

func (s *RKE2CustomizeStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

func (s *RKE2CustomizeStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

func (s *RKE2CustomizeStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if runtimeState.Action == "install" {
		fmt.Printf("Running %s run-step\n", s.Name())

		// Copy configuration files and render templates for audit policy, RKE2 config, CoreDNS, and registries.
		for _, entry := range [][]string{
			{"audit-policy.yaml", "/etc/rancher/rke2/audit-policy.yaml"},
			{"rke2-config.yaml", "/etc/rancher/rke2/config.yaml"},
			{"rke2-coredns-config.yaml", "/var/lib/rancher/rke2/server/manifests/rke2-coredns-config.yaml"},
			{"rke2-registries.yaml", "/etc/rancher/rke2/registries.yaml"},
		} {
			if err := copyConfig(s.AssetsDir, entry[0], entry[1]); err != nil {
				return runtimeState, &internal.OrchInstallerError{
					ErrorMsg:  fmt.Sprintf("failed install %s into %s: %s", entry[0], entry[1], err),
					ErrorCode: internal.OrchInstallerErrorCodeInternal,
				}
			}
		}

		// Render rke2-coredns-config.yaml with namespace value to avoid Trivy warnings
		if err := renderConfig("/var/lib/rancher/rke2/server/manifests/rke2-coredns-config.yaml", map[string]string{
			"Namespace": "kube-system",
		}); err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorMsg:  fmt.Sprintf("failed to render rke2-coredns-config.yaml: %s", err),
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
			}
		}

		// Render RKE2 registries.yaml with Docker credentials if provided
		if config.Onprem.DockerUsername != "" && config.Onprem.DockerToken != "" {
			// Render the registries.yaml file with the Docker credentials
			if err := renderConfig("/etc/rancher/rke2/registries.yaml", map[string]string{
				"DockerUsername": config.Onprem.DockerUsername,
				"DockerToken":    config.Onprem.DockerToken,
			}); err != nil {
				return runtimeState, &internal.OrchInstallerError{
					ErrorMsg:  fmt.Sprintf("failed to render registries.yaml: %s", err),
					ErrorCode: internal.OrchInstallerErrorCodeInternal,
				}
			}
		}
	}

	return runtimeState, nil
}

func (s *RKE2CustomizeStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, prevStepError
}

func copyConfig(assetsDir, srcFile, destFile string) error {
	srcPath := filepath.Join(assetsDir, srcFile)
	if _, err := os.Stat(srcPath); err != nil {
		return fmt.Errorf("file does not exist at path: %s", srcPath)
	}

	destDir := filepath.Dir(destFile)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destDir, err)
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", srcPath, err)
	}
	defer src.Close()

	dest, err := os.Create(destFile)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", destFile, err)
	}
	defer dest.Close()

	if _, err := io.Copy(dest, src); err != nil {
		return fmt.Errorf("failed to copy %s to %s: %w", srcFile, destFile, err)
	}

	fmt.Printf("Copied %s to %s\n", srcFile, destFile)
	return nil
}

func renderConfig(file string, data any) error {
	// Read file content to check for template delimiters
	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	tmpl, err := template.New(filepath.Base(file)).Parse(string(content))
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	f, err := os.OpenFile(file, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("executing template: %w", err)
	}

	fmt.Printf("Rendered config file in place: %s\n", file)
	return nil
}
