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
	ModulePath                           = "new-installer/targets/aws/iac/s3"
	ObservabilityBucketsBackendBucketKey = "observability_buckets.tfstate"
)

type ObservabilityBucketsVariables struct {
	Region        string `json:"region" yaml:"region"`
	CustomerTag   string `json:"customer_tag" yaml:"customer_tag"`
	S3Prefix      string `json:"s3_prefix" yaml:"s3_prefix"`
	ClusterName   string `json:"cluster_name" yaml:"cluster_name"`
	CreateTracing bool   `json:"create_tracing" yaml:"create_tracing"`
}

func NewObservabilityBucketsVariables() ObservabilityBucketsVariables {
	return ObservabilityBucketsVariables{
		Region:        "",
		CustomerTag:   "",
		S3Prefix:      "",
		ClusterName:   "",
		CreateTracing: false,
	}
}

type ObservabilityBucketsStep struct {
	variables          ObservabilityBucketsVariables
	backendConfig      steps.TerraformAWSBucketBackendConfig
	terraformState     string
	RootPath           string
	KeepGeneratedFiles bool
	TerraformExecPath  string
	StepLabels         []string
}

func (s *ObservabilityBucketsStep) Name() string {
	return "AWSObservabilityBucketsStep"
}

func (s *ObservabilityBucketsStep) Labels() []string {
	return s.StepLabels
}

func (s *ObservabilityBucketsStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	s.variables = NewObservabilityBucketsVariables()
	s.variables.Region = config.AWS.Region
	s.variables.CustomerTag = config.AWS.CustomerTag
	s.variables.S3Prefix = config.Global.OrchName // TODO: set this to the correct value from config
	s.variables.ClusterName = config.Global.OrchName
	s.variables.CreateTracing = false
	s.backendConfig = steps.TerraformAWSBucketBackendConfig{
		Bucket: config.Global.OrchName + "-" + config.Generated.DeploymentID,
		Region: config.AWS.Region,
		Key:    ObservabilityBucketsBackendBucketKey,
	}
	return config.Generated, nil
}

func (s *ObservabilityBucketsStep) PreStep(ctx context.Context, config config.OrchInstallerConfig) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	terraformExecPath, err := steps.InstallTerraformAndGetExecPath()
	s.TerraformExecPath = terraformExecPath
	if err != nil {
		return config.Generated, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to get terraform exec path: %v", err),
		}
	}
	return config.Generated, nil
}

func (s *ObservabilityBucketsStep) RunStep(ctx context.Context, config config.OrchInstallerConfig) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	terraformUtility := steps.CreateTerraformUtility()
	terraformStepInput := steps.TerraformUtilityInput{
		Action:             config.Generated.Action,
		ExecPath:           s.TerraformExecPath,
		ModulePath:         filepath.Join(s.RootPath, ModulePath),
		Variables:          s.variables,
		BackendConfig:      s.backendConfig,
		TerraformState:     s.terraformState,
		LogFile:            filepath.Join(config.Generated.LogDir, "aws_observability_bucket.log"),
		KeepGeneratedFiles: s.KeepGeneratedFiles,
	}
	terraformStepOutput, err := terraformUtility.Run(ctx, terraformStepInput)
	if err != nil {
		return config.Generated, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeTerraform,
			ErrorMsg:  fmt.Sprintf("failed to run terraform: %v", err),
		}
	}
	if config.Generated.Action == "uninstall" {
		return config.Generated, nil
	}
	if terraformStepOutput.Output != nil {
		fmt.Println("Terraform Output:")
	}
	return config.Generated, nil
}

func (s *ObservabilityBucketsStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return config.Generated, prevStepError
}
