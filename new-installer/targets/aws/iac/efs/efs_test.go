// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_efs_test

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

type EFSTestSuite struct {
	suite.Suite
	name             string
	vpcID            string
	publicSubnetIDs  []string
	privateSubnetIDs []string
	randomPostfix    string
}

func TestEFSTestSuite(t *testing.T) {
	suite.Run(t, new(EFSTestSuite))
}

func (s *EFSTestSuite) SetupTest() {
	randomPostfix := strings.ToLower(rand.Text()[:8])
	// Bucket for EFS state
	s.name = "efs-unit-test-" + randomPostfix
	terratest_aws.CreateS3Bucket(s.T(), utils.DefaultTestRegion, s.name)

	// VPC and subnets for EFS
	var err error
	s.vpcID, s.publicSubnetIDs, s.privateSubnetIDs, _, _, err = utils.CreateVPC(s.T(), s.name)
	if err != nil {
		s.NoError(err, "Failed to create VPC and subnet")
		return
	}
}

func (s *EFSTestSuite) TearDownTest() {
	err := utils.DeleteVPC(s.T(), s.name)
	if err != nil {
		s.NoError(err, "Failed to delete VPC")
		return
	}

	terratest_aws.EmptyS3Bucket(s.T(), utils.DefaultTestRegion, s.name)
	terratest_aws.DeleteS3Bucket(s.T(), utils.DefaultTestRegion, s.name)
}

func (s *EFSTestSuite) TestApplyingModule() {
	efsVars := steps_aws.EFSVariables{
		ClusterName:      "test-efs-cluster" + s.randomPostfix,
		Region:           utils.DefaultTestRegion,
		CustomerTag:      utils.DefaultTestCustomerTag,
		PrivateSubnetIDs: s.privateSubnetIDs,
		VPCID:            s.vpcID,
		EKSOIDCIssuer:    "oidc.eks.us-west-2.amazonaws.com/id/mock-issuer",
	}

	jsonData, err := json.Marshal(efsVars)
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
		TerraformDir: "../efs",
		VarFiles:     []string{tempFile.Name()},
		BackendConfig: map[string]interface{}{
			"region": utils.DefaultTestRegion,
			"bucket": s.name,
			"key":    "efs.tfstate",
		},
		Reconfigure: true,
		Upgrade:     true,
	})

	defer terraform.Destroy(s.T(), terraformOptions)
	terraform.InitAndApply(s.T(), terraformOptions)

	efsID := terraform.Output(s.T(), terraformOptions, "efs_id")
	s.NotEmpty(efsID, "Expected EFS ID to be created")
	s.True(strings.HasPrefix(efsID, "fs-"), "Expected EFS ID to start with 'fs-'")
	s.GreaterOrEqual(len(efsID), 14, "Expected EFS ID to be at least 14 characters long")
}
