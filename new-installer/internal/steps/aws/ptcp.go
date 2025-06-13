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
	PTCPModulePath          = "new-installer/targets/aws/iac/ptcp"
	PTCPBackendBucketKey    = "ptcp.tfstate"
	DefaultPTCPCPU          = 1024 // 1 vCPU
	DefaultPTCPMemory       = 2048 // 2048 MB
	DefaultPTCPDesiredCount = 1
)

var ptcpStepLabels = []string{
	"aws",
	"ptcp",
	"pull-through-cache-proxy",
}

type PTCPVariables struct {
	ClusterName     string   `json:"cluster_name"`
	Region          string   `json:"region"`
	VPCID           string   `json:"vpc_id"`
	SubnetIDs       []string `json:"subnet_ids"`
	HTTPProxy       string   `json:"http_proxy"`
	HTTPSProxy      string   `json:"https_proxy"`
	NoProxy         string   `json:"no_proxy"`
	CustomerTag     string   `json:"customer_tag"`
	Route53ZoneName string   `json:"route53_zone_name"`
	IPAllowList     []string `json:"ip_allow_list"`
	CPU             int      `json:"cpu"`
	Memory          int      `json:"memory"`
	DesiredCount    int      `json:"desired_count"`
	TLSCertKey      string   `json:"tls_cert_key"`
	TLSCertBody     string   `json:"tls_cert_body"`
}

type PTCPStep struct {
	variables          PTCPVariables
	backendConfig      TerraformAWSBucketBackendConfig
	RootPath           string
	KeepGeneratedFiles bool
	TerraformUtility   steps.TerraformUtility
	AWSUtility         AWSUtility
}

func CreatePTCPStep(rootPath string, keepGeneratedFiles bool, terraformUtility steps.TerraformUtility, awsUtility AWSUtility) *PTCPStep {
	return &PTCPStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: keepGeneratedFiles,
		TerraformUtility:   terraformUtility,
		AWSUtility:         awsUtility,
	}
}

func (s *PTCPStep) Name() string {
	return "PTCPStep"
}

func (s *PTCPStep) Labels() []string {
	return ptcpStepLabels
}

func (s *PTCPStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if config.Global.OrchName == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "OrchName is not set in the configuration",
		}
	}
	if config.AWS.Region == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "Region is not set in the configuration",
		}
	}
	if config.Global.ParentDomain == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "ParentDomain is not set in the configuration",
		}
	}
	s.variables = PTCPVariables{
		ClusterName:     config.Global.OrchName,
		Region:          config.AWS.Region,
		VPCID:           runtimeState.AWS.VPCID,
		SubnetIDs:       runtimeState.AWS.PrivateSubnetIDs,
		HTTPProxy:       config.Proxy.HTTPProxy,
		HTTPSProxy:      config.Proxy.HTTPSProxy,
		NoProxy:         config.Proxy.NoProxy,
		CustomerTag:     config.AWS.CustomerTag,
		Route53ZoneName: config.Global.ParentDomain,
		IPAllowList:     []string{DefaultNetworkCIDR}, // Only allow traffic from the VPC CIDR.
		CPU:             DefaultPTCPCPU,
		Memory:          DefaultPTCPMemory,
		DesiredCount:    DefaultPTCPDesiredCount,
		TLSCertKey:      runtimeState.Cert.TLSKey,
		TLSCertBody:     runtimeState.Cert.TLSCert,
	}
	s.backendConfig = TerraformAWSBucketBackendConfig{
		Region: config.AWS.Region,
		Bucket: config.Global.OrchName + "-" + runtimeState.DeploymentID,
		Key:    PTCPBackendBucketKey,
	}
	return runtimeState, nil
}

func (s *PTCPStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if config.AWS.PreviousS3StateBucket == "" {
		// No need to migrate state, since there is no previous state bucket
		return runtimeState, nil
	}

	// Need to move Terraform state from old bucket to new bucket:
	oldACMBucketKey := fmt.Sprintf("%s/pull-through-cache-proxy/%s", config.AWS.Region, config.Global.OrchName)
	err := s.AWSUtility.S3CopyToS3(config.AWS.Region,
		config.AWS.PreviousS3StateBucket,
		oldACMBucketKey,
		config.AWS.Region,
		s.backendConfig.Bucket,
		s.backendConfig.Key)
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to move Terraform state from old ACM bucket to new ACM bucket: %v", err),
		}
	}
	modulePath := filepath.Join(s.RootPath, PTCPModulePath)
	states := map[string]string{
		"module.pull_through_cache_proxy.aws_security_group.alb":                                                      "aws_security_group.alb",
		"module.pull_through_cache_proxy.aws_security_group_rule.sg_to_alb":                                           "aws_security_group_rule.sg_to_alb",
		"module.pull_through_cache_proxy.aws_security_group_rule.alb_to_ecs_egress":                                   "aws_security_group_rule.alb_to_ecs_egress",
		"module.pull_through_cache_proxy.aws_lb.pull_through_cache_proxy":                                             "aws_lb.pull_through_cache_proxy",
		"module.pull_through_cache_proxy.aws_acm_certificate.cert":                                                    "aws_acm_certificate.cert",
		"module.pull_through_cache_proxy.aws_lb_listener.https":                                                       "aws_lb_listener.https",
		"module.pull_through_cache_proxy.aws_lb_target_group.pull_through_cache_proxy":                                "aws_lb_target_group.pull_through_cache_proxy",
		"module.pull_through_cache_proxy.aws_ecs_cluster.pull_through_cache_proxy":                                    "aws_ecs_cluster.pull_through_cache_proxy",
		"module.pull_through_cache_proxy.aws_ecs_cluster_capacity_providers.pull_through_cache_proxy":                 "aws_ecs_cluster_capacity_providers.pull_through_cache_proxy",
		"module.pull_through_cache_proxy.aws_iam_role.ecs_task_execution_role":                                        "aws_iam_role.ecs_task_execution_role",
		"module.pull_through_cache_proxy.aws_iam_role_policy_attachment.ecs_task_execution_role":                      "aws_iam_role_policy_attachment.ecs_task_execution_role",
		"module.pull_through_cache_proxy.aws_iam_policy.ecs_task_execution_secrets_policy":                            "aws_iam_policy.ecs_task_execution_secrets_policy",
		"module.pull_through_cache_proxy.aws_iam_policy.aws_secretsmanager_secret.pull_through_cache_proxy":           "aws_iam_policy.aws_secretsmanager_secret.pull_through_cache_proxy",
		"module.pull_through_cache_proxy.aws_iam_role_policy_attachment.ecs_task_execution_secrets_policy_attachment": "aws_iam_role_policy_attachment.ecs_task_execution_secrets_policy_attachment",
		"module.pull_through_cache_proxy.aws_iam_role.ecs_task_role":                                                  "aws_iam_role.ecs_task_role",
		"module.pull_through_cache_proxy.aws_iam_policy.ecs_task_ecr_policy":                                          "aws_iam_policy.ecs_task_ecr_policy",
		"module.pull_through_cache_proxy.aws_iam_role_policy_attachment.ecs_task_ecr_policy_attachment":               "aws_iam_role_policy_attachment.ecs_task_ecr_policy_attachment",
		"module.pull_through_cache_proxy.aws_ecs_task_definition.pull_through_cache_proxy":                            "aws_ecs_task_definition.pull_through_cache_proxy",
		"module.pull_through_cache_proxy.aws_cloudwatch_log_group.pull_through_cache_proxy":                           "aws_cloudwatch_log_group.pull_through_cache_proxy",
		"module.pull_through_cache_proxy.aws_security_group.ecs_service":                                              "aws_security_group.ecs_service",
		"module.pull_through_cache_proxy.aws_security_group_rule.alb_to_ecs_ingress":                                  "aws_security_group_rule.alb_to_ecs_ingress",
		"module.pull_through_cache_proxy.aws_security_group_rule.ecs_to_internet_https":                               "aws_security_group_rule.ecs_to_internet_https",
		"module.pull_through_cache_proxy.aws_ecs_service.pull_through_cache_proxy":                                    "aws_ecs_service.pull_through_cache_proxy",
		"module.pull_through_cache_proxy.aws_route53_record.pull_through_cache_proxy":                                 "aws_route53_record.pull_through_cache_proxy",
		"module.pull_through_cache_proxy.aws_secretsmanager_secret.pull_through_cache_proxy":                          "aws_secretsmanager_secret.pull_through_cache_proxy",
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

func (s *PTCPStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {

	return runtimeState, nil
}

func (s *PTCPStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {

	return runtimeState, prevStepError
}
