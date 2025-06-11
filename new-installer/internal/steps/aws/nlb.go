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
	NLBModulePath       = "new-installer/targets/aws/iac/nlb"
	NLBBackendBucketKey = "nlb.tfstate"
)

var nlbStepLabels = []string{"aws", "nlb"}

type NLBVariables struct {
	Internal                 bool     `json:"internal"`
	VPCID                    string   `json:"vpc_id"`
	ClusterName              string   `json:"cluster_name"`
	PublicSubnetIDs          []string `json:"public_subnet_ids"`
	IPAllowList              []string `json:"ip_allow_list"`
	EnableDeletionProtection bool     `json:"enable_deletion_protection"`
	Region                   string   `json:"region"`
	CustomerTag              string   `json:"customer_tag,omitempty"`
}

type NLBStep struct {
	variables          NLBVariables
	backendConfig      TerraformAWSBucketBackendConfig
	RootPath           string
	KeepGeneratedFiles bool
	TerraformUtility   steps.TerraformUtility
	AWSUtility         AWSUtility
}

func CreateNLBStep(rootPath string, keepGeneratedFiles bool, terraformUtility steps.TerraformUtility, awsUtility AWSUtility) *NLBStep {
	return &NLBStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: keepGeneratedFiles,
		TerraformUtility:   terraformUtility,
		AWSUtility:         awsUtility,
	}
}

func (s *NLBStep) Name() string {
	return "NLBStep"
}

func (s *NLBStep) Labels() []string {
	return nlbStepLabels
}

func (s *NLBStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if len(runtimeState.AWS.PublicSubnetIDs) == 0 {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "Public subnet IDs must be provided for NLB step",
		}
	}
	if runtimeState.AWS.VPCID == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "VPC ID must be provided for NLB step",
		}
	}
	if config.Global.OrchName == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "Orchestrator name must be provided for NLB step",
		}
	}
	if len(config.AWS.LoadBalancerAllowList) == 0 {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "IP allow list must be provided for NLB step",
		}
	}
	if config.AWS.Region == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "AWS region must be provided for NLB step",
		}
	}
	s.variables = NLBVariables{
		Internal:                 config.AWS.VPCID != "",
		VPCID:                    runtimeState.AWS.VPCID,
		ClusterName:              config.Global.OrchName,
		PublicSubnetIDs:          runtimeState.AWS.PublicSubnetIDs,
		IPAllowList:              config.AWS.LoadBalancerAllowList,
		EnableDeletionProtection: config.AWS.EnableLBDeletionProtection,
		Region:                   config.AWS.Region,
		CustomerTag:              config.AWS.CustomerTag,
	}
	s.backendConfig = TerraformAWSBucketBackendConfig{
		Region: config.AWS.Region,
		Bucket: config.Global.OrchName + "-" + runtimeState.DeploymentID,
		Key:    NLBBackendBucketKey,
	}
	return runtimeState, nil
}

func (s *NLBStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if config.AWS.PreviousS3StateBucket == "" {
		// No need to migrate state, since there is no previous state bucket
		return runtimeState, nil
	}
	// Need to move Terraform state from old bucket to new bucket:
	oldNLBBucketKey := fmt.Sprintf("%s/orch-load-balancer/%s", config.AWS.Region, config.Global.OrchName)
	err := s.AWSUtility.S3CopyToS3(config.AWS.Region,
		config.AWS.PreviousS3StateBucket,
		oldNLBBucketKey,
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
	modulePath := filepath.Join(s.RootPath, NLBModulePath)
	states := map[string]string{
		"module.traefik2_load_balancer.aws_security_group.common":           "aws_security_group.common",
		"module.traefik2_load_balancer.aws_lb.main":                         "aws_lb.main",
		"module.traefik2_load_balancer.aws_lb_target_group.main[\"https\"]": "aws_lb_target_group.main",
		"module.traefik2_load_balancer.aws_lb_listener.main[\"https\"]":     "aws_lb_listener.main",
	}
	for _, subnetID := range runtimeState.AWS.PublicSubnetIDs {
		states[fmt.Sprintf("module.traefik2_load_balancer.aws_eip.main[\"%s\"]", subnetID)] = fmt.Sprintf("aws_eip.main[\"%s\"]", subnetID)
	}
	mvErr := s.TerraformUtility.MoveStates(ctx, steps.TerraformUtilityMoveStatesInput{
		ModulePath: modulePath,
		States:     states,
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
			"module.traefik_load_balancer",
			"module.argocd_load_balancer",
			"module.traefik_lb_target_group_binding",
			"module.aws_lb_security_group_roles",
			"module.wait_until_alb_ready",
			"module.waf_web_acl_traefik",
			"module.waf_web_acl_argocd",
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

func (s *NLBStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if runtimeState.Action == "uninstall" {
		if err := s.AWSUtility.DisableLBDeletionProtection(config.AWS.Region, runtimeState.AWS.NLBARN); err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorMsg:  fmt.Sprintf("failed to disable NLB deletion protection: %v", err),
			}
		}
	}
	terraformStepInput := steps.TerraformUtilityInput{
		Action:             runtimeState.Action,
		ModulePath:         filepath.Join(s.RootPath, NLBModulePath),
		Variables:          s.variables,
		BackendConfig:      s.backendConfig,
		LogFile:            filepath.Join(runtimeState.LogDir, "aws_nlb.log"),
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
		if nlbDNSName, ok := terraformStepOutput.Output["nlb_dns_name"]; ok {
			runtimeState.AWS.NLBDNSName = strings.Trim(string(nlbDNSName.Value), "\"")
		} else {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  fmt.Sprintf("cannot find nlb_dns_name in %s module output", s.Name()),
			}
		}
		if nlbTargetGroupARN, ok := terraformStepOutput.Output["nlb_target_group_arn"]; ok {
			runtimeState.AWS.NLBTargetGroupARN = strings.Trim(string(nlbTargetGroupARN.Value), "\"")
		} else {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  fmt.Sprintf("cannot find nlb_target_group_arn in %s module output", s.Name()),
			}
		}
		if nlbARN, ok := terraformStepOutput.Output["nlb_arn"]; ok {
			runtimeState.AWS.NLBARN = strings.Trim(string(nlbARN.Value), "\"")
		} else {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  fmt.Sprintf("cannot find nlb_arn in %s module output", s.Name()),
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

func (s *NLBStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}
