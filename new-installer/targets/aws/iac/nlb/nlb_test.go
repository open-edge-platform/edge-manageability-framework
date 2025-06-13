// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_nlb_test

import (
	"crypto/rand"
	"encoding/json"
	"os"
	"strings"
	"testing"

	terratest_aws "github.com/gruntwork-io/terratest/modules/aws"
	"github.com/gruntwork-io/terratest/modules/terraform"
	steps_aws "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/aws"
	"github.com/open-edge-platform/edge-manageability-framework/installer/targets/aws/iac/utils"
	"github.com/stretchr/testify/suite"
)

type NLBTestSuite struct {
	suite.Suite
	name             string
	vpcID            string
	publicSubnetIDs  []string
	privateSubnetIDs []string
}

func TestNLBTestSuite(t *testing.T) {
	suite.Run(t, new(NLBTestSuite))
}

func (s *NLBTestSuite) SetupTest() {
	// Bucket for NLB state
	s.name = "nlb-unit-test-" + strings.ToLower(rand.Text()[0:8])
	terratest_aws.CreateS3Bucket(s.T(), utils.DefaultTestRegion, s.name)

	// VPC NLB
	var err error
	s.vpcID, s.publicSubnetIDs, s.privateSubnetIDs, _, _, err = utils.CreateVPCWithEndpoints(s.T(), s.name, []string{})
	if err != nil {
		s.NoError(err, "Failed to create VPC and subnet")
		return
	}
}

func (s *NLBTestSuite) TearDownTest() {
	err := utils.DeleteVPCWithEndpoints(s.T(), s.name, []string{})
	if err != nil {
		s.NoError(err, "Failed to delete VPC %s", s.name)
	}
	terratest_aws.EmptyS3Bucket(s.T(), utils.DefaultTestRegion, s.name)
	terratest_aws.DeleteS3Bucket(s.T(), utils.DefaultTestRegion, s.name)
}

func (s *NLBTestSuite) TestApplyingModule() {
	variables := steps_aws.NLBVariables{
		Internal:                 false,
		VPCID:                    s.vpcID,
		ClusterName:              s.name,
		SubnetIDs:                s.publicSubnetIDs,
		IPAllowList:              []string{"10.0.0.0/8"},
		EnableDeletionProtection: false,
		Region:                   utils.DefaultTestRegion,
		CustomerTag:              utils.DefaultTestCustomerTag,
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
		TerraformDir: ".",
		VarFiles:     []string{tempFile.Name()},
		BackendConfig: map[string]interface{}{
			"region": utils.DefaultTestRegion,
			"bucket": s.name,
			"key":    "nlb.tfstate",
		},
		Reconfigure: true,
		Upgrade:     true,
	})

	defer terraform.Destroy(s.T(), terraformOptions)
	terraform.InitAndApply(s.T(), terraformOptions)
}
