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
	EC2LogModulePath       = "new-installer/targets/aws/iac/ec2log"
	EC2LogBackendBucketKey = "ec2log.tfstate"
)

var EC2LOGStepLabels = []string{
	"aws",
	"ec2log",
}

type EC2logVariables struct {
	ClusterName   string `json:"cluster_name" yaml:"cluster_name"`
	NodeGroupRole string `json:"nodegroup_role" yaml:"nodegroup_role"`
	S3Prefix      string `json:"s3_prefix" yaml:"s3_prefix"`
	Region        string `json:"region" yaml:"region"`
	CustomerTag   string `json:"customer_tag" yaml:"customer_tag"`
}

// NewDefaultEC2LogVariables creates a new EC2logVariables with default values
// based on variable.tf default definitions.
func NewDefaultEC2LogVariables() EC2logVariables {
	return EC2logVariables{
		S3Prefix:    "",
		CustomerTag: "",
	}
}

type EC2LogStep struct {
	variables          EC2logVariables
	backendConfig      TerraformAWSBucketBackendConfig
	RootPath           string
	KeepGeneratedFiles bool
	TerraformUtility   steps.TerraformUtility
	AWSUtility         AWSUtility
}

func CreateEC2LogStep(rootPath string, keepGeneratedFiles bool, terraformUtility steps.TerraformUtility, awsUtility AWSUtility) *EC2LogStep {
	return &EC2LogStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: keepGeneratedFiles,
		TerraformUtility:   terraformUtility,
		AWSUtility:         awsUtility,
	}
}

func (s *EC2LogStep) Name() string {
	return "EC2LogStep"
}

func (s *EC2LogStep) Labels() []string {
	return EC2LOGStepLabels
}

func (s *EC2LogStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	// no output to check if the EC2Log step is already created. Does it need to be added?
	s.variables = NewDefaultEC2LogVariables()

	if config.Global.OrchName == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "OrchName is not set",
		}
	}
	if config.AWS.Region == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "AWS Region is not set",
		}
	}
	if runtimeState.AWS.NodeGroupRole == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "NodeGroupRole is not set",
		}
	}

	s.variables.Region = config.AWS.Region           // Region is required for AWS resources
	s.variables.ClusterName = config.Global.OrchName // ClusterName is required for AWS resources
	s.variables.NodeGroupRole = runtimeState.AWS.NodeGroupRole

	//to do: need to create a default for these 2 variables if not provided
	s.variables.S3Prefix = runtimeState.DeploymentID
	s.variables.CustomerTag = config.AWS.CustomerTag

	s.backendConfig = TerraformAWSBucketBackendConfig{
		Region: config.AWS.Region,
		Bucket: config.Global.OrchName + "-" + runtimeState.DeploymentID,
		Key:    EC2LogBackendBucketKey,
	}
	return runtimeState, nil
}

func (s *EC2LogStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if config.AWS.PreviousS3StateBucket == "" {
		// No need to migrate state, since there is no previous state bucket
		return runtimeState, nil
	}

	// Need to move Terraform state from old bucket to new bucket:
	oldVPCBucketKey := fmt.Sprintf("%s/cluster/%s", config.AWS.Region, config.Global.OrchName)
	err := s.AWSUtility.S3CopyToS3(config.AWS.Region,
		config.AWS.PreviousS3StateBucket,
		oldVPCBucketKey,
		config.AWS.Region,
		s.backendConfig.Bucket,
		s.backendConfig.Key)
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to move Terraform state from old bucket to new bucket: %v", err),
		}
	}

	modulePath := filepath.Join(s.RootPath, EC2LogModulePath)
	states := map[string]string{
		"module.ec2log.aws_ssm_document.push_log":                                    "aws_ssm_document.push_log",
		"module.ec2log.local_file.ssm_term_src":                                      "local_file.ssm_term_src",
		"module.ec2log.aws_lambda_function.ssm_term":                                 "aws_lambda_function.ssm_term",
		"module.ec2log.aws_cloudwatch_log_group.ssm_term":                            "aws_cloudwatch_log_group.ssm_term",
		"module.ec2log.aws_lambda_permission.allow_cloudwatch":                       "aws_lambda_permission.allow_cloudwatch",
		"module.ec2log.aws_s3_bucket.logs":                                           "aws_s3_bucket.logs",
		"module.ec2log.aws_iam_policy.cloudwatch":                                    "aws_iam_policy.cloudwatch",
		"module.ec2log.aws_iam_policy.s3":                                            "aws_iam_policy.s3",
		"module.ec2log.aws_iam_policy.ssm":                                           "aws_iam_policy.ssm",
		"module.ec2log.aws_iam_role_policy_attachment.ec2_cloudwatch":                "aws_iam_role_policy_attachment.ec2_cloudwatch",
		"module.ec2log.aws_iam_role_policy_attachment.ec2_s3":                        "aws_iam_role_policy_attachment.ec2_s3",
		"module.ec2log.aws_iam_role.lambda":                                          "aws_iam_role.lambda",
		"module.ec2log.aws_iam_role_policy_attachment.lambda_cloudwatch":             "aws_iam_role_policy_attachment.lambda_cloudwatch",
		"module.ec2log.aws_iam_role_policy_attachment.lambda_ssm":                    "aws_iam_role_policy_attachment.lambda_ssm",
		"module.ec2log.aws_autoscaling_lifecycle_hook.nodegroup1-terminating":        "aws_autoscaling_lifecycle_hook.nodegroup1-terminating",
		"module.ec2log.aws_autoscaling_lifecycle_hook.observability-terminating":     "aws_autoscaling_lifecycle_hook.observability-terminating",
		"module.ec2log.aws_cloudwatch_event_rule.instance_terminate":                 "aws_cloudwatch_event_rule.instance_terminate",
		"module.ec2log.aws_cloudwatch_event_target.lambda_runssm_instance_terminate": "aws_cloudwatch_event_target.lambda_runssm_instance_terminate",
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

	rmErr := s.TerraformUtility.RemoveStates(ctx, steps.TerraformUtilityRemoveStatesInput{
		ModulePath: modulePath,
		States: []string{
			"module.s3",
			"module.eks",
			"module.aurora",
			"module.aurora_database",
			"module.aurora_import",
			"module.kms",
			"module.orch_init",
			"module.eks_auth",
			"module.efs",
			"module.aws_lb_controller",
			"module.gitea",
		},
	})
	if rmErr != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to remove Terraform states: %v", rmErr),
		}
	}
	return runtimeState, nil
}

func (s *EC2LogStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	terraformStepInput := steps.TerraformUtilityInput{
		Action:             runtimeState.Action,
		ModulePath:         filepath.Join(s.RootPath, EC2LogModulePath),
		Variables:          s.variables,
		BackendConfig:      s.backendConfig,
		LogFile:            filepath.Join(runtimeState.LogDir, "aws_ec2log.log"),
		KeepGeneratedFiles: s.KeepGeneratedFiles,
	}

	_, err := s.TerraformUtility.Run(ctx, terraformStepInput)
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeTerraform,
			ErrorMsg:  fmt.Sprintf("failed to run terraform: %v", err),
		}
	}
	// No output to be parsed from ec2log Terraform module.
	return runtimeState, nil
}

func (s *EC2LogStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, prevStepError
}
