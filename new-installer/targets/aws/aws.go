// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	steps_aws "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/aws"
)

func CreateAWSStages(rootPath string, keepGeneratedFiles bool, orchConfigReaderWriter config.OrchConfigReaderWriter) ([]internal.OrchInstallerStage, error) {
	tfExecPath, err := steps.InstallTerraformAndGetExecPath()
	if err != nil {
		return nil, err
	}
	return []internal.OrchInstallerStage{
		NewAWSStage("PreInfra", []steps.OrchInstallerStep{
			&steps_aws.CreateAWSStateBucket{
				TerraformExecPath:  tfExecPath,
				RootPath:           rootPath,
				KeepGeneratedFiles: keepGeneratedFiles,
				StepLabels:         []string{"pre-infra", "state-bucket"},
			},
			&steps_aws.AWSVPCStep{
				TerraformExecPath:  tfExecPath,
				RootPath:           rootPath,
				KeepGeneratedFiles: keepGeneratedFiles,
				StepLabels:         []string{"pre-infra", "vpc"},
			},
		}, []string{"pre-infra"}, orchConfigReaderWriter),
		NewAWSStage("Infra", []steps.OrchInstallerStep{}, []string{"infra"}, orchConfigReaderWriter),
	}, nil
}
