// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_rds_test

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

type RDSTestSuite struct {
	suite.Suite
	name             string
	vpcID            string
	publicSubnetIDs  []string
	privateSubnetIDs []string
}

func TestRDSTestSuite(t *testing.T) {
	suite.Run(t, new(RDSTestSuite))
}

func (s *RDSTestSuite) SetupTest() {
	// Bucket for RDS state
	s.name = "rds-unit-test-" + strings.ToLower(rand.Text()[:8])
	terratest_aws.CreateS3Bucket(s.T(), utils.DefaultTestRegion, s.name)

	// VPC and subnets for RDS
	var err error
	s.vpcID, s.publicSubnetIDs, s.privateSubnetIDs, _, _, err = utils.CreateVPCWithEndpoints(s.T(), s.name, []string{})
	if err != nil {
		s.NoError(err, "Failed to create VPC and subnet")
		return
	}
}

func (s *RDSTestSuite) TearDownTest() {
	err := utils.DeleteVPCWithEndpoints(s.T(), s.name, []string{})
	if err != nil {
		s.NoError(err, "Failed to delete VPC")
		return
	}

	terratest_aws.EmptyS3Bucket(s.T(), utils.DefaultTestRegion, s.name)
	terratest_aws.DeleteS3Bucket(s.T(), utils.DefaultTestRegion, s.name)
}

func (s *RDSTestSuite) TestApplyingModule() {
	zones, err := steps_aws.CreateAWSUtility().GetAvailableZones(utils.DefaultTestRegion)
	if err != nil {
		s.T().Fatalf("Failed to get available zones: %v", err)
	}
	rdsVars := steps_aws.RDSVariables{
		ClusterName:               s.name,
		Region:                    utils.DefaultTestRegion,
		CustomerTag:               utils.DefaultTestCustomerTag,
		SubnetIDs:                 s.privateSubnetIDs,
		VPCID:                     s.vpcID,
		IPAllowList:               []string{steps_aws.DefaultNetworkCIDR},
		AvailabilityZones:         zones,
		InstanceAvailabilityZones: zones,
		PostgresVerMajor:          "",
		PostgresVerMinor:          "",
		MinACUs:                   0.5,
		MaxACUs:                   2,
		DevMode:                   true,
		Username:                  "",
		CACertIdentifier:          "",
	}

	jsonData, err := json.Marshal(rdsVars)
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
			"key":    "rds.tfstate",
		},
		Reconfigure: true,
		Upgrade:     true,
	})

	defer terraform.Destroy(s.T(), terraformOptions)
	terraform.InitAndApply(s.T(), terraformOptions)

	host := terraform.Output(s.T(), terraformOptions, "host")
	s.NotEmpty(host, "Expected RDS ID to be created")

	readerHost := terraform.Output(s.T(), terraformOptions, "host_reader")
	s.NotEmpty(readerHost, "Expected RDS Reader Host to be created")

	port := terraform.Output(s.T(), terraformOptions, "port")
	s.NotEmpty(port, "Expected RDS Port to be created")

	username := terraform.Output(s.T(), terraformOptions, "username")
	s.NotEmpty(username, "Expected RDS Username to be created")

	password := terraform.Output(s.T(), terraformOptions, "password")
	s.NotEmpty(password, "Expected RDS Password to be created")
}
