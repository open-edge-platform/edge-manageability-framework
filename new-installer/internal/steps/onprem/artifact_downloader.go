// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package onprem

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	// oras "oras.land/oras-go/v2"
	// "oras.land/oras-go/v2/content/file"
	// "oras.land/oras-go/v2/registry/remote"
)

const (
	RS_URL             = "registry-rs.edgeorchestration.intel.com"
	INSTALLERS_RS_PATH = "edge-orch/common/files"
	ARCHIVES_RS_PATH   = "edge-orch/common/files/orchestrator"
	ORCH_VERSION       = "3.1.0-dev-eca1939"
	INSTALLERS_DIR     = "/tmp/installers"

	rke2Version        = "v1.30.10+rke2r1"
	rke2Binary         = "rke2.linux-amd64.tar.gz"
	rke2ImagesPkg      = "rke2-images.linux-amd64.tar.zst"
	rke2CalicoImagePkg = "rke2-images-calico.linux-amd64.tar.zst"
	rke2LibSHAFile     = "sha256sum-amd64.txt"
	rke2ImagesUrl      = "https://github.com/rancher/rke2/releases/download"
	rke2InstallerUrl   = "https://get.rke2.io"
	rke2ImagesDir      = "/var/lib/rancher/rke2/agent/images"
)

var installerList = []string{
	"onprem-ke-installer",
	"onprem-orch-installer",
}

type ArtifactDownloader struct {
	RootPath               string
	KeepGeneratedFiles     bool
	orchConfigReaderWriter config.OrchConfigReaderWriter
	StepLabels             []string
}

func CreateArtifactDownloader(rootPath string, keepGeneratedFiles bool, orchConfigReaderWriter config.OrchConfigReaderWriter) *ArtifactDownloader {
	return &ArtifactDownloader{
		RootPath:               rootPath,
		KeepGeneratedFiles:     keepGeneratedFiles,
		orchConfigReaderWriter: orchConfigReaderWriter,
	}
}

func (s *ArtifactDownloader) Name() string {
	return "ArtifactDownloader"
}

func (s *ArtifactDownloader) Labels() []string {
	return s.StepLabels
}

func (s *ArtifactDownloader) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

func (s *ArtifactDownloader) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	// no-op for now
	return runtimeState, nil
}

func (s *ArtifactDownloader) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if runtimeState.Action == "install" {
		fmt.Println("Running ArtifactDownloader step")

		// Create directories for installers and archives
		if err := os.MkdirAll(INSTALLERS_DIR, 0o755); err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorMsg:  fmt.Sprintf("failed to create installers directory %s: %s", INSTALLERS_DIR, err),
			}
		}

		fmt.Println("Created directories for installers")

		if err := downloadImages(ctx, INSTALLERS_DIR); err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorMsg:  fmt.Sprintf("failed to download RKE2 artifacts: %s", err),
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
			}
		}

		fmt.Println("RKE2 images and install script downloaded successfully")

		if err := createImagesDir(rke2ImagesDir); err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorMsg:  fmt.Sprintf("failed to create RKE2 images dir %s: %s", rke2ImagesDir, err),
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
			}
		}

		fmt.Println("RKE2 images directory created successfully")

		if err := copyImages(ctx, INSTALLERS_DIR, rke2ImagesDir); err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorMsg:  fmt.Sprintf("failed to copy RKE2 images to %s: %s", rke2ImagesDir, err),
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
			}
		}

		fmt.Println("RKE2 images copied successfully")

		// if err := downloadArtifacts(ctx, RS_URL, INSTALLERS_RS_PATH, ORCH_VERSION, INSTALLERS_DIR, installerList); err != nil {
		// 	return runtimeState, &internal.OrchInstallerError{
		// 		ErrorCode: internal.OrchInstallerErrorCodeInternal,
		// 		ErrorMsg:  fmt.Sprintf("failed to download installers: %s", err),
		// 	}
		// }

		// fmt.Println("Downloaded installers successfully")
	}

	return runtimeState, nil
}

func (s *ArtifactDownloader) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, prevStepError
}

// func downloadArtifacts(ctx context.Context, registryUrl, registryPath, orchVersion, artifactDir string, artifactList []string) error {
// 	fileStore, err := file.New(artifactDir)
// 	if err != nil {
// 		return fmt.Errorf("failed to create file store: %w", err)
// 	}
// 	defer fileStore.Close()

// 	for _, artifact := range artifactList {
// 		fmt.Println("downloading artifact: " + artifact)
// 		repo, err := remote.NewRepository(registryUrl + "/" + registryPath + "/" + artifact)
// 		if err != nil {
// 			return fmt.Errorf("failed to create repository for %s: %w", artifact, err)
// 		}

// 		manifestDescriptor, err := oras.Copy(ctx, repo, orchVersion, fileStore, orchVersion, oras.DefaultCopyOptions)
// 		if err != nil {
// 			return fmt.Errorf("failed to copy artifact %s: %w", artifact, err)
// 		}
// 		fmt.Println("manifest descriptor:", manifestDescriptor)
// 	}
// 	return nil
// }

func downloadImages(ctx context.Context, artifactDir string) error {
	fmt.Println("Downloading RKE2 images and install script...")
	rke2VersionEscaped := url.QueryEscape(rke2Version)

	// Download the RKE2 images and binaries
	for _, image := range []string{
		rke2Binary,
		rke2ImagesPkg,
		rke2CalicoImagePkg,
		rke2LibSHAFile,
	} {
		rke2DownloadURL := fmt.Sprintf("%s/%s/%s", rke2ImagesUrl, rke2VersionEscaped, image)
		fmt.Printf("Downloading RKE2 from %s\n", rke2DownloadURL)
		cmd := exec.CommandContext(ctx, "curl", "-L", rke2DownloadURL, "-o", fmt.Sprintf("%s/%s", artifactDir, image))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to download RKE2 image %s: %w", image, err)
		}
	}

	// Download the RKE2 install script
	fmt.Printf("Downloading RKE2 install script from %s\n", rke2InstallerUrl)
	cmd := exec.CommandContext(ctx, "curl", "-sfL", rke2InstallerUrl, "-o", fmt.Sprintf("%s/%s", artifactDir, "install.sh"))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to download install script: %w", err)
	}

	return nil
}

func createImagesDir(imagesDir string) error {

	// Create the images directory if it doesn't exist
	if err := os.MkdirAll(imagesDir, 0o755); err != nil {
		return fmt.Errorf("failed to create images directory: %w", err)
	}

	return nil
}

func copyImages(ctx context.Context, source, destination string) error {

	// List of images to copy
	for _, image := range []string{
		rke2ImagesPkg,
		rke2CalicoImagePkg,
	} {
		src := fmt.Sprintf("%s/%s", source, image)
		dst := fmt.Sprintf("%s/%s", destination, image)

		input, err := os.Open(src)
		if err != nil {
			return fmt.Errorf("failed to open source image %s: %w", src, err)
		}
		defer input.Close()

		output, err := os.Create(dst)
		if err != nil {
			return fmt.Errorf("failed to create destination image %s: %w", dst, err)
		}
		defer output.Close()

		if _, err := io.Copy(output, input); err != nil {
			return fmt.Errorf("failed to copy image from %s to %s: %w", src, dst, err)
		}
	}

	return nil
}