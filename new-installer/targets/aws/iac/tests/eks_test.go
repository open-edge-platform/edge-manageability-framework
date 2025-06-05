// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_test

import (
	"crypto/rand"
	"encoding/json"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	terra_test_aws "github.com/gruntwork-io/terratest/modules/aws"
	"github.com/gruntwork-io/terratest/modules/terraform"
	steps_aws "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/aws"
	"github.com/stretchr/testify/suite"
)

type EKSTestSuite struct {
	suite.Suite
	stateBucketName string
	vpcID           string
	subnetIDs       []string
	randomPostfix   string
	jumphostID      string
}

func TestEKSTestSuite(t *testing.T) {
	suite.Run(t, new(EKSTestSuite))
}

func (s *EKSTestSuite) SetupTest() {
	// Bucket for EKS state
	s.randomPostfix = strings.ToLower(rand.Text()[:8])
	s.stateBucketName = "test-bucket-" + s.randomPostfix
	terra_test_aws.CreateS3Bucket(s.T(), DefaultRegion, s.stateBucketName)

	// VPC and subnets for EKS
	var err error
	s.vpcID, s.subnetIDs, err = CreateVPC(DefaultRegion, "eks-unit-test-"+s.randomPostfix)
	if err != nil {
		s.NoError(err, "Failed to create VPC and subnet")
		return
	}
	var ipCIDRAllowList []string = make([]string, 0)
	if cidrAllowListStr := os.Getenv("JUMPHOST_IP_CIDR_ALLOW_LIST"); cidrAllowListStr != "" {
		for _, cidr := range strings.Split(cidrAllowListStr, ",") {
			cidr = strings.TrimSpace(cidr)
			if _, _, err := net.ParseCIDR(cidr); err != nil {
				continue // Skip invalid CIDR
			}
			if cidr != "" {
				ipCIDRAllowList = append(ipCIDRAllowList, cidr)
			}
		}
	}
	var jumphostPrivateKey, jumphostIP string
	s.jumphostID, jumphostPrivateKey, jumphostIP, err = CreateJumpHost(s.vpcID, s.subnetIDs[0], DefaultRegion, ipCIDRAllowList)
	if err != nil {
		s.NoError(err, "Failed to create jump host")
		return
	}

	err = StartSshuttle(jumphostIP, jumphostPrivateKey, "10.250.0.0/16")
	if err != nil {
		s.NoError(err, "Failed to start sshuttle")
		return
	}
}

func (s *EKSTestSuite) TearDownTest() {
	err := StopSshuttle()
	if err != nil {
		s.NoError(err, "Failed to stop sshuttle")
	}

	err = DeleteJumpHost(s.jumphostID, DefaultRegion)
	if err != nil {
		s.NoError(err, "Failed to delete jump host %s", s.jumphostID)
	}
	// Note: Deleting a VPC will also delete all subnets.
	for i := 0; i < 10; i++ {
		time.Sleep(10 * time.Second)
		err := DeleteVPC(DefaultRegion, s.vpcID)
		if err == nil {
			break
		}
		if i == 4 {
			s.NoError(err, "Failed to delete VPC %s after 10 attempts", s.vpcID)
			break
		}
	}
	terra_test_aws.EmptyS3Bucket(s.T(), DefaultRegion, s.stateBucketName)
	terra_test_aws.DeleteS3Bucket(s.T(), DefaultRegion, s.stateBucketName)

}

func (s *EKSTestSuite) TestApplyingModule() {
	eksVars := steps_aws.EKSVariables{
		Name:                "test-eks-cluster" + s.randomPostfix,
		Region:              DefaultRegion,
		VPCID:               s.vpcID,
		CustomerTag:         "test-customer",
		SubnetIDs:           s.subnetIDs,
		EKSVersion:          "1.32",
		NodeInstanceType:    "t3.medium",
		DesiredSize:         1,
		MinSize:             1,
		MaxSize:             1,
		MaxPods:             58,
		VolumeSize:          20,
		VolumeType:          "gp3",
		EnableCacheRegistry: true,
		CacheRegistry:       "test-cache-registry",
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
		TerraformDir: "../eks",
		VarFiles:     []string{tempFile.Name()},
		BackendConfig: map[string]interface{}{
			"region": DefaultRegion,
			"bucket": s.stateBucketName,
			"key":    "eks.tfstate",
		},
		Reconfigure: true,
		Upgrade:     true,
	})

	defer terraform.Destroy(s.T(), terraformOptions)
	terraform.InitAndApply(s.T(), terraformOptions)

	// No outputs from EKS module, so we just check if everything we need is created
	// TODO: Check if EKS cluster is created and has the expected properties
}
