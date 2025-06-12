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

const (
	StateBucketModulePath = "new-installer/targets/aws/iac/state_bucket"
)

var StateBucketStepLabels = []string{"aws", "state_bucket"}

type StateBucketVariables struct {
	Region   string `json:"region"`
	OrchName string `json:"orch_name"`
	Bucket   string `json:"bucket"`
}

type AWSStateBucketStep struct {
	variables          StateBucketVariables
	RootPath           string
	KeepGeneratedFiles bool
	TerraformUtility   steps.TerraformUtility
	StepLabels         []string
}

func CreateAWSStateBucketStep(rootPath string, keepGeneratedFiles bool, terraformUtility steps.TerraformUtility) *AWSStateBucketStep {
	return &AWSStateBucketStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: keepGeneratedFiles,
		TerraformUtility:   terraformUtility,
		StepLabels:         StateBucketStepLabels,
	}
}

func (s *AWSStateBucketStep) Name() string {
	return "AWSStateBucketStep"
}

func (s *AWSStateBucketStep) Labels() []string {
	return s.StepLabels
}

func (s *AWSStateBucketStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if config.Global.OrchName == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "OrchName is not set",
		}
	}
	if config.AWS.Region == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "Region is not set",
		}
	}
	if runtimeState.DeploymentID == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "DeploymentId is not set",
		}
	}
	s.variables = StateBucketVariables{
		Region:   config.AWS.Region,
		OrchName: config.Global.OrchName,
		Bucket:   fmt.Sprintf("%s-%s", config.Global.OrchName, runtimeState.DeploymentID),
	}
	return runtimeState, nil
}

func (s *AWSStateBucketStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	// no-op for now
	return runtimeState, nil
}

func (s *AWSStateBucketStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if runtimeState.Action == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "Action is not set",
		}
	}
	output, err := s.TerraformUtility.Run(ctx, steps.TerraformUtilityInput{
		Action:             runtimeState.Action,
		ModulePath:         filepath.Join(s.RootPath, StateBucketModulePath),
		Variables:          s.variables,
		LogFile:            filepath.Join(s.RootPath, ".logs", "aws_state_bucket.log"),
		KeepGeneratedFiles: s.KeepGeneratedFiles,
		TerraformState:     runtimeState.StateBucketState,
	})
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("Failed to run Terraform for AWS state bucket: %v", err),
		}
	}
	if runtimeState.Action != "uninstall" && output.TerraformState == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  "Terraform state is empty",
		}
	} else {
		runtimeState.StateBucketState = output.TerraformState
	}
	return runtimeState, err
}

func (s *AWSStateBucketStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, prevStepError
}
