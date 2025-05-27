// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps_aws

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
)

const StateBucketModulePath = "new-installer/targets/aws/iac/state_bucket"

type StateBucketVariables struct {
	Region   string `json:"region"`
	OrchName string `json:"orch_name"`
	Bucket   string `json:"bucket"`
}

type CreateAWSStateBucket struct {
	variables          StateBucketVariables
	RootPath           string
	KeepGeneratedFiles bool
	TerraformUtility   steps.TerraformUtility
	StepLabels         []string
}

func (s *CreateAWSStateBucket) Name() string {
	return "CreateAWSStateBucket"
}

func (s *CreateAWSStateBucket) Labels() []string {
	return s.StepLabels
}

func (s *CreateAWSStateBucket) ConfigStep(ctx context.Context, config config.OrchInstallerConfig) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if config.Global.OrchName == "" {
		return config.Generated, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "OrchName is not set",
		}
	}
	if config.AWS.Region == "" {
		return config.Generated, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "Region is not set",
		}
	}
	if config.Generated.DeploymentID == "" {
		return config.Generated, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "DeploymentId is not set",
		}
	}
	s.variables = StateBucketVariables{
		Region:   config.AWS.Region,
		OrchName: config.Global.OrchName,
		Bucket:   fmt.Sprintf("%s-%s", config.Global.OrchName, config.Generated.DeploymentID),
	}
	return config.Generated, nil
}

func (s *CreateAWSStateBucket) PreStep(ctx context.Context, config config.OrchInstallerConfig) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	// no-op for now
	return config.Generated, nil
}

func (s *CreateAWSStateBucket) RunStep(ctx context.Context, config config.OrchInstallerConfig) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if config.Generated.Action == "" {
		return config.Generated, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "Action is not set",
		}
	}
	output, err := s.TerraformUtility.Run(ctx, steps.TerraformUtilityInput{
		Action:             config.Generated.Action,
		ModulePath:         filepath.Join(s.RootPath, StateBucketModulePath),
		Variables:          s.variables,
		LogFile:            filepath.Join(s.RootPath, ".logs", "aws_state_bucket.log"),
		KeepGeneratedFiles: s.KeepGeneratedFiles,
	})
	if config.Generated.Action != "uninstall" && output.TerraformState == "" {
		return config.Generated, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  "Terraform state is empty",
		}
	} else {
		config.Generated.StateBucketState = output.TerraformState
	}
	return config.Generated, err
}

func (s *CreateAWSStateBucket) PostStep(ctx context.Context, config config.OrchInstallerConfig, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return config.Generated, prevStepError
}
