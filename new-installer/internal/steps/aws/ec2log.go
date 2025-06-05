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

func (s *AWSEC2LogStep) Name() string {
	return "AWSEC2LogStep"
}

func (s *AWSEC2LogStep) Labels() []string {
	return s.StepLabels
}

func (s *AWSEC2LogStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
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
