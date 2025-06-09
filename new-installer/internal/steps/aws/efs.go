// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package steps_aws

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
)

const (
	EFSModulePath       = "new-installer/targets/aws/iac/efs"
	EFSBackendBucketKey = "efs.tfstate"
)

var (
	StepLabels = []string{"aws", "efs"}
)

type EFSVariables struct {
	ClusterName      string   `json:"cluster_name" yaml:"cluster_name"`
	Region           string   `json:"region" yaml:"region"`
	CustomerTag      string   `json:"customer_tag" yaml:"customer_tag"`
	PrivateSubnetIDs []string `json:"private_subnet_ids" yaml:"private_subnet_ids"`
	VPCID            string   `json:"vpc_id" yaml:"vpc_id"`
	EKSOIDCIssuer    string   `json:"eks_oidc_issuer" yaml:"eks_oidc_issuer"`
}

// NewDefaultEFSVariables creates a new AWSVPCVariables with default values
// based on variable.tf default definitions.
func NewDefaultEFSVariables() EFSVariables {
	return EFSVariables{
		ClusterName:      "",
		Region:           "",
		CustomerTag:      "",
		PrivateSubnetIDs: []string{},
		VPCID:            "",
		EKSOIDCIssuer:    "",
	}
}

type EFSStep struct {
	variables          EFSVariables
	backendConfig      TerraformAWSBucketBackendConfig
	RootPath           string
	KeepGeneratedFiles bool
	TerraformUtility   steps.TerraformUtility
	AWSUtility         AWSUtility
}

func CreateEFSStep(rootPath string, keepGeneratedFiles bool, terraformUtility steps.TerraformUtility, awsUtility AWSUtility) *EFSStep {
	return &EFSStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: keepGeneratedFiles,
		TerraformUtility:   terraformUtility,
		AWSUtility:         awsUtility,
	}
}

func (s *EFSStep) Name() string {
	return "EFSStep"
}

func (s *EFSStep) Labels() []string {
	return StepLabels
}

func (s *EFSStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	s.variables = NewDefaultEFSVariables()
	s.variables.ClusterName = config.Global.OrchName
	s.variables.Region = config.AWS.Region
	s.variables.CustomerTag = config.AWS.CustomerTag

	if runtimeState.AWS.EKSOIDCIssuer == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidRuntimeState,
			ErrorMsg:  fmt.Sprintf("EKSOIDCIssuer should not be empty in runtime state for step %s", s.Name()),
		}
	}
	s.variables.EKSOIDCIssuer = runtimeState.AWS.EKSOIDCIssuer

	if len(runtimeState.AWS.PrivateSubnetIDs) == 0 {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidRuntimeState,
			ErrorMsg:  fmt.Sprintf("PrivateSubnetIDs should not be empty in runtime state for step %s", s.Name()),
		}
	}
	s.variables.PrivateSubnetIDs = runtimeState.AWS.PrivateSubnetIDs

	if runtimeState.AWS.VPCID == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidRuntimeState,
			ErrorMsg:  fmt.Sprintf("VPCID should not be empty in runtime state for step %s", s.Name()),
		}
	}
	s.variables.VPCID = runtimeState.AWS.VPCID

	s.backendConfig = TerraformAWSBucketBackendConfig{
		Region: config.AWS.Region,
		Bucket: config.Global.OrchName + "-" + runtimeState.DeploymentID,
		Key:    EFSBackendBucketKey,
	}

	return runtimeState, nil
}

func (s *EFSStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if config.AWS.PreviousS3StateBucket == "" {
		// No need to migrate state, since there is no previous state bucket
		return runtimeState, nil
	}
	// Need to move Terraform state from old bucket to new bucket:
	oldEFSBucketKey := fmt.Sprintf("%s/cluster/%s", config.AWS.Region, config.Global.OrchName)
	err := s.AWSUtility.S3CopyToS3(config.AWS.Region,
		config.AWS.PreviousS3StateBucket,
		oldEFSBucketKey,
		config.AWS.Region,
		s.backendConfig.Bucket,
		s.backendConfig.Key)
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to move Terraform state from old bucket to new bucket: %v", err),
		}
	}

	// Need to delete unrelevant states.
	// Anything that is not related to EKS should be deleted.
	modulePath := filepath.Join(s.RootPath, EFSModulePath)
	mvErr := s.TerraformUtility.MoveStates(ctx, steps.TerraformUtilityMoveStatesInput{
		ModulePath: modulePath,
		States: map[string]string{
			"module.efs.aws_efs_file_system.efs":      "aws_efs_file_system.efs",
			"module.efs.aws_efs_mount_target.target":  "aws_efs_mount_target.target",
			"module.efs.aws_iam_policy.efs_policy":    "aws_iam_policy.efs_policy",
			"module.efs.aws_iam_role.efs_role":        "aws_iam_role.efs_role",
			"module.efs.aws_security_group.allow_nfs": "aws_security_group.allow_nfs",
		},
	})
	if mvErr != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to move Terraform states: %v", mvErr),
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
			"module.ec2log",
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

func (s *EFSStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	terraformStepInput := steps.TerraformUtilityInput{
		Action:             runtimeState.Action,
		ModulePath:         filepath.Join(s.RootPath, EFSModulePath),
		Variables:          s.variables,
		BackendConfig:      s.backendConfig,
		LogFile:            filepath.Join(runtimeState.LogDir, "aws_efs.log"),
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

	if terraformStepOutput.Output != nil {
		if fileSystemID, ok := terraformStepOutput.Output["efs_id"]; ok {
			runtimeState.AWS.EFSFileSystemID = strings.Trim(string(fileSystemID.Value), "\"")
		} else {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  fmt.Sprintf("cannot find efs id in %s module output", s.Name()),
			}
		}
	} else {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeTerraform,
			ErrorMsg:  fmt.Sprintf("cannot find any output from %s module", s.Name()),
		}
	}

	return runtimeState, nil
}

func (s *EFSStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, prevStepError
}
