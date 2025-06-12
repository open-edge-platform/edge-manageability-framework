// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package onprem

import (
	"context"
	"fmt"
	"os"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
)

const (
	RS_URL             = "registry-rs.edgeorchestration.intel.com"
	INSTALLERS_RS_PATH = "edge-orch/common/files"
	ARCHIVES_RS_PATH   = "edge-orch/common/files/orchestrator"
	ORCH_VERSION       = "3.1.0-dev-eca1939"
	INSTALLERS_DIR     = "/tmp/installers"
)

var (
	installerList = []string{
		"onprem-ke-installer",
		"onprem-orch-installer",
	}
)

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
		if err := os.MkdirAll(INSTALLERS_DIR, 0755); err != nil {
			return runtimeState, &internal.OrchInstallerError{ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorMsg: fmt.Sprintf("failed to create installers directory %s: %s", INSTALLERS_DIR, err),
			}
		}

		fmt.Println("Created directories for installers")

		if err := downloadArtifacts(ctx, RS_URL, INSTALLERS_RS_PATH, ORCH_VERSION, INSTALLERS_DIR, installerList); err != nil {
			return runtimeState, &internal.OrchInstallerError{ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorMsg: fmt.Sprintf("failed to download installers: %s", err),
			}
		}

		fmt.Println("Downloaded installers successfully")
	}
	return runtimeState, nil
}

func (s *ArtifactDownloader) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
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
