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
	ALBModulePath       = "new-installer/targets/aws/iac/alb"
	ALBBackendBucketKey = "alb.tfstate"
)

var albStepLabels = []string{"aws", "alb"}

type ALBVariables struct {
	Internal                 bool     `json:"internal"`
	VPCID                    string   `json:"vpc_id"`
	ClusterName              string   `json:"cluster_name"`
	PublicSubnetIDs          []string `json:"public_subnet_ids"`
	IPAllowList              []string `json:"ip_allow_list"`
	EnableDeletionProtection bool     `json:"enable_deletion_protection"`
	TLSCertARN               string   `json:"tls_cert_arn"`
	Region                   string   `json:"region"`
	CustomerTag              string   `json:"customer_tag,omitempty"`
}

type ALBStep struct {
	variables          ALBVariables
	backendConfig      TerraformAWSBucketBackendConfig
	RootPath           string
	KeepGeneratedFiles bool
	TerraformUtility   steps.TerraformUtility
	AWSUtility         AWSUtility
}

func CreateALBStep(rootPath string, keepGeneratedFiles bool, terraformUtility steps.TerraformUtility, awsUtility AWSUtility) *ALBStep {
	return &ALBStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: keepGeneratedFiles,
		TerraformUtility:   terraformUtility,
		AWSUtility:         awsUtility,
	}
}

func (s *ALBStep) Name() string {
	return "ALBStep"
}

func (s *ALBStep) Labels() []string {
	return albStepLabels
}

func (s *ALBStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if len(runtimeState.AWS.PublicSubnetIDs) == 0 {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "Public subnet IDs must be provided for ALB step",
		}
	}
	if runtimeState.AWS.VPCID == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "VPC ID must be provided for ALB step",
		}
	}
	if config.Global.OrchName == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "Orchestrator name must be provided for ALB step",
		}
	}
	if len(config.AWS.LoadBalancerAllowList) == 0 {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "IP allow list must be provided for ALB step",
		}
	}
	if runtimeState.AWS.ACMCertARN == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "TLS certificate ARN must be provided for ALB step",
		}
	}
	if config.AWS.Region == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "AWS region must be provided for ALB step",
		}
	}
	s.variables = ALBVariables{
		Internal:                 config.AWS.VPCID != "",
		VPCID:                    runtimeState.AWS.VPCID,
		ClusterName:              config.Global.OrchName,
		PublicSubnetIDs:          runtimeState.AWS.PublicSubnetIDs,
		IPAllowList:              config.AWS.LoadBalancerAllowList,
		EnableDeletionProtection: config.AWS.EnableLBDeletionProtection,
		TLSCertARN:               runtimeState.AWS.ACMCertARN,
		Region:                   config.AWS.Region,
		CustomerTag:              config.AWS.CustomerTag,
	}
	s.backendConfig = TerraformAWSBucketBackendConfig{
		Region: config.AWS.Region,
		Bucket: config.Global.OrchName + "-" + runtimeState.DeploymentID,
		Key:    ALBBackendBucketKey,
	}
	return runtimeState, nil
}

func (s *ALBStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if config.AWS.PreviousS3StateBucket == "" {
		// No need to migrate state, since there is no previous state bucket
		return runtimeState, nil
	}
	// Need to move Terraform state from old bucket to new bucket:
	oldALBBucketKey := fmt.Sprintf("%s/orch-load-balancer/%s", config.AWS.Region, config.Global.OrchName)
	err := s.AWSUtility.S3CopyToS3(config.AWS.Region,
		config.AWS.PreviousS3StateBucket,
		oldALBBucketKey,
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
	modulePath := filepath.Join(s.RootPath, ALBModulePath)
	mvErr := s.TerraformUtility.MoveStates(ctx, steps.TerraformUtilityMoveStatesInput{
		ModulePath: modulePath,
		States: map[string]string{
			// Traefik
			"module.traefik_load_balancer.aws_security_group.common":                    "aws_security_group.traefik",
			"module.traefik_load_balancer.aws_lb.main":                                  "aws_lb.traefik",
			"module.traefik_load_balancer.aws_lb_target_group.main[\"default\"]":        "aws_lb_target_group.traefik",
			"module.traefik_load_balancer.aws_lb_target_group.main[\"grpc\"]":           "aws_lb_target_group.traefik_grpc",
			"module.traefik_load_balancer.aws_lb_listener.main":                         "aws_lb_listener.traefik",
			"module.traefik_load_balancer.aws_lb_listener_rule.match_headers[\"grpc\"]": "aws_lb_listener_rule.traefik_grpc",
			// ArgoCD and Gitea
			"module.argocd_load_balancer.aws_security_group.common":                    "aws_security_group.infra",
			"module.argocd_load_balancer.aws_lb.main":                                  "aws_lb.infra",
			"module.argocd_load_balancer.aws_lb_target_group.main[\"argocd\"]":         "aws_lb_target_group.infra_argocd",
			"module.argocd_load_balancer.aws_lb_target_group.main[\"gitea\"]":          "aws_lb_target_group.infra_gitea",
			"module.argocd_load_balancer.aws_lb_listener.main":                         "aws_lb_listener.infra",
			"module.argocd_load_balancer.aws_lb_listener_rule.match_hosts[\"argocd\"]": "aws_lb_listener_rule.infra_argocd",
			"module.argocd_load_balancer.aws_lb_listener_rule.match_hosts[\"gitea\"]":  "aws_lb_listener_rule.infra_gitea",
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
			"module.traefik2_load_balancer",
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

func (s *ALBStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if runtimeState.Action == "uninstall" {
		if err := s.AWSUtility.DisableALBDeletionProtection(config.AWS.Region, runtimeState.AWS.TraefikLBARN); err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorMsg:  fmt.Sprintf("failed to disable ALB deletion protection: %v", err),
			}
		}
		if err := s.AWSUtility.DisableALBDeletionProtection(config.AWS.Region, runtimeState.AWS.InfraLBARN); err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorMsg:  fmt.Sprintf("failed to disable ALB deletion protection: %v", err),
			}
		}
	}
	terraformStepInput := steps.TerraformUtilityInput{
		Action:             runtimeState.Action,
		ModulePath:         filepath.Join(s.RootPath, ALBModulePath),
		Variables:          s.variables,
		BackendConfig:      s.backendConfig,
		LogFile:            filepath.Join(runtimeState.LogDir, "aws_alb.log"),
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
		if traefikDNSName, ok := terraformStepOutput.Output["traefik_dns_name"]; ok {
			runtimeState.AWS.TraefikDNSName = strings.Trim(string(traefikDNSName.Value), "\"")
		} else {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  fmt.Sprintf("cannot find efs id in %s module output", s.Name()),
			}
		}
		if infraDNSName, ok := terraformStepOutput.Output["infra_dns_name"]; ok {
			runtimeState.AWS.InfraDNSName = strings.Trim(string(infraDNSName.Value), "\"")
		} else {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  fmt.Sprintf("cannot find infra DNS name in %s module output", s.Name()),
			}
		}
		if traefikTargetGroupARN, ok := terraformStepOutput.Output["traefik_target_group_arn"]; ok {
			runtimeState.AWS.TraefikTargetGroupARN = strings.Trim(string(traefikTargetGroupARN.Value), "\"")
		} else {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  fmt.Sprintf("cannot find Traefik target group ARN in %s module output", s.Name()),
			}
		}
		if traefikGRPCTargetGroupARN, ok := terraformStepOutput.Output["traefik_grpc_target_group_arn"]; ok {
			runtimeState.AWS.TraefikGRPCTargetGroupARN = strings.Trim(string(traefikGRPCTargetGroupARN.Value), "\"")
		} else {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  fmt.Sprintf("cannot find Traefik gRPC target group ARN in %s module output", s.Name()),
			}
		}
		if infraArgoCDTargetGroupARN, ok := terraformStepOutput.Output["infra_argocd_target_group_arn"]; ok {
			runtimeState.AWS.InfraArgoCDTargetGroupARN = strings.Trim(string(infraArgoCDTargetGroupARN.Value), "\"")
		} else {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  fmt.Sprintf("cannot find Infra ArgoCD target group ARN in %s module output", s.Name()),
			}
		}
		if infraGiteaTargetGroupARN, ok := terraformStepOutput.Output["infra_gitea_target_group_arn"]; ok {
			runtimeState.AWS.InfraGiteaTargetGroupARN = strings.Trim(string(infraGiteaTargetGroupARN.Value), "\"")
		} else {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  fmt.Sprintf("cannot find Infra Gitea target group ARN in %s module output", s.Name()),
			}
		}
		if traefikLBARN, ok := terraformStepOutput.Output["traefik_lb_arn"]; ok {
			runtimeState.AWS.TraefikLBARN = strings.Trim(string(traefikLBARN.Value), "\"")
		} else {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  fmt.Sprintf("cannot find Traefik load balancer ARN in %s module output", s.Name()),
			}
		}
		if infraLBARN, ok := terraformStepOutput.Output["infra_lb_arn"]; ok {
			runtimeState.AWS.InfraLBARN = strings.Trim(string(infraLBARN.Value), "\"")
		} else {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  fmt.Sprintf("cannot find Infra load balancer ARN in %s module output", s.Name()),
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

func (s *ALBStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}
