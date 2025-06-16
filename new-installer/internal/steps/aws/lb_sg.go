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
	LBSGModulePath       = "new-installer/targets/aws/iac/lbsg"
	LBSGBackendBucketKey = "lbsg.tfstate"
)

var lbsgStepLabels = []string{
	"aws",
	"load balancer",
	"security group",
}

type LBSGVariables struct {
	ClusterName  string `json:"cluster_name" yaml:"cluster_name"`
	CustomerTag  string `json:"customer_tag" yaml:"customer_tag"`
	Region       string `json:"region" yaml:"region"`
	EKSNodeSGID  string `json:"eks_node_sg_id" yaml:"eks_node_sg_id"`
	TraefikSGID  string `json:"traefik_sg_id" yaml:"traefik_sg_id"`
	Traefik2SGID string `json:"traefik2_sg_id" yaml:"traefik2_sg_id"`
	ArgoCDSGID   string `json:"argocd_sg_id" yaml:"argocd_sg_id"`
}

func NewLBSGVariables() LBSGVariables {
	return LBSGVariables{
		Region:       "",
		CustomerTag:  "",
		ClusterName:  "",
		EKSNodeSGID:  "",
		TraefikSGID:  "",
		Traefik2SGID: "",
		ArgoCDSGID:   "",
	}
}

type LBSGStep struct {
	variables          LBSGVariables
	backendConfig      steps.TerraformAWSBucketBackendConfig
	RootPath           string
	KeepGeneratedFiles bool
	TerraformUtility   steps.TerraformUtility
	AWSUtility         AWSUtility
}

func CreateLBSGStep(rootPath string, keepGeneratedFiles bool, terraformUtility steps.TerraformUtility, awsUtility AWSUtility) *LBSGStep {
	return &LBSGStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: keepGeneratedFiles,
		TerraformUtility:   terraformUtility,
		AWSUtility:         awsUtility,
	}
}

func (s *LBSGStep) Name() string {
	return "LBSGStep"
}

func (s *LBSGStep) Labels() []string {
	return lbsgStepLabels
}

func (s *LBSGStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	s.variables = NewLBSGVariables()
	s.variables.Region = config.AWS.Region
	s.variables.CustomerTag = config.AWS.CustomerTag
	s.variables.ClusterName = config.Global.OrchName
	s.backendConfig = steps.TerraformAWSBucketBackendConfig{
		Bucket: config.Global.OrchName + "-" + runtimeState.DeploymentID,
		Region: config.AWS.Region,
		Key:    LBSGBackendBucketKey,
	}
	s.variables.EKSNodeSGID = runtimeState.AWS.EKSNodeSecurityGroupID
	s.variables.TraefikSGID = runtimeState.AWS.TraefikSecurityGroupID
	s.variables.Traefik2SGID = runtimeState.AWS.Traefik2SecurityGroupID
	s.variables.ArgoCDSGID = runtimeState.AWS.ArgoCDSecurityGroupID

	return runtimeState, nil
}

func (s *LBSGStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if config.AWS.PreviousS3StateBucket == "" {
		// No need to migrate state, since there is no previous state bucket
		return runtimeState, nil
	}

	// Need to move Terraform state from old bucket to new bucket:
	oldLBSGBucketKey := fmt.Sprintf("%s/orch-load-balancer/%s", config.AWS.Region, config.Global.OrchName)
	err := s.AWSUtility.S3CopyToS3(config.AWS.Region,
		config.AWS.PreviousS3StateBucket,
		oldLBSGBucketKey,
		config.AWS.Region,
		s.backendConfig.Bucket,
		s.backendConfig.Key)
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to move Terraform state from old bucket to new bucket: %v", err),
		}
	}

	modulePath := filepath.Join(s.RootPath, LBSGModulePath)

	_, destroyErr := s.TerraformUtility.Run(ctx, steps.TerraformUtilityInput{
		Action:             "uninstall",
		ModulePath:         modulePath,
		Variables:          s.variables,
		BackendConfig:      s.backendConfig,
		LogFile:            filepath.Join(runtimeState.LogDir, "aws_lbsg.log"),
		KeepGeneratedFiles: s.KeepGeneratedFiles,
		DestroyTarget:      "module.aws_lb_security_group_roles.aws_security_group_rule.node_sg_rule",
	})

	if destroyErr != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to destroy old security group rule: %v", destroyErr),
		}
	}

	return runtimeState, nil
}

func (s *LBSGStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	terraformStepInput := steps.TerraformUtilityInput{
		Action:             runtimeState.Action,
		ModulePath:         filepath.Join(s.RootPath, LBSGModulePath),
		Variables:          s.variables,
		BackendConfig:      s.backendConfig,
		LogFile:            filepath.Join(runtimeState.LogDir, "aws_lbsg.log"),
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

func (s *LBSGStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, prevStepError
}
