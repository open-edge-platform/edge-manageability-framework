// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package onprem

import (
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	onpremSteps "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/onprem"
)

func CreateOnPremStages(rootPath string, keepGeneratedFiles bool, orchConfigReaderWriter config.OrchConfigReaderWriter) ([]internal.OrchInstallerStage, error) {
	infraStage := NewOnPremStage(
		"Infra",
		[]steps.OrchInstallerStep{
			// onpremSteps.CreateRKE2DownloadStep(rootPath, keepGeneratedFiles, orchConfigReaderWriter),
			onpremSteps.CreateRKE2InstallStep(rootPath, keepGeneratedFiles, orchConfigReaderWriter),
			// onpremSteps.CreateRKE2CustomizeStep(
			// 	rootPath,
			// 	keepGeneratedFiles,
			// 	orchConfigReaderWriter,
			// 	filepath.Join(rootPath, "new-installer", "targets", "onprem", "rke2"),
			// ),
			// commonSteps.CreateArgoStep(rootPath, keepGeneratedFiles, orchConfigReaderWriter),
		},
		[]string{"infra"},
		orchConfigReaderWriter,
	)

	return []internal.OrchInstallerStage{infraStage}, nil
}
