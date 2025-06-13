// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_ptcp_test

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

type PTCPTestSuite struct {
	suite.Suite
	name             string
	vpcID            string
	publicSubnetIDs  []string
	privateSubnetIDs []string
}

func TestPTCPSuite(t *testing.T) {
	suite.Run(t, new(PTCPTestSuite))
}

func (s *PTCPTestSuite) SetupTest() {
	// Bucket for PTCP state
	s.name = "ptcp-unit-test-" + strings.ToLower(rand.Text()[0:8])
	terratest_aws.CreateS3Bucket(s.T(), utils.DefaultTestRegion, s.name)

	// VPC and subnets for PTCP
	var err error
	s.vpcID, s.publicSubnetIDs, s.privateSubnetIDs, _, _, err = utils.CreateVPCWithEndpoints(s.T(), s.name, []string{})
	if err != nil {
		s.NoError(err, "Failed to create VPC and subnet")
		return
	}
}

func (s *PTCPTestSuite) TearDownTest() {
	err := utils.DeleteVPC(s.T(), s.name)
	if err != nil {
		s.NoError(err, "Failed to delete VPC")
		return
	}
	terratest_aws.EmptyS3Bucket(s.T(), utils.DefaultTestRegion, s.name)
	terratest_aws.DeleteS3Bucket(s.T(), utils.DefaultTestRegion, s.name)
}

func (s *PTCPTestSuite) TestApplyingModule() {
	testDomain := strings.ToLower(rand.Text()[0:8]) + "." + utils.DefaultTestRoute53Zone
	tlsCertPEM, _, keyPEM, err := steps_aws.GenerateSelfSignedTLSCert(testDomain)
	if err != nil {
		s.NoError(err, "Failed to generate self-signed TLS certificate")
		return
	}
	variables := steps_aws.PTCPVariables{
		ClusterName:     s.name,
		Region:          utils.DefaultTestRegion,
		VPCID:           s.vpcID,
		SubnetIDs:       s.privateSubnetIDs,
		CustomerTag:     utils.DefaultTestCustomerTag,
		Route53ZoneName: utils.DefaultTestRoute53Zone,
		IPAllowList:     []string{steps_aws.DefaultNetworkCIDR},
		TLSCertKey:      keyPEM,
		TLSCertBody:     tlsCertPEM,
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
			"key":    "ptcp.tfstate",
		},
		Reconfigure: true,
		Upgrade:     true,
	})

	defer terraform.Destroy(s.T(), terraformOptions)
	terraform.InitAndApply(s.T(), terraformOptions)
}
