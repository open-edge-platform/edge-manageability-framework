// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_vpc_test

import (
	"crypto/rand"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/aws"
	"github.com/gruntwork-io/terratest/modules/terraform"
	steps_aws "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/aws"
	"github.com/open-edge-platform/edge-manageability-framework/installer/targets/aws/iac/utils"
	"github.com/stretchr/testify/suite"
)

type VPCTestSuite struct {
	suite.Suite
}

func TestVPCTestSuite(t *testing.T) {
	suite.Run(t, new(VPCTestSuite))
}

func (s *VPCTestSuite) TestApplyingModule() {
	// Bucket for VPC state
	randomPostfix := strings.ToLower(rand.Text()[:8])
	bucketName := "test-bucket-" + randomPostfix
	aws.CreateS3Bucket(s.T(), utils.DefaultTestRegion, bucketName)
	defer func() {
		aws.EmptyS3Bucket(s.T(), utils.DefaultTestRegion, bucketName)
		aws.DeleteS3Bucket(s.T(), utils.DefaultTestRegion, bucketName)
	}()

	_, publicSSHKey, _ := utils.GenerateSSHKeyPair()
	variables := steps_aws.VPCVariables{
		Region:             utils.DefaultTestRegion,
		Name:               "test-vpc-" + randomPostfix,
		CidrBlock:          "10.250.0.0/16",
		EnableDnsHostnames: true,
		EnableDnsSupport:   true,
		PrivateSubnets: map[string]steps_aws.VPCSubnet{
			"private-subnet-1": {
				Az:        "us-west-2a",
				CidrBlock: "10.250.0.0/22",
			},
		},
		PublicSubnets: map[string]steps_aws.VPCSubnet{
			"public-subnet-1": {
				Az:        "us-west-2a",
				CidrBlock: "10.250.4.0/24",
			},
		},
		EndpointSGName: "test-vpc-" + randomPostfix + "-ep-sg",
		Endpoints: []string{
			"elasticfilesystem",
			"s3",
		},
		JumphostIPAllowList:    []string{"10.0.0.0/16"},
		JumphostInstanceSSHKey: publicSSHKey,
		JumphostSubnet:         "public-subnet-1",
		Production:             true,
		CustomerTag:            utils.DefaultTestCustomerTag,
	}

	jsonData, err := json.Marshal(variables)
	if err != nil {
		s.T().Fatalf("Failed to marshal variables: %v", err)
	}
	tempFile, err := os.CreateTemp("", "variables-*.tfvar.json")
	if err != nil {
		s.T().Fatalf("Failed to create temporary file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	if _, err := tempFile.Write(jsonData); err != nil {
		s.T().Fatalf("Failed to write to temporary file: %v", err)
	}

	terraformOptions := terraform.WithDefaultRetryableErrors(s.T(), &terraform.Options{
		TerraformDir: "../vpc",
		VarFiles:     []string{tempFile.Name()},
		BackendConfig: map[string]interface{}{
			"region": utils.DefaultTestRegion,
			"bucket": bucketName,
			"key":    "vpc.tfstate",
		},
		Reconfigure: true,
		Upgrade:     true,
	})
	defer terraform.Destroy(s.T(), terraformOptions)
	terraform.InitAndApply(s.T(), terraformOptions)

	vpcID, err := terraform.OutputE(s.T(), terraformOptions, "vpc_id")
	if err != nil {
		s.NoError(err, "Failed to get VPC ID from Terraform output")
		return
	}
	vpc := aws.GetVpcById(s.T(), vpcID, utils.DefaultTestRegion)
	s.Equal("10.250.0.0/16", *vpc.CidrBlock, "VPC CIDR block does not match expected value")
	s.NotEmpty(vpcID, "VPC ID should not be empty")
	privateSubnets := terraform.OutputMapOfObjects(s.T(), terraformOptions, "private_subnets")
	privateSubnet, ok := privateSubnets["private-subnet-1"].(map[string]interface{})
	s.True(ok, "Expected private subnet to be a map of objects")
	sid, ok := privateSubnet["id"].(string)
	s.True(ok, "Expected private subnet ID to be a string")
	s.NotEmpty(sid, "Private subnet ID should not be empty")

	publicSubnetIDs := terraform.OutputMapOfObjects(s.T(), terraformOptions, "public_subnets")
	publicSubnet, ok := publicSubnetIDs["public-subnet-1"].(map[string]interface{})
	s.True(ok, "Expected public subnet to be a map of objects")
	sid, ok = publicSubnet["id"].(string)
	s.True(ok, "Expected public subnet ID to be a string")
	s.NotEmpty(sid, "Public subnet ID should not be empty")

	ec2Filters := map[string][]string{
		"tag:Name": {"test-vpc-" + randomPostfix + "-jump"},
		"tag:VPC":  {"test-vpc-" + randomPostfix},
	}
	instanceIDs := aws.GetEc2InstanceIdsByFilters(s.T(), utils.DefaultTestRegion, ec2Filters)
	s.NotEmpty(instanceIDs, "No EC2 instances found with the specified filters")

	_, err = utils.GetInternetGatewaysByTags(utils.DefaultTestRegion, map[string][]string{
		"Name": {"test-vpc-" + randomPostfix + "-igw"},
		"VPC":  {"test-vpc-" + randomPostfix},
	})
	if err != nil {
		s.NoError(err, "Failed to get Internet Gateway for VPC")
		return
	}

	_, err = utils.GetNATGatewaysByTags(utils.DefaultTestRegion, map[string][]string{
		"VPC": {"test-vpc-" + randomPostfix},
	})
	if err != nil {
		s.NoError(err, "Failed to get NAT Gateway for VPC")
		return
	}
}
