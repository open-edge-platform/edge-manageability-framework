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
	KMSModulePath       = "new-installer/targets/aws/iac/kms"
	KMSBackendBucketKey = "kms.tfstate"
)

var kmsStepLabels = []string{
	"aws",
	"kms",
}

type KMSVariables struct {
	Region      string `json:"region" yaml:"region"`
	CustomerTag string `json:"customer_tag" yaml:"customer_tag"`
	ClusterName string `json:"cluster_name" yaml:"cluster_name"`
}

func NewKMSVariables() KMSVariables {
	return KMSVariables{
		Region:      "",
		CustomerTag: "",
		ClusterName: "",
	}
}

type KMSStep struct {
	variables          KMSVariables
	backendConfig      steps.TerraformAWSBucketBackendConfig
	RootPath           string
	KeepGeneratedFiles bool
	TerraformUtility   steps.TerraformUtility
	AWSUtility         AWSUtility
}

func CreateKMSStep(rootPath string, keepGeneratedFiles bool, terraformUtility steps.TerraformUtility, awsUtility AWSUtility) *KMSStep {
	return &KMSStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: keepGeneratedFiles,
		TerraformUtility:   terraformUtility,
		AWSUtility:         awsUtility,
	}
}

func (s *KMSStep) Name() string {
	return "KMSStep"
}

func (s *KMSStep) Labels() []string {
	return kmsStepLabels
}

func (s *KMSStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	s.variables = NewKMSVariables()
	s.variables.Region = config.AWS.Region
	s.variables.CustomerTag = config.AWS.CustomerTag
	s.variables.ClusterName = config.Global.OrchName
	s.backendConfig = steps.TerraformAWSBucketBackendConfig{
		Bucket: config.Global.OrchName + "-" + runtimeState.DeploymentID,
		Region: config.AWS.Region,
		Key:    KMSBackendBucketKey,
	}
	return runtimeState, nil
}

func (s *KMSStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if config.AWS.PreviousS3StateBucket == "" {
		// No need to migrate state, since there is no previous state bucket
		return runtimeState, nil
	}

	// Need to move Terraform state from old bucket to new bucket:
	oldKMSBucketKey := fmt.Sprintf("%s/cluster/%s", config.AWS.Region, config.Global.OrchName)
	err := s.AWSUtility.S3CopyToS3(config.AWS.Region,
		config.AWS.PreviousS3StateBucket,
		oldKMSBucketKey,
		config.AWS.Region,
		s.backendConfig.Bucket,
		s.backendConfig.Key)
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to move Terraform state from old bucket to new bucket: %v", err),
		}
	}

	modulePath := filepath.Join(s.RootPath, KMSModulePath)
	states := map[string]string{
		"module.kms.aws_iam_user.vault":       "aws_iam_user.vault",
		"module.kms.aws_iam_access_key.vault": "aws_iam_access_key.vault",
		"module.kms.aws_kms_key.vault":        "aws_kms_key.vault",
		"module.kms.aws_kms_alias.vault":      "aws_kms_alias.vault",
		"module.kms.aws_kms_key_policy.vault": "aws_kms_key_policy.vault",
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
			"module.eks",
			"module.efs",
			"module.aurora",
			"module.aurora_database",
			"module.aurora_import",
			"module.s3",
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

func (s *KMSStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	terraformStepInput := steps.TerraformUtilityInput{
		Action:             runtimeState.Action,
		ModulePath:         filepath.Join(s.RootPath, KMSModulePath),
		Variables:          s.variables,
		BackendConfig:      s.backendConfig,
		LogFile:            filepath.Join(runtimeState.LogDir, "aws_kms.log"),
		KeepGeneratedFiles: s.KeepGeneratedFiles,
	}
	internal.Logger().Debugf("Running Terraform util %s with input: %+v\n", s.TerraformUtility, terraformStepInput)
	_, err := s.TerraformUtility.Run(ctx, terraformStepInput)
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeTerraform,
			ErrorMsg:  fmt.Sprintf("failed to run terraform: %v", err),
		}
	}
	return runtimeState, nil
}

func (s *KMSStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, prevStepError
}
