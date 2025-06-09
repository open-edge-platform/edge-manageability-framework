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

var AWSEC2LOGStepLabels = []string{
	"aws",
	"ec2log",
}

type AWSEC2logVariables struct {
	ClusterName   string `json:"cluster_name" yaml:"cluster_name"`
	NodeGroupRole string `json:"nodegroup_role" yaml:"nodegroup_role"`
	S3Prefix      string `json:"s3_prefix" yaml:"s3_prefix"`
	Region        string `json:"region" yaml:"region"`
	CustomerTag   string `json:"customer_tag" yaml:"customer_tag"`
}

// NewDefaultAWSEC2LogVariables creates a new AWSEC2logVariables with default values
// based on variable.tf default definitions.
func NewDefaultAWSEC2LogVariables() AWSEC2logVariables {
	return AWSEC2logVariables{
		S3Prefix:    "",
		CustomerTag: "",
	}
}

type AWSEC2LogStep struct {
	variables          AWSEC2logVariables
	backendConfig      TerraformAWSBucketBackendConfig
	RootPath           string
	KeepGeneratedFiles bool
	StepLabels         []string
	TerraformUtility   steps.TerraformUtility
	AWSUtility         AWSUtility
}

func CreateAWSEC2LogStep(rootPath string, keepGeneratedFiles bool, terraformUtility steps.TerraformUtility, awsUtility AWSUtility) *AWSEC2LogStep {
	return &AWSEC2LogStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: keepGeneratedFiles,
		TerraformUtility:   terraformUtility,
		AWSUtility:         awsUtility,
		StepLabels:         AWSVPCStepLabels,
	}
}

func (s *AWSEC2LogStep) Name() string {
	return "AWSEC2LogStep"
}

func (s *AWSEC2LogStep) Labels() []string {
	return s.StepLabels
}

func (s *AWSEC2LogStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	// no output to check if the EC2Log step is already created. Does it need to be added?
	s.variables = NewDefaultAWSEC2LogVariables()

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
	if config.AWS.NodeGroupRole == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "NodeGroupRole is not set",
		}
	}

	s.variables.Region = config.AWS.Region               // Region is required for AWS resources
	s.variables.ClusterName = config.Global.OrchName     // ClusterName is required for AWS resources
	s.variables.NodeGroupRole = config.AWS.NodeGroupRole // NodeGroupRole is required for AWS resources

	//to do: need to create a default for these 2 variables if not provided
	s.variables.S3Prefix = config.AWS.S3Prefix
	s.variables.CustomerTag = config.AWS.CustomerTag

	s.backendConfig = TerraformAWSBucketBackendConfig{
		Region: config.AWS.Region,
		Bucket: config.Global.OrchName + "-" + runtimeState.DeploymentID,
		Key:    EC2LogBackendBucketKey,
	}
	return runtimeState, nil
}

func (s *AWSEC2LogStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if config.AWS.PreviousS3StateBucket == "" {
		// No need to migrate state, since there is no previous state bucket
		return runtimeState, nil
	}

	// Need to move Terraform state from old bucket to new bucket:
	oldVPCBucketKey := fmt.Sprintf("%s/ec2log/%s", config.AWS.Region, config.Global.OrchName)
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

	return runtimeState, nil
}

func (s *AWSEC2LogStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	terraformStepInput := steps.TerraformUtilityInput{
		Action:             runtimeState.Action,
		ModulePath:         filepath.Join(s.RootPath, EC2LogModulePath),
		Variables:          s.variables,
		BackendConfig:      s.backendConfig,
		LogFile:            filepath.Join(runtimeState.LogDir, "aws_ec2log.log"),
		KeepGeneratedFiles: s.KeepGeneratedFiles,
	}

	_, err := s.TerraformUtility.Run(ctx, terraformStepInput)
	// terraformStepOutput, err := s.TerraformUtility.Run(ctx, terraformStepInput)
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeTerraform,
			ErrorMsg:  fmt.Sprintf("failed to run terraform: %v", err),
		}
	}
	if runtimeState.Action == "uninstall" {
		return runtimeState, nil
	}
	// to do: determine if any terraform output needs to be parsed, using pattern like below
	/* if terraformStepOutput.Output != nil {
		if vpcIDMeta, ok := terraformStepOutput.Output["vpc_id"]; !ok {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  "vpc_id does not exist in terraform output",
			}
		} else {
			runtimeState.VPCID = strings.Trim(string(vpcIDMeta.Value), "\"")
		}
	} */

	return runtimeState, nil
}

func (s *AWSEC2LogStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, prevStepError
}
