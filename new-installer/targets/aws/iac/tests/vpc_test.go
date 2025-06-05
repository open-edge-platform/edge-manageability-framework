// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/aws"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/suite"
	"golang.org/x/crypto/ssh"
)

const (
	JumphostSSHKeySize = 2048
)

type VPCTestSuite struct {
	suite.Suite
}

func TestVPCTestSuite(t *testing.T) {
	suite.Run(t, new(VPCTestSuite))
}

type AWSVPCVariables struct {
	Region                 string                  `json:"region" yaml:"region"`
	Name                   string                  `json:"name" yaml:"name"`
	CidrBlock              string                  `json:"cidr_block" yaml:"cidr_block"`
	EnableDnsHostnames     bool                    `json:"enable_dns_hostnames" yaml:"enable_dns_hostnames"`
	EnableDnsSupport       bool                    `json:"enable_dns_support" yaml:"enable_dns_support"`
	PrivateSubnets         map[string]AWSVPCSubnet `json:"private_subnets" yaml:"private_subnets"`
	PublicSubnets          map[string]AWSVPCSubnet `json:"public_subnets" yaml:"public_subnets"`
	EndpointSGName         string                  `json:"endpoint_sg_name" yaml:"endpoint_sg_name"`
	JumphostIPAllowList    []string                `json:"jumphost_ip_allow_list" yaml:"jumphost_ip_allow_list"`
	JumphostInstanceSSHKey string                  `json:"jumphost_instance_ssh_key_pub" yaml:"jumphost_instance_ssh_key_pub"`
	JumphostSubnet         string                  `json:"jumphost_subnet" yaml:"jumphost_subnet"`
	Production             bool                    `json:"production" yaml:"production"`
	CustomerTag            string                  `json:"customer_tag" yaml:"customer_tag"`
}

type AWSVPCSubnet struct {
	Az        string `json:"az" yaml:"az"`
	CidrBlock string `json:"cidr_block" yaml:"cidr_block"`
}

func (s *VPCTestSuite) TestApplyingModule() {
	// Bucket for VPC state
	randomPostfix := strings.ToLower(rand.Text()[:8])
	bucketName := "test-bucket-" + randomPostfix
	aws.CreateS3Bucket(s.T(), DefaultRegion, bucketName)
	defer func() {
		aws.EmptyS3Bucket(s.T(), DefaultRegion, bucketName)
		aws.DeleteS3Bucket(s.T(), DefaultRegion, bucketName)
	}()

	_, publicSSHKey, _ := GenerateSSHKeyPair()
	variables := AWSVPCVariables{
		Region:             DefaultRegion,
		Name:               "test-vpc-" + randomPostfix,
		CidrBlock:          "10.250.0.0/16",
		EnableDnsHostnames: true,
		EnableDnsSupport:   true,
		PrivateSubnets: map[string]AWSVPCSubnet{
			"private-subnet-1": {
				Az:        "us-west-2a",
				CidrBlock: "10.250.0.0/22",
			},
		},
		PublicSubnets: map[string]AWSVPCSubnet{
			"public-subnet-1": {
				Az:        "us-west-2a",
				CidrBlock: "10.250.4.0/24",
			},
		},
		EndpointSGName:         "test-vpc-" + randomPostfix + "-ep-sg",
		JumphostIPAllowList:    []string{"10.0.0.0/16"},
		JumphostInstanceSSHKey: publicSSHKey,
		JumphostSubnet:         "public-subnet-1",
		Production:             true,
		CustomerTag:            "unit-test",
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
			"region": DefaultRegion,
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
	vpc := aws.GetVpcById(s.T(), vpcID, DefaultRegion)
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
	instanceIDs := aws.GetEc2InstanceIdsByFilters(s.T(), DefaultRegion, ec2Filters)
	s.NotEmpty(instanceIDs, "No EC2 instances found with the specified filters")

	_, err = GetInternetGatewaysByTags(DefaultRegion, map[string][]string{
		"Name": {"test-vpc-" + randomPostfix + "-igw"},
		"VPC":  {"test-vpc-" + randomPostfix},
	})
	if err != nil {
		s.NoError(err, "Failed to get Internet Gateway for VPC")
		return
	}

	_, err = GetNATGatewaysByTags(DefaultRegion, map[string][]string{
		"VPC": {"test-vpc-" + randomPostfix},
	})
	if err != nil {
		s.NoError(err, "Failed to get NAT Gateway for VPC")
		return
	}
}

func GenerateSSHKeyPair() (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, JumphostSSHKeySize)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}
	privateKeyString := string(pem.EncodeToMemory(privateKeyPEM))
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", err
	}
	publicKeyString := string(ssh.MarshalAuthorizedKey(pub))
	return privateKeyString, publicKeyString, nil
}
