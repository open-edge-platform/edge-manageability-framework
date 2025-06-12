// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_test

import (
	"crypto/rand"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	terratest_aws "github.com/gruntwork-io/terratest/modules/aws"
	"github.com/gruntwork-io/terratest/modules/terraform"
	steps_aws "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/aws"
	"github.com/open-edge-platform/edge-manageability-framework/installer/targets/aws/iac/utils"
	"github.com/stretchr/testify/suite"
)

type EKSTestSuite struct {
	suite.Suite
	name             string // for everything, such as vpc, bucket, eks cluster, etc.
	vpcID            string
	publicSubnetIDs  []string
	privateSubnetIDs []string
}

func TestEKSTestSuite(t *testing.T) {
	suite.Run(t, new(EKSTestSuite))
}

func (s *EKSTestSuite) SetupTest() {
	// Bucket for EKS state
	s.name = "eks-unit-test-" + strings.ToLower(rand.Text()[0:8])
	terratest_aws.CreateS3Bucket(s.T(), utils.DefaultTestRegion, s.name)

	// VPC and subnets for EKS
	var err error
	var jumphostPrivateKey, jumphostIP string
	s.vpcID, s.publicSubnetIDs, s.privateSubnetIDs, jumphostPrivateKey, jumphostIP, err = utils.CreateVPCWithEndpoints(s.T(), s.name, []string{"ec2"})
	if err != nil {
		s.NoError(err, "Failed to create VPC and subnet")
		return
	}
	err = utils.StartSshuttle(jumphostIP, jumphostPrivateKey, steps_aws.DefaultNetworkCIDR)
	if err != nil {
		s.NoError(err, "Failed to start sshuttle")
		return
	}
}

func (s *EKSTestSuite) TearDownTest() {
	err := utils.StopSshuttle()
	if err != nil {
		s.NoError(err, "Failed to stop sshuttle")
	}

	err = utils.DeleteVPC(s.T(), s.name)
	if err != nil {
		s.NoError(err, "Failed to delete VPC")
		return
	}

	terratest_aws.EmptyS3Bucket(s.T(), utils.DefaultTestRegion, s.name)
	terratest_aws.DeleteS3Bucket(s.T(), utils.DefaultTestRegion, s.name)
}

func (s *EKSTestSuite) TestApplyingModule() {
	eksVars := steps_aws.EKSVariables{
		Name:                s.name,
		Region:              utils.DefaultTestRegion,
		VPCID:               s.vpcID,
		CustomerTag:         utils.DefaultTestCustomerTag,
		SubnetIDs:           s.privateSubnetIDs,
		EKSVersion:          "1.32",
		NodeInstanceType:    "t3.medium",
		DesiredSize:         1,
		MinSize:             1,
		MaxSize:             1,
		MaxPods:             58,
		VolumeSize:          20,
		VolumeType:          "gp3",
		EnableCacheRegistry: true,
		CacheRegistry:       "",
		HTTPProxy:           "",
		HTTPSProxy:          "",
		NoProxy:             "",
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
		AdditionalNodeGroups: map[string]steps_aws.EKSNodeGroup{},
	}

	jsonData, err := json.Marshal(eksVars)
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
		TerraformDir: ".",
		VarFiles:     []string{tempFile.Name()},
		BackendConfig: map[string]interface{}{
			"region": utils.DefaultTestRegion,
			"bucket": s.name,
			"key":    "eks.tfstate",
		},
		Reconfigure: true,
		Upgrade:     true,
	})

	defer terraform.Destroy(s.T(), terraformOptions)
	terraform.InitAndApply(s.T(), terraformOptions)
	time.Sleep(5 * time.Minute) // Debug. TODO: to be removed
	eksOIDCIssuer := terraform.Output(s.T(), terraformOptions, "eks_oidc_issuer")
	s.NotEmpty(eksOIDCIssuer, "EKS OIDC issuer should not be empty")
}
