// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package onprem

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
)

const (
	rsURL            = "registry-rs.edgeorchestration.intel.com"
	installersRSPath = "edge-orch/common/files"
	archivesRSPath   = "edge-orch/common/files/orchestrator"
	orchVersion      = "3.1.0-dev-eca1939"
	installersDir    = "/tmp/installers"

	rke2Version            = "v1.30.10+rke2r1"
	rke2BinaryFile         = "rke2.linux-amd64.tar.gz"
	rke2ImagesPackage      = "rke2-images.linux-amd64.tar.zst"
	rke2CalicoImagePackage = "rke2-images-calico.linux-amd64.tar.zst"
	rke2LibSHAFile         = "sha256sum-amd64.txt"
	rke2ImagesURL          = "https://github.com/rancher/rke2/releases/download"
	rke2InstallerURL       = "https://get.rke2.io"
)

var installerList = []string{
	"onprem-ke-installer",
	"onprem-orch-installer",
}

type RKE2DownloadStep struct {
	RootPath               string
	KeepGeneratedFiles     bool
	OrchConfigReaderWriter config.OrchConfigReaderWriter
	StepLabels             []string
}

func CreateRKE2DownloadStep(rootPath string, keepGeneratedFiles bool, orchConfigReaderWriter config.OrchConfigReaderWriter) *RKE2DownloadStep {
	return &RKE2DownloadStep{
		RootPath:               rootPath,
		KeepGeneratedFiles:     keepGeneratedFiles,
		OrchConfigReaderWriter: orchConfigReaderWriter,
	}
}

func (s *RKE2DownloadStep) Name() string {
	return "DownloadRKE2Step"
}

func (s *RKE2DownloadStep) Labels() []string {
	return s.StepLabels
}

func (s *RKE2DownloadStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

func (s *RKE2DownloadStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if runtimeState.Action == "install" {
		fmt.Printf("Running %s pre-step\n", s.Name())

		// Create directories for installers and archives
		if err := os.MkdirAll(installersDir, 0o755); err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorMsg:  fmt.Sprintf("failed to create installers directory %s: %s", installersDir, err),
			}
		}

		fmt.Printf("Created directories for installers (%s)\n", installersDir)

		// Create additional directories if needed
		if err := os.MkdirAll(archivesRSPath, 0o755); err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorMsg:  fmt.Sprintf("failed to create archives directory %s: %s", archivesRSPath, err),
			}
		}

		fmt.Printf("Created directories for archives (%s)\n", archivesRSPath)
	}

	return runtimeState, nil
}

func (s *RKE2DownloadStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if runtimeState.Action == "install" {
		fmt.Printf("Running %s run-step\n", s.Name())

		if err := downloadRKE2Images(ctx, installersDir); err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorMsg:  fmt.Sprintf("failed to download RKE2 artifacts: %s", err),
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
			}
		}

		fmt.Println("RKE2 images and install script downloaded successfully")

		if err := downloadArtifacts(ctx, rsURL, installersRSPath, orchVersion, installersDir, installerList); err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorMsg:  fmt.Sprintf("failed to download installers: %s", err),
			}
		}

		fmt.Println("Downloaded installers successfully")
	}

	return runtimeState, nil
}

func (s *RKE2DownloadStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if runtimeState.Action == "install" {
		fmt.Printf("Running %s post-step\n", s.Name())

	}

	return runtimeState, prevStepError
}

func downloadArtifacts(ctx context.Context, registryUrl, registryPath, orchVersion, artifactDir string, artifactList []string) error {
	fileStore, err := file.New(artifactDir)
	if err != nil {
		return fmt.Errorf("failed to create file store: %w", err)
	}
	defer fileStore.Close()

	for _, artifact := range artifactList {
		fmt.Println("downloading artifact: " + artifact)
		repo, err := remote.NewRepository(registryUrl + "/" + registryPath + "/" + artifact)
		if err != nil {
			return fmt.Errorf("failed to create repository for %s: %w", artifact, err)
		}

		manifestDescriptor, err := oras.Copy(ctx, repo, orchVersion, fileStore, orchVersion, oras.DefaultCopyOptions)
		if err != nil {
			return fmt.Errorf("failed to copy artifact %s: %w", artifact, err)
		}
		fmt.Println("manifest descriptor:", manifestDescriptor)
	}
	return nil
}

func downloadRKE2Images(ctx context.Context, artifactDir string) error {
	fmt.Println("Downloading RKE2 images and install script...")

	if _, err := exec.LookPath("curl"); err != nil {
		return fmt.Errorf("curl is not installed or not found in PATH: %w", err)
	}

	for _, image := range []string{
		rke2BinaryFile,
		rke2ImagesPackage,
		rke2CalicoImagePackage,
		rke2LibSHAFile,
	} {
		url := fmt.Sprintf("%s/%s/%s", rke2ImagesURL, rke2Version, image)
		out := filepath.Join(artifactDir, image)
		fmt.Printf("Downloading %s\n", url)
		if err := exec.CommandContext(ctx, "curl", "-L", url, "-o", out).Run(); err != nil {
			return fmt.Errorf("failed to download %s: %w", image, err)
		}
	}

	installScript := filepath.Join(artifactDir, "rke2-install.sh")
	if err := exec.CommandContext(ctx, "curl", "-sfL", rke2InstallerURL, "-o", installScript).Run(); err != nil {
		return fmt.Errorf("failed to download install script: %w", err)
	}

	return nil
}
