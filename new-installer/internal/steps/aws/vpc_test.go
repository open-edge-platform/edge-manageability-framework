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

type VPCStepTest struct {
	suite.Suite
	config       config.OrchInstallerConfig
	runtimeState config.OrchInstallerRuntimeState
	step         *steps_aws.VPCStep
	randomText   string
	logDir       string
	tfUtility    *MockTerraformUtility
	awsUtility   *MockAWSUtility
}

func TestVPCStep(t *testing.T) {
	suite.Run(t, new(VPCStepTest))
}

func (s *VPCStepTest) SetupTest() {
	rootPath, err := filepath.Abs("../../../../")
	if err != nil {
		s.NoError(err)
		return
	}
	s.randomText = strings.ToLower(rand.Text()[0:8])
	s.logDir = filepath.Join(rootPath, ".logs")
	if err := internal.InitLogger("debug", s.logDir); err != nil {
		s.NoError(err)
		return
	}
	s.config.AWS.Region = "us-west-2"
	s.config.Global.OrchName = "test"
	s.runtimeState.DeploymentID = s.randomText
	s.config.AWS.JumpHostWhitelist = []string{"10.250.0.0/16"}
	s.runtimeState.LogDir = filepath.Join(rootPath, ".logs")
	s.runtimeState.AWS.JumpHostSSHKeyPrivateKey = "foobar"
	s.runtimeState.AWS.JumpHostSSHKeyPublicKey = "foobar"

	if _, err := os.Stat(s.logDir); os.IsNotExist(err) {
		err := os.MkdirAll(s.logDir, os.ModePerm)
		if err != nil {
			s.NoError(err)
			return
		}
	}
	s.tfUtility = &MockTerraformUtility{}
	s.awsUtility = &MockAWSUtility{}
	s.step = &steps_aws.VPCStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: true,
		TerraformUtility:   s.tfUtility,
		AWSUtility:         s.awsUtility,
	}
}

func (s *VPCStepTest) TestInstallAndUninstallVPC() {
	s.runtimeState.Action = "install"
	s.expectUtiliyCall("install")
	rs, err := steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}

	s.Equal("vpc-12345678", rs.AWS.VPCID)
	s.ElementsMatch([]string{
		"subnet-1",
		"subnet-2",
		"subnet-3",
	}, rs.AWS.PrivateSubnetIDs)
	s.ElementsMatch([]string{
		"subnet-4",
		"subnet-5",
		"subnet-6",
	}, rs.AWS.PublicSubnetIDs)

	s.runtimeState.Action = "uninstall"
	s.expectUtiliyCall("uninstall")
	_, err = steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
	}
}

func (s *VPCStepTest) TestUpgradeVPC() {
	s.runtimeState.Action = "upgrade"
	s.config.AWS.PreviousS3StateBucket = "old-bucket-name"
	// We will mostlly test if the prestep make correct calls to AWS and Terraform utilities.
	s.expectUtiliyCall("upgrade")
	rs, err := steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}
	s.Equal("vpc-12345678", rs.AWS.VPCID)
	s.ElementsMatch([]string{
		"subnet-1",
		"subnet-2",
		"subnet-3",
	}, rs.AWS.PrivateSubnetIDs)
	s.ElementsMatch([]string{
		"subnet-4",
		"subnet-5",
		"subnet-6",
	}, rs.AWS.PublicSubnetIDs)
}

func (s *VPCStepTest) expectUtiliyCall(action string) {
	s.awsUtility.On("GetAvailableZones", "us-west-2").Return([]string{"us-west-2a", "us-west-2b", "us-west-2c"}, nil).Once()
	input := steps.TerraformUtilityInput{
		Action:             action,
		ModulePath:         filepath.Join(s.step.RootPath, steps_aws.VPCModulePath),
		LogFile:            filepath.Join(s.logDir, "aws_vpc.log"),
		KeepGeneratedFiles: s.step.KeepGeneratedFiles,
		Variables: steps_aws.VPCVariables{
			Name:               s.config.Global.OrchName,
			Region:             s.config.AWS.Region,
			CidrBlock:          "10.250.0.0/16",
			EnableDnsHostnames: true,
			EnableDnsSupport:   true,
			JumphostIPAllowList: []string{
				"10.250.0.0/16",
			},
			JumphostInstanceSSHKey: "foobar",
			Production:             true,
			CustomerTag:            "",
			EndpointSGName:         s.config.Global.OrchName + "-vpc-ep",
			PrivateSubnets: map[string]steps_aws.VPCSubnet{
				"subnet-us-west-2a": {
					Az:        "us-west-2a",
					CidrBlock: "10.250.0.0/22",
				},
				"subnet-us-west-2b": {
					Az:        "us-west-2b",
					CidrBlock: "10.250.4.0/22",
				},
				"subnet-us-west-2c": {
					Az:        "us-west-2c",
					CidrBlock: "10.250.8.0/22",
				},
			},
			PublicSubnets: map[string]steps_aws.VPCSubnet{
				"subnet-us-west-2a-pub": {
					Az:        "us-west-2a",
					CidrBlock: "10.250.12.0/24",
				},
				"subnet-us-west-2b-pub": {
					Az:        "us-west-2b",
					CidrBlock: "10.250.13.0/24",
				},
				"subnet-us-west-2c-pub": {
					Az:        "us-west-2c",
					CidrBlock: "10.250.14.0/24",
				},
			},
			JumphostSubnet: "subnet-us-west-2a-pub",
		},
		BackendConfig: steps_aws.TerraformAWSBucketBackendConfig{
			Region: s.config.AWS.Region,
			Bucket: fmt.Sprintf("%s-%s", s.config.Global.OrchName, s.runtimeState.DeploymentID),
			Key:    "vpc.tfstate",
		},
		TerraformState: "",
	}
	if action == "install" {
		s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
			TerraformState: "",
			Output: map[string]tfexec.OutputMeta{
				"vpc_id": {
					Type:  json.RawMessage(`"string"`),
					Value: json.RawMessage(`"vpc-12345678"`),
				},
				"private_subnets": {
					Type:  json.RawMessage(`"list"`),
					Value: json.RawMessage(`{"subnet-us-west-2a":{"id":"subnet-1"},"subnet-us-west-2b":{"id":"subnet-2"},"subnet-us-west-2c":{"id":"subnet-3"}}`),
				},
				"public_subnets": {
					Type:  json.RawMessage(`"list"`),
					Value: json.RawMessage(`{"subnet-us-west-2a-pub":{"id":"subnet-4"},"subnet-us-west-2b-pub":{"id":"subnet-5"},"subnet-us-west-2c-pub":{"id":"subnet-6"}}`),
				},
				"jumphost_ip": {
					Type:  json.RawMessage(`"string"`),
					Value: json.RawMessage(`"10.0.0.1"`),
				},
			},
		}, nil).Once()
	}
	if action == "upgrade" {
		input.Action = "uninstall"
		input.DestroyTarget = "aws_security_group_rule.jumphost_egress_https"
		s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
			TerraformState: "",
			Output:         map[string]tfexec.OutputMeta{},
		}, nil).Once()

		input.Action = "upgrade"
		input.DestroyTarget = ""
		s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
			TerraformState: "",
			Output: map[string]tfexec.OutputMeta{
				"vpc_id": {
					Type:  json.RawMessage(`"string"`),
					Value: json.RawMessage(`"vpc-12345678"`),
				},
				"private_subnets": {
					Type:  json.RawMessage(`"list"`),
					Value: json.RawMessage(`{"subnet-us-west-2a":{"id":"subnet-1"},"subnet-us-west-2b":{"id":"subnet-2"},"subnet-us-west-2c":{"id":"subnet-3"}}`),
				},
				"public_subnets": {
					Type:  json.RawMessage(`"list"`),
					Value: json.RawMessage(`{"subnet-us-west-2a-pub":{"id":"subnet-4"},"subnet-us-west-2b-pub":{"id":"subnet-5"},"subnet-us-west-2c-pub":{"id":"subnet-6"}}`),
				},
				"jumphost_ip": {
					Type:  json.RawMessage(`"string"`),
					Value: json.RawMessage(`"10.0.0.1"`),
				},
			},
		}, nil).Once()

		s.awsUtility.On("S3CopyToS3",
			s.config.AWS.Region,
			s.config.AWS.PreviousS3StateBucket,
			fmt.Sprintf("%s/vpc/%s", s.config.AWS.Region, s.config.Global.OrchName),
			s.config.AWS.Region,
			s.config.Global.OrchName+"-"+s.runtimeState.DeploymentID,
			"vpc.tfstate",
		).Return(nil).Once()

		s.tfUtility.On("MoveStates", mock.Anything, steps.TerraformUtilityMoveStatesInput{
			ModulePath: filepath.Join(s.step.RootPath, steps_aws.VPCModulePath),
			States: map[string]string{
				"module.endpoint.aws_security_group.vpc_endpoints":                                        "aws_security_group.vpc_endpoints",
				"module.endpoint.aws_vpc_endpoint.endpoint[\"ec2\"]":                                      "aws_vpc_endpoint.endpoint[\"ec2\"]",
				"module.endpoint.aws_vpc_endpoint.endpoint[\"ec2messages\"]":                              "aws_vpc_endpoint.endpoint[\"ec2messages\"]",
				"module.endpoint.aws_vpc_endpoint.endpoint[\"ecr.api\"]":                                  "aws_vpc_endpoint.endpoint[\"ecr.api\"]",
				"module.endpoint.aws_vpc_endpoint.endpoint[\"ecr.dkr\"]":                                  "aws_vpc_endpoint.endpoint[\"ecr.dkr\"]",
				"module.endpoint.aws_vpc_endpoint.endpoint[\"eks\"]":                                      "aws_vpc_endpoint.endpoint[\"eks\"]",
				"module.endpoint.aws_vpc_endpoint.endpoint[\"elasticfilesystem\"]":                        "aws_vpc_endpoint.endpoint[\"elasticfilesystem\"]",
				"module.endpoint.aws_vpc_endpoint.endpoint[\"elasticloadbalancing\"]":                     "aws_vpc_endpoint.endpoint[\"elasticloadbalancing\"]",
				"module.endpoint.aws_vpc_endpoint.endpoint[\"s3\"]":                                       "aws_vpc_endpoint.endpoint[\"s3\"]",
				"module.endpoint.aws_vpc_endpoint.endpoint[\"sts\"]":                                      "aws_vpc_endpoint.endpoint[\"sts\"]",
				"module.internet_gateway.aws_internet_gateway.igw":                                        "aws_internet_gateway.igw",
				"module.jumphost.aws_eip.jumphost":                                                        "aws_eip.jumphost",
				"module.jumphost.aws_eip_association.jumphost":                                            "aws_eip_association.jumphost",
				"module.jumphost.aws_iam_instance_profile.ec2":                                            "aws_iam_instance_profile.ec2",
				"module.jumphost.aws_iam_policy.eks_cluster_access_policy":                                "aws_iam_policy.eks_cluster_access_policy",
				"module.jumphost.aws_iam_role.ec2":                                                        "aws_iam_role.ec2",
				"module.jumphost.aws_iam_role_policy_attachment.AmazonSSMManagedInstanceCore":             "aws_iam_role_policy_attachment.AmazonSSMManagedInstanceCore",
				"module.jumphost.aws_iam_role_policy_attachment.eks_cluster_access":                       "aws_iam_role_policy_attachment.eks_cluster_access",
				"module.jumphost.aws_instance.jumphost":                                                   "aws_instance.jumphost",
				"module.jumphost.aws_key_pair.jumphost_instance_launch_key":                               "aws_key_pair.jumphost_instance_launch_key",
				"module.jumphost.aws_security_group.jumphost":                                             "aws_security_group.jumphost",
				"module.jumphost.aws_security_group_rule.jumphost_egress_https":                           "aws_security_group_rule.jumphost_egress_https",
				"module.nat_gateway.aws_eip.ngw[\"subnet-us-west-2a-pub\"]":                               "aws_eip.ngw[\"subnet-us-west-2a-pub\"]",
				"module.nat_gateway.aws_eip.ngw[\"subnet-us-west-2b-pub\"]":                               "aws_eip.ngw[\"subnet-us-west-2b-pub\"]",
				"module.nat_gateway.aws_eip.ngw[\"subnet-us-west-2c-pub\"]":                               "aws_eip.ngw[\"subnet-us-west-2c-pub\"]",
				"module.nat_gateway.aws_nat_gateway.ngw_with_eip[\"subnet-us-west-2a-pub\"]":              "aws_nat_gateway.main[\"subnet-us-west-2a-pub\"]",
				"module.nat_gateway.aws_nat_gateway.ngw_with_eip[\"subnet-us-west-2b-pub\"]":              "aws_nat_gateway.main[\"subnet-us-west-2b-pub\"]",
				"module.nat_gateway.aws_nat_gateway.ngw_with_eip[\"subnet-us-west-2c-pub\"]":              "aws_nat_gateway.main[\"subnet-us-west-2c-pub\"]",
				"module.route_table.aws_route_table.private_subnet[\"subnet-us-west-2a\"]":                "aws_route_table.private_subnet[\"subnet-us-west-2a\"]",
				"module.route_table.aws_route_table.private_subnet[\"subnet-us-west-2b\"]":                "aws_route_table.private_subnet[\"subnet-us-west-2b\"]",
				"module.route_table.aws_route_table.private_subnet[\"subnet-us-west-2c\"]":                "aws_route_table.private_subnet[\"subnet-us-west-2c\"]",
				"module.route_table.aws_route_table.public_subnet[\"subnet-us-west-2a-pub\"]":             "aws_route_table.public_subnet[\"subnet-us-west-2a-pub\"]",
				"module.route_table.aws_route_table.public_subnet[\"subnet-us-west-2b-pub\"]":             "aws_route_table.public_subnet[\"subnet-us-west-2b-pub\"]",
				"module.route_table.aws_route_table.public_subnet[\"subnet-us-west-2c-pub\"]":             "aws_route_table.public_subnet[\"subnet-us-west-2c-pub\"]",
				"module.route_table.aws_route_table_association.private_subnet[\"subnet-us-west-2a\"]":    "aws_route_table_association.private_subnet[\"subnet-us-west-2a\"]",
				"module.route_table.aws_route_table_association.private_subnet[\"subnet-us-west-2b\"]":    "aws_route_table_association.private_subnet[\"subnet-us-west-2b\"]",
				"module.route_table.aws_route_table_association.private_subnet[\"subnet-us-west-2c\"]":    "aws_route_table_association.private_subnet[\"subnet-us-west-2c\"]",
				"module.route_table.aws_route_table_association.public_subnet[\"subnet-us-west-2a-pub\"]": "aws_route_table_association.public_subnet[\"subnet-us-west-2a-pub\"]",
				"module.route_table.aws_route_table_association.public_subnet[\"subnet-us-west-2b-pub\"]": "aws_route_table_association.public_subnet[\"subnet-us-west-2b-pub\"]",
				"module.route_table.aws_route_table_association.public_subnet[\"subnet-us-west-2c-pub\"]": "aws_route_table_association.public_subnet[\"subnet-us-west-2c-pub\"]",
				"module.vpc.aws_subnet.private_subnet[\"subnet-us-west-2a\"]":                             "aws_subnet.private_subnet[\"subnet-us-west-2a\"]",
				"module.vpc.aws_subnet.private_subnet[\"subnet-us-west-2b\"]":                             "aws_subnet.private_subnet[\"subnet-us-west-2b\"]",
				"module.vpc.aws_subnet.private_subnet[\"subnet-us-west-2c\"]":                             "aws_subnet.private_subnet[\"subnet-us-west-2c\"]",
				"module.vpc.aws_subnet.public_subnet[\"subnet-us-west-2a-pub\"]":                          "aws_subnet.public_subnet[\"subnet-us-west-2a-pub\"]",
				"module.vpc.aws_subnet.public_subnet[\"subnet-us-west-2b-pub\"]":                          "aws_subnet.public_subnet[\"subnet-us-west-2b-pub\"]",
				"module.vpc.aws_subnet.public_subnet[\"subnet-us-west-2c-pub\"]":                          "aws_subnet.public_subnet[\"subnet-us-west-2c-pub\"]",
				"module.vpc.main": "aws_vpc.main",
			},
		}).Return(nil).Once()

	}
	if action == "uninstall" {
		s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
			TerraformState: "",
			Output:         map[string]tfexec.OutputMeta{},
		}, nil).Once()
	}
}
