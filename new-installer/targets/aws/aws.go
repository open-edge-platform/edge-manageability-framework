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
	tfUtil, err := steps.CreateTerraformUtility(rootPath)
	if err != nil {
		return nil, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "Failed to create Terraform utility: " + err.Error(),
		}
	}
	aws_util := steps_aws.CreateAWSUtility()
	return []internal.OrchInstallerStage{
		NewAWSStage("PreInfra", []steps.OrchInstallerStep{
			steps_aws.CreateAWSStateBucketStep(rootPath, keepGeneratedFiles, tfUtil),
			steps_aws.CreateAWSVPCStep(rootPath, keepGeneratedFiles, tfUtil, aws_util),
		}, []string{"pre-infra"}, orchConfigReaderWriter),
		NewAWSStage("Infra", []steps.OrchInstallerStep{}, []string{"infra"}, orchConfigReaderWriter),
	}, nil
}
