// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_state_bucket_test

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

const SSKKeySize = 2048

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
	aws.CreateS3Bucket(s.T(), "us-west-2", bucketName)
	defer func() {
		aws.EmptyS3Bucket(s.T(), "us-west-2", bucketName)
		aws.DeleteS3Bucket(s.T(), "us-west-2", bucketName)
	}()

	_, publicSSHKey, _ := GenerateSSHKeyPair()
	variables := AWSVPCVariables{
		Region:             "us-west-2",
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
		CustomerTag:            "test-customer",
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
			"region": "us-west-2",
			"bucket": bucketName,
			"key":    "vpc.tfstate",
		},
	})
	defer terraform.Destroy(s.T(), terraformOptions)
	terraform.InitAndApply(s.T(), terraformOptions)

	// TODO: Check VPC and subnets were created successfully
}

func GenerateSSHKeyPair() (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, SSKKeySize)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %v", err)
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
