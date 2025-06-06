// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package steps_aws_test

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	steps_aws "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/aws"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type EKSStepTest struct {
	suite.Suite
	config       config.OrchInstallerConfig
	runtimeState config.OrchInstallerRuntimeState
	step         *steps_aws.EKSStep
	randomText   string
	logDir       string
	tfUtility    *MockTerraformUtility
	awsUtility   *MockAWSUtility
}

func TestEKSStep(t *testing.T) {
	suite.Run(t, new(EKSStepTest))
}

func (s *EKSStepTest) SetupTest() {
	rootPath, err := filepath.Abs("../../../../")
	if err != nil {
		s.NoError(err)
		return
	}
	s.randomText = strings.ToLower(rand.Text()[0:8])
	s.logDir = filepath.Join(rootPath, ".logs")
	err = internal.InitLogger("debug", s.logDir)
	if err != nil {
		s.NoError(err)
		return
	}
	s.config.AWS.Region = "us-west-2"
	s.config.Global.Scale = config.Scale50
	s.config.Global.OrchName = "test"
	s.config.AWS.CacheRegistry = "test-cache-registry"
	s.runtimeState.DeploymentID = s.randomText
	s.runtimeState.LogDir = filepath.Join(rootPath, ".logs")
	s.runtimeState.AWS.VPCID = "vpc-12345678"
	s.runtimeState.AWS.PrivateSubnetIDs = []string{"subnet-12345678", "subnet-23456789", "subnet-34567890"}

	if _, err := os.Stat(s.logDir); os.IsNotExist(err) {
		err := os.MkdirAll(s.logDir, os.ModePerm)
		if err != nil {
			s.NoError(err)
			return
		}
	}
	s.tfUtility = &MockTerraformUtility{}
	s.awsUtility = &MockAWSUtility{}
	s.step = &steps_aws.EKSStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: true,
		TerraformUtility:   s.tfUtility,
		AWSUtility:         s.awsUtility,
	}
}

func (s *EKSStepTest) TestInstallAndUninstallEKS() {
	s.runtimeState.Action = "install"
	s.expectUtiliyCall("install")
	rs, err := steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}
	s.Equal("Mock OIDC Issuer", rs.AWS.EKSOIDCIssuer)

	s.runtimeState.Action = "uninstall"
	s.expectUtiliyCall("uninstall")
	rs, err = steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}
	s.Equal("", rs.AWS.EKSOIDCIssuer)
}

func (s *EKSStepTest) TestUpgradeEKS() {
	s.runtimeState.Action = "upgrade"
	s.config.AWS.PreviousS3StateBucket = "old-state-bucket"
	s.expectUtiliyCall("upgrade")
	rs, err := steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}
	s.Equal("Mock OIDC Issuer", rs.AWS.EKSOIDCIssuer)
}

func (s *EKSStepTest) expectUtiliyCall(action string) {
	expectTfOutput := steps.TerraformUtilityOutput{}
	if action == "install" || action == "upgrade" {
		expectTfOutput.Output = map[string]tfexec.OutputMeta{
			"eks_oidc_issuer": {
				Type:  json.RawMessage(`"string"`),
				Value: json.RawMessage(`"Mock OIDC Issuer"`),
			},
		}
	}
	if action == "upgrade" {
		s.awsUtility.On("S3CopyToS3",
			s.config.AWS.Region,
			s.config.AWS.PreviousS3StateBucket,
			fmt.Sprintf("%s/cluster/%s", s.config.AWS.Region, s.config.Global.OrchName),
			s.config.AWS.Region,
			s.config.Global.OrchName+"-"+s.runtimeState.DeploymentID,
			"eks.tfstate",
		).Return(nil).Once()
		s.tfUtility.On("MoveStates", mock.Anything, steps.TerraformUtilityMoveStatesInput{
			ModulePath: filepath.Join(s.step.RootPath, steps_aws.EKSModulePath),
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
		}).Return(nil).Once()
		s.tfUtility.On("RemoveStates", mock.Anything, steps.TerraformUtilityRemoveStatesInput{
			ModulePath: filepath.Join(s.step.RootPath, steps_aws.EKSModulePath),
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
		}).Return(nil).Once()
	}
	expectedVariables := steps_aws.EKSVariables{
		EKSVersion: "1.32",
		AddOns: []steps_aws.EKSAddOn{
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
		},
		UserScriptPreCloudInit:  "",
		UserScriptPostCloudInit: "",
		Name:                    "test",
		Region:                  "us-west-2",
		VPCID:                   "vpc-12345678",
		CustomerTag:             "",
		SubnetIDs:               []string{"subnet-12345678", "subnet-23456789", "subnet-34567890"},
		NodeInstanceType:        "t3.2xlarge",
		DesiredSize:             3,
		MinSize:                 3,
		MaxSize:                 3,
		MaxPods:                 58,
		VolumeSize:              20,
		VolumeType:              "gp3",
		AdditionalNodeGroups: map[string]steps_aws.EKSNodeGroup{
			"observability": {
				InstanceType: "t3.2xlarge",
				DesiredSize:  1,
				MinSize:      1,
				MaxSize:      1,
				VolumeSize:   20,
				VolumeType:   "gp3",
				Taints: map[string]steps_aws.EKSTaint{
					"node.kubernetes.io/custom-rule": {
						Value:  "observability",
						Effect: "NO_SCHEDULE",
					},
				},
				Labels: map[string]string{
					"node.kubernetes.io/custom-rule": "observability",
				},
			},
		},
		EnableCacheRegistry: true,
		CacheRegistry:       "test-cache-registry",
		HTTPProxy:           "",
		HTTPSProxy:          "",
		NoProxy:             "",
	}

	expectedBackendConfig := steps_aws.TerraformAWSBucketBackendConfig{
		Region: "us-west-2",
		Bucket: "test-" + s.randomText,
		Key:    "eks.tfstate",
	}

	s.tfUtility.On("Run", mock.Anything, steps.TerraformUtilityInput{
		Action:             action,
		ModulePath:         filepath.Join(s.step.RootPath, steps_aws.EKSModulePath),
		Variables:          expectedVariables,
		BackendConfig:      expectedBackendConfig,
		LogFile:            filepath.Join(s.runtimeState.LogDir, "aws_eks.log"),
		KeepGeneratedFiles: true,
	}).Return(expectTfOutput, nil).Once()
}
