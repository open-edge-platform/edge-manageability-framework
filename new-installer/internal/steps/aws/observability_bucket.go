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
	ObservabilityBucketsModulePath       = "new-installer/targets/aws/iac/o11y_buckets"
	ObservabilityBucketsBackendBucketKey = "o11y_buckets.tfstate"
)

var observabilityBucketsStepLabels = []string{
	"aws",
	"observability",
	"s3",
}

type ObservabilityBucketsVariables struct {
	Region        string `json:"region" yaml:"region"`
	CustomerTag   string `json:"customer_tag" yaml:"customer_tag"`
	S3Prefix      string `json:"s3_prefix" yaml:"s3_prefix"`
	OIDCIssuer    string `json:"oidc_issuer" yaml:"oidc_issuer"`
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
	RootPath           string
	KeepGeneratedFiles bool
	TerraformUtility   steps.TerraformUtility
	AWSUtility         AWSUtility
	StepLabels         []string
}

func CreateObservabilityBucketsStep(rootPath string, keepGeneratedFiles bool, terraformUtility steps.TerraformUtility, awsUtility AWSUtility) *ObservabilityBucketsStep {
	return &ObservabilityBucketsStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: keepGeneratedFiles,
		TerraformUtility:   terraformUtility,
		AWSUtility:         awsUtility,
		StepLabels:         observabilityBucketsStepLabels,
	}
}

func (s *ObservabilityBucketsStep) Name() string {
	return "ObservabilityBucketsStep"
}

func (s *ObservabilityBucketsStep) Labels() []string {
	return s.StepLabels
}

func (s *ObservabilityBucketsStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	s.variables = NewObservabilityBucketsVariables()
	s.variables.Region = config.AWS.Region
	s.variables.CustomerTag = config.AWS.CustomerTag
	s.variables.S3Prefix = runtimeState.DeploymentID
	s.variables.OIDCIssuer = runtimeState.AWS.EKSOIDCIssuer
	s.variables.ClusterName = config.Global.OrchName
	s.variables.CreateTracing = false
	s.backendConfig = steps.TerraformAWSBucketBackendConfig{
		Bucket: config.Global.OrchName + "-" + runtimeState.DeploymentID,
		Region: config.AWS.Region,
		Key:    ObservabilityBucketsBackendBucketKey,
	}
	return runtimeState, nil
}

func (s *ObservabilityBucketsStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if config.AWS.PreviousS3StateBucket == "" {
		// No need to migrate state, since there is no previous state bucket
		return runtimeState, nil
	}

	oldObservabilityBucketsBucketKey := fmt.Sprintf("%s/o11y_buckets/%s", config.AWS.Region, config.Global.OrchName)
	err := s.AWSUtility.S3CopyToS3(config.AWS.Region,
		config.AWS.PreviousS3StateBucket,
		oldObservabilityBucketsBucketKey,
		config.AWS.Region,
		s.backendConfig.Bucket,
		s.backendConfig.Key)
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to move Terraform state from old bucket to new bucket: %v", err),
		}
	}

	modulePath := filepath.Join(s.RootPath, ObservabilityBucketsModulePath)
	// TODO: Handle the cases where the state wasn't present in old installer
	states := map[string]string{
		"module.s3.aws_iam_policy.s3_policy": "aws_iam_policy.s3_policy",
		"module.s3.aws_iam_role.s3_role":     "aws_iam_role.s3_role",
		//nil:                                  "aws_iam_role_policy_attachment.s3_role",
		//nil:                                  "aws_kms_key.bucket_key",
		"module.s3.aws_s3_bucket.bucket": "aws_s3_bucket.bucket",
		// nil:                                  "aws_s3_bucket_public_access_block.bucket",
		// nil:                                  "aws_s3_bucket_server_side_encryption_configuration.bucket",
		// nil:                                  "aws_s3_bucket_versioning.bucket",
		"module.s3.aws_s3_bucket_lifecycle_configuration.bucket_config": "aws_s3_bucket_lifecycle_configuration.bucket_config",
		"module.s3.aws_s3_bucket_policy.bucket_policy":                  "aws_s3_bucket_policy.bucket_policy",
		"module.s3.aws_s3_bucket.tracing":                               "aws_s3_bucket.tracing",
		// nil:                                                             "aws_s3_bucket_server_side_encryption_configuration.tracing",
		// nil:                                                             "aws_s3_bucket_public_access_block.tracing",
		// nil:                                                             "aws_s3_bucket_versioning.tracing",
		"module.s3.aws_s3_bucket_lifecycle_configuration.tracing_config": "aws_s3_bucket_lifecycle_configuration.tracing_config",
		// nil: "aws_iam_policy_document.tracing_policy_doc",
		"module.s3.aws_s3_bucket_policy.tracing_policy": "aws_s3_bucket_policy.tracing_policy",
	}

	mvErr := s.TerraformUtility.MoveStates(ctx, steps.TerraformUtilityMoveStatesInput{
		ModulePath: modulePath,
		States:     states,
	})
	if mvErr != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to move Terraform state: %v", mvErr),
		}
	}

	return runtimeState, nil
}

func (s *ObservabilityBucketsStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	terraformStepInput := steps.TerraformUtilityInput{
		Action:             runtimeState.Action,
		ModulePath:         filepath.Join(s.RootPath, ObservabilityBucketsModulePath),
		Variables:          s.variables,
		BackendConfig:      s.backendConfig,
		LogFile:            filepath.Join(runtimeState.LogDir, "aws_observability_bucket.log"),
		KeepGeneratedFiles: s.KeepGeneratedFiles,
	}
	terraformStepOutput, err := s.TerraformUtility.Run(ctx, terraformStepInput)
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeTerraform,
			ErrorMsg:  fmt.Sprintf("failed to run terraform: %v", err),
		}
	}
	if runtimeState.Action == "uninstall" {
		return runtimeState, nil
	}
	if terraformStepOutput.Output != nil && len(terraformStepOutput.Output) > 0 {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeTerraform,
			ErrorMsg:  fmt.Sprintf("unexpected output from observability buckets module: %v", terraformStepOutput.Output),
		}
	}
	return runtimeState, nil
}

func (s *ObservabilityBucketsStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, prevStepError
}
