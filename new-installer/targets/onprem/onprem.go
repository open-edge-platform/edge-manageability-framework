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
	var preInfraStage, infraStage, orchStage internal.OrchInstallerStage

	preInfraStage = NewOnPremStage(
		"PreInfra",
		[]steps.OrchInstallerStep{
			onpremSteps.CreateGenericStep(rootPath, keepGeneratedFiles, orchConfigReaderWriter),
		},
		[]string{"pre-infra"},
		orchConfigReaderWriter,
	)

	infraStage = NewOnPremStage(
		"Infra",
		[]steps.OrchInstallerStep{
			//onpremSteps.CreateArtifactDownloader(rootPath, keepGeneratedFiles, orchConfigReaderWriter),
			onpremSteps.CreateRke2Step(rootPath, keepGeneratedFiles, orchConfigReaderWriter),
		},
		[]string{"infra"},
		orchConfigReaderWriter,
	)

	orchStage = NewOnPremStage(
		"Orchestrator",
		[]steps.OrchInstallerStep{
			onpremSteps.CreateGenericStep(rootPath, keepGeneratedFiles, orchConfigReaderWriter),
		},
		[]string{"orchestator"},
		orchConfigReaderWriter,
	)

	return []internal.OrchInstallerStage{
		preInfraStage,
		infraStage,
		orchStage,
	}, nil
}
