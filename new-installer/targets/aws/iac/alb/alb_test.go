// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_alb_test

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/aws/aws-sdk-go/aws"
	terratest_aws "github.com/gruntwork-io/terratest/modules/aws"
	"github.com/gruntwork-io/terratest/modules/terraform"
	steps_aws "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/aws"
	"github.com/open-edge-platform/edge-manageability-framework/installer/targets/aws/iac/utils"
	"github.com/stretchr/testify/suite"
)

type ALBTestSuite struct {
	suite.Suite
	name             string
	vpcID            string
	publicSubnetIDs  []string
	privateSubnetIDs []string
	certARN          string
}

func TestALBTestSuite(t *testing.T) {
	suite.Run(t, new(ALBTestSuite))
}

func (s *ALBTestSuite) SetupTest() {
	// Bucket for ALB state
	s.name = "alb-unit-test-" + strings.ToLower(rand.Text()[0:8])
	terratest_aws.CreateS3Bucket(s.T(), utils.DefaultTestRegion, s.name)

	// VPC and certificate for ALB
	var err error
	s.vpcID, s.publicSubnetIDs, s.privateSubnetIDs, _, _, err = utils.CreateVPC(s.T(), s.name)
	if err != nil {
		s.NoError(err, "Failed to create VPC and subnet")
		return
	}
	tlsCert, tlsCA, tlsKey, err := steps_aws.GenerateSelfSignedTLSCert("alb-unit-test.example.com")
	if err != nil {
		s.NoError(err, "Failed to generate self-signed TLS certificate")
		return
	}
	acmClient := terratest_aws.NewAcmClient(s.T(), utils.DefaultTestRegion)
	output, err := acmClient.ImportCertificate(context.Background(), &acm.ImportCertificateInput{
		Certificate:      []byte(tlsCert),
		PrivateKey:       []byte(tlsKey),
		CertificateChain: []byte(tlsCA),
		Tags:             []types.Tag{{Key: aws.String("customer"), Value: aws.String(utils.DefaultTestCustomerTag)}},
	})
	if err != nil {
		s.NoError(err, "Failed to import certificate")
		return
	}
	s.certARN = *output.CertificateArn
}

func (s *ALBTestSuite) TearDownTest() {
	err := utils.DeleteVPC(s.T(), s.name)
	if err != nil {
		s.NoError(err, "Failed to delete VPC %s", s.name)
	}
	acmClient := terratest_aws.NewAcmClient(s.T(), utils.DefaultTestRegion)
	_, err = acmClient.DeleteCertificate(context.Background(), &acm.DeleteCertificateInput{
		CertificateArn: aws.String(s.certARN),
	})
	if err != nil {
		s.NoError(err, "Failed to delete ACM certificate %s", s.certARN)
	}
	terratest_aws.EmptyS3Bucket(s.T(), utils.DefaultTestRegion, s.name)
	terratest_aws.DeleteS3Bucket(s.T(), utils.DefaultTestRegion, s.name)
}

func (s *ALBTestSuite) TestApplyingModule() {
	variables := steps_aws.ALBVariables{
		Internal:                 false,
		VPCID:                    s.vpcID,
		ClusterName:              s.name,
		PublicSubnetIDs:          s.publicSubnetIDs,
		IPAllowList:              []string{"10.0.0.0/8"},
		EnableDeletionProtection: false,
		TLSCertARN:               s.certARN,
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
			"key":    "alb.tfstate",
		},
		Reconfigure: true,
		Upgrade:     true,
	})

	defer terraform.Destroy(s.T(), terraformOptions)
	terraform.InitAndApply(s.T(), terraformOptions)
}
