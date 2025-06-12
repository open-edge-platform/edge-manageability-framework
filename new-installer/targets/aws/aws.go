// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"context"

	"github.com/hashicorp/go-version"
	install "github.com/hashicorp/hc-install"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/hashicorp/hc-install/src"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	steps_aws "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/aws"
)

func installTerraform() (execPath string, err error) {
	ctx := context.Background()
	i := install.NewInstaller()
	v1_3 := version.Must(version.NewVersion(steps.TerraformVersion))
	execPath, err = i.Install(ctx, []src.Installable{
		&releases.ExactVersion{
			Product: product.Terraform,
			Version: v1_3,
		},
	})
	if err != nil {
		return "", err
	}
	internal.Logger().Debugf("Terraform %s installed to %s", v1_3, execPath)
	return execPath, nil
}

func CreateAWSStages(rootPath string, keepGeneratedFiles bool, orchConfigReaderWriter config.OrchConfigReaderWriter) ([]internal.OrchInstallerStage, error) {
	terraformExecPath, installErr := installTerraform()
	if installErr != nil {
		return nil, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "Failed to install Terraform: " + installErr.Error(),
		}
	}

	tfUtil, err := steps.CreateTerraformUtility(terraformExecPath)
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
			steps_aws.CreateVPCStep(rootPath, keepGeneratedFiles, tfUtil, aws_util),
		}, []string{"pre-infra"}, orchConfigReaderWriter),
		NewAWSStage("Infra", []steps.OrchInstallerStep{
			// steps_aws.CreateEFSStep(rootPath, keepGeneratedFiles, tfUtil, aws_util),
			// steps_aws.CreateObservabilityBucketsStep(rootPath, keepGeneratedFiles, tfUtil, aws_util),
			steps_aws.CreateKMSStep(rootPath, keepGeneratedFiles, tfUtil, aws_util),
		}, []string{"infra"}, orchConfigReaderWriter),
	}, nil
}
