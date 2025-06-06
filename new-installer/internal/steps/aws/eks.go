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
	DefaultEKSVersion   = "1.32"
	EKSBackendBucketKey = "eks.tfstate"
	EKSModulePath       = "new-installer/targets/aws/iac/eks"
	eksStepName         = "EKSStep"
)

var eksStepLabels = []string{"aws", "eks"}

type EKSAddOn struct {
	Name                string `json:"name"`
	Version             string `json:"version"`
	ConfigurationValues string `json:"configuration_values"`
}

type EKSNodeGroup struct {
	DesiredSize  int                 `json:"desired_size"`
	MinSize      int                 `json:"min_size"`
	MaxSize      int                 `json:"max_size"`
	MaxPods      int                 `json:"max_pods,omitempty"`
	Taints       map[string]EKSTaint `json:"taints"`
	Labels       map[string]string   `json:"labels"`
	InstanceType string              `json:"instance_type"`
	VolumeSize   int                 `json:"volume_size"`
	VolumeType   string              `json:"volume_type"`
}

type EKSTaint struct {
	Value  string `json:"value"`
	Effect string `json:"effect"`
}

type EKSVariables struct {
	Name                    string                  `json:"name"`
	Region                  string                  `json:"region"`
	VPCID                   string                  `json:"vpc_id"`
	CustomerTag             string                  `json:"customer_tag"`
	SubnetIDs               []string                `json:"subnet_ids"`
	EKSVersion              string                  `json:"eks_version"`
	VolumeSize              int                     `json:"volume_size"`
	VolumeType              string                  `json:"volume_type"`
	NodeInstanceType        string                  `json:"node_instance_type"`
	DesiredSize             int                     `json:"desired_size"`
	MinSize                 int                     `json:"min_size"`
	MaxSize                 int                     `json:"max_size"`
	AddOns                  []EKSAddOn              `json:"addons"`
	MaxPods                 int                     `json:"max_pods"`
	AdditionalNodeGroups    map[string]EKSNodeGroup `json:"additional_node_groups"`
	EnableCacheRegistry     bool                    `json:"enable_cache_registry"`
	CacheRegistry           string                  `json:"cache_registry"`
	UserScriptPreCloudInit  string                  `json:"user_script_pre_cloud_init"`
	UserScriptPostCloudInit string                  `json:"user_script_post_cloud_init"`
	HTTPProxy               string                  `json:"http_proxy"`
	HTTPSProxy              string                  `json:"https_proxy"`
	NoProxy                 string                  `json:"no_proxy"`
}

type EKSStep struct {
	variables          EKSVariables
	backendConfig      TerraformAWSBucketBackendConfig
	RootPath           string
	KeepGeneratedFiles bool
	TerraformUtility   steps.TerraformUtility
	AWSUtility         AWSUtility
}

func (s *EKSStep) Name() string {
	return eksStepName
}

func (s *EKSStep) Labels() []string {
	return eksStepLabels
}

func (s *EKSStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	// With fixed default values
	s.variables.EKSVersion = DefaultEKSVersion
	s.variables.AddOns = []EKSAddOn{
		{
			Name:    "aws-ebs-csi-driver",
			Version: "v1.39.0-eksbuild.1",
		},
		{
			Name:                "vpc-cni",
			Version:             "v1.19.2-eksbuild.1",
			ConfigurationValues: "{\"enableNetworkPolicy\": \"true\", \"nodeAgent\": {\"healthProbeBindAddr\": \"8163\", \"metricsBindAddr\": \"8162\"}}",
		},
		{
			Name:    "aws-efs-csi-driver",
			Version: "v2.1.4-eksbuild.1",
		},
	}
	s.variables.UserScriptPreCloudInit = ""
	s.variables.UserScriptPostCloudInit = ""

	// With values from config
	s.variables.Name = config.Global.OrchName
	s.variables.Region = config.AWS.Region
	s.variables.VPCID = runtimeState.VPCID
	s.variables.CustomerTag = config.AWS.CustomerTag
	s.variables.SubnetIDs = runtimeState.PrivateSubnetIDs
	scaleSetup := mapScaleToAWSEKSSetup(config.Global.Scale)
	s.variables.NodeInstanceType = scaleSetup.General.InstanceType
	s.variables.DesiredSize = scaleSetup.General.DesiredSize
	s.variables.MinSize = scaleSetup.General.MinSize
	s.variables.MaxSize = scaleSetup.General.MaxSize
	s.variables.MaxPods = scaleSetup.General.MaxPods
	s.variables.VolumeSize = scaleSetup.General.VolumeSize
	s.variables.VolumeType = scaleSetup.General.VolumeType
	s.variables.AdditionalNodeGroups = make(map[string]EKSNodeGroup)
	s.variables.AdditionalNodeGroups["observability"] = scaleSetup.O11y
	s.variables.EnableCacheRegistry = config.AWS.CacheRegistry != ""
	s.variables.CacheRegistry = config.AWS.CacheRegistry
	s.variables.HTTPProxy = config.Proxy.HTTPProxy
	s.variables.HTTPSProxy = config.Proxy.HTTPSProxy
	s.variables.NoProxy = config.Proxy.NoProxy

	s.backendConfig.Key = EKSBackendBucketKey
	s.backendConfig.Region = config.AWS.Region
	s.backendConfig.Bucket = config.Global.OrchName + "-" + runtimeState.DeploymentID
	return runtimeState, nil
}

func (s *EKSStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if config.AWS.PreviousS3StateBucket == "" {
		// No need to migrate state, since there is no previous state bucket
		return runtimeState, nil
	}
	// Need to move Terraform state from old bucket to new bucket:
	oldEKSBucketKey := fmt.Sprintf("%s/cluster/%s", config.AWS.Region, config.Global.OrchName)
	err := s.AWSUtility.S3CopyToS3(config.AWS.Region,
		config.AWS.PreviousS3StateBucket,
		oldEKSBucketKey,
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
	modulePath := filepath.Join(s.RootPath, EKSModulePath)
	mvErr := s.TerraformUtility.MoveStates(ctx, steps.TerraformUtilityMoveStatesInput{
		ModulePath: modulePath,
		States: map[string]string{
			"module.eks.aws_iam_role.iam_role_eks_cluster":                                   "aws_iam_role.iam_role_eks_cluster",
			"module.eks.aws_iam_role_policy_attachment.eks_cluster_AmazonEKSClusterPolicy":   "aws_iam_role_policy_attachment.eks_cluster_AmazonEKSClusterPolicy",
			"module.eks.aws_iam_role_policy_attachment.eks_cluster_AmazonEKSServicePolicy":   "aws_iam_role_policy_attachment.eks_cluster_AmazonEKSServicePolicy",
			"module.eks.aws_iam_role_policy_attachment.AmazonEKSWorkerNodePolicy":            "aws_iam_role_policy_attachment.AmazonEKSWorkerNodePolicy",
			"module.eks.aws_iam_role_policy_attachment.AmazonEKS_CNI_Policy":                 "aws_iam_role_policy_attachment.AmazonEKS_CNI_Policy",
			"module.eks.aws_iam_role_policy_attachment.AmazonEC2ContainerRegistryReadOnly":   "aws_iam_role_policy_attachment.AmazonEC2ContainerRegistryReadOnly",
			"module.eks.aws_iam_role_policy_attachment.AmazonEBSCSIDriverPolicy":             "aws_iam_role_policy_attachment.AmazonEBSCSIDriverPolicy",
			"module.eks.aws_iam_role_policy_attachment.AmazonEFSCSIDriverPolicy":             "aws_iam_role_policy_attachment.AmazonEFSCSIDriverPolicy",
			"module.eks.aws_iam_role_policy_attachment.AmazonSSMManagedInstanceCore":         "aws_iam_role_policy_attachment.AmazonSSMManagedInstanceCore",
			"module.eks.aws_iam_role_policy_attachment.ELB_Controller":                       "aws_iam_role_policy_attachment.ELB_Controller",
			"module.eks.aws_iam_role_policy_attachment.additional_policy":                    "aws_iam_role_policy_attachment.additional_policy",
			"module.eks.aws_iam_role.eks_nodes":                                              "aws_iam_role.eks_nodes",
			"module.eks.aws_iam_role.cas_controller":                                         "aws_iam_role.cas_controller",
			"module.eks.aws_iam_role_policy_attachment.cas_controller":                       "aws_iam_role_policy_attachment.cas_controller",
			"module.eks.aws_iam_role.certmgr":                                                "aws_iam_role.certmgr",
			"module.eks.aws_iam_role_policy_attachment.certmgr_AmazonSSMManagedInstanceCore": "aws_iam_role_policy_attachment.certmgr_AmazonSSMManagedInstanceCore",
			"module.eks.aws_iam_role_policy_attachment.certmgr_acm_sync_certmgr":             "aws_iam_role_policy_attachment.certmgr_acm_sync_certmgr",
			"module.eks.aws_iam_role_policy_attachment.certmgr_acm_sync_eks_node":            "aws_iam_role_policy_attachment.certmgr_acm_sync_eks_node",
			"module.eks.aws_iam_role_policy_attachment.certmgr_write_route53":                "aws_iam_role_policy_attachment.certmgr_write_route53",
			"module.eks.aws_iam_policy.certmgr_acm_sync":                                     "aws_iam_policy.certmgr_acm_sync",
			"module.eks.aws_iam_policy.certmgr_write_route53":                                "aws_iam_policy.certmgr_write_route53",
			"module.eks.aws_iam_openid_connect_provider.cluster":                             "aws_iam_openid_connect_provider.cluster",
			"module.eks.aws_iam_policy.cas_controller":                                       "aws_iam_policy.cas_controller",
			"module.eks.aws_security_group.eks_cluster":                                      "aws_security_group.eks_cluster",
			"module.eks.aws_eks_cluster.eks_cluster":                                         "aws_eks_cluster.eks_cluster",
			"module.eks.aws_eks_node_group.nodegroup":                                        "aws_eks_node_group.nodegroup",
			"module.eks.aws_eks_node_group.additional_node_group":                            "aws_eks_node_group.additional_node_group",
			"module.eks.aws_eks_addon.addons":                                                "aws_eks_addon.addons",
			"module.eks.null_resource.wait_eks_complete":                                     "null_resource.wait_eks_complete",
			"module.eks.null_resource.create_kubecnofig":                                     "null_resource.create_kubecnofig",
			"module.eks.null_resource.set_env":                                               "null_resource.set_env",
			"module.eks.aws_launch_template.eks_launch_template":                             "aws_launch_template.eks_launch_template",
			"module.eks.aws_launch_template.additional_node_group_launch_template":           "aws_launch_template.additional_node_group_launch_template",
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
			"module.efs",
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

func (s *EKSStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	terraformStepInput := steps.TerraformUtilityInput{
		Action:             runtimeState.Action,
		ModulePath:         filepath.Join(s.RootPath, EKSModulePath),
		Variables:          s.variables,
		BackendConfig:      s.backendConfig,
		LogFile:            filepath.Join(runtimeState.LogDir, "aws_eks.log"),
		KeepGeneratedFiles: s.KeepGeneratedFiles,
	}
	// No output from EKS module for now
	_, err := s.TerraformUtility.Run(ctx, terraformStepInput)
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeTerraform,
			ErrorMsg:  fmt.Sprintf("failed to run terraform: %v", err),
		}
	}
	return runtimeState, nil
}

func (s *EKSStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

type EKSScaleSetup struct {
	General EKSNodeGroup
	O11y    EKSNodeGroup
}

func mapScaleToAWSEKSSetup(scale config.Scale) EKSScaleSetup {
	switch scale {
	case config.Scale50:
		return EKSScaleSetup{
			General: EKSNodeGroup{
				DesiredSize:  3,
				MinSize:      3,
				MaxSize:      3,
				MaxPods:      58,
				VolumeSize:   20,
				VolumeType:   "gp3",
				InstanceType: "t3.2xlarge",
			},
			O11y: EKSNodeGroup{
				DesiredSize: 1,
				MinSize:     1,
				MaxSize:     1,
				Labels: map[string]string{
					"node.kubernetes.io/custom-rule": "observability",
				},
				Taints: map[string]EKSTaint{
					"node.kubernetes.io/custom-rule": {
						Value:  "observability",
						Effect: "NO_SCHEDULE",
					},
				},
				InstanceType: "t3.2xlarge",
				VolumeSize:   20,
				VolumeType:   "gp3",
			},
		}
	case config.Scale100:
		return EKSScaleSetup{
			General: EKSNodeGroup{
				DesiredSize:  3,
				MinSize:      3,
				MaxSize:      3,
				MaxPods:      58,
				VolumeSize:   128,
				VolumeType:   "gp3",
				InstanceType: "t3.2xlarge",
			},
			O11y: EKSNodeGroup{
				DesiredSize: 1,
				MinSize:     1,
				MaxSize:     1,
				Labels: map[string]string{
					"node.kubernetes.io/custom-rule": "observability",
				},
				Taints: map[string]EKSTaint{
					"node.kubernetes.io/custom-rule": {
						Value:  "observability",
						Effect: "NO_SCHEDULE",
					},
				},
				InstanceType: "r5.2xlarge",
				VolumeSize:   128,
				VolumeType:   "gp3",
			},
		}
	case config.Scale500:
		return EKSScaleSetup{
			General: EKSNodeGroup{
				DesiredSize:  3,
				MinSize:      3,
				MaxSize:      3,
				MaxPods:      58,
				VolumeSize:   128,
				VolumeType:   "gp3",
				InstanceType: "t3.2xlarge",
			},
			O11y: EKSNodeGroup{
				DesiredSize: 2,
				MinSize:     2,
				MaxSize:     2,
				Labels: map[string]string{
					"node.kubernetes.io/custom-rule": "observability",
				},
				Taints: map[string]EKSTaint{
					"node.kubernetes.io/custom-rule": {
						Value:  "observability",
						Effect: "NO_SCHEDULE",
					},
				},
				InstanceType: "r5.4xlarge",
				VolumeSize:   128,
				VolumeType:   "gp3",
			},
		}
	case config.Scale1000:
		return EKSScaleSetup{
			General: EKSNodeGroup{
				DesiredSize:  3,
				MinSize:      3,
				MaxSize:      3,
				MaxPods:      58,
				VolumeSize:   128,
				VolumeType:   "gp3",
				InstanceType: "t3.2xlarge",
			},
			O11y: EKSNodeGroup{
				DesiredSize: 2,
				MinSize:     2,
				MaxSize:     2,
				Labels: map[string]string{
					"node.kubernetes.io/custom-rule": "observability",
				},
				Taints: map[string]EKSTaint{
					"node.kubernetes.io/custom-rule": {
						Value:  "observability",
						Effect: "NO_SCHEDULE",
					},
				},
				InstanceType: "r5.4xlarge",
				VolumeSize:   128,
				VolumeType:   "gp3",
			},
		}
	}
	return EKSScaleSetup{}
}
