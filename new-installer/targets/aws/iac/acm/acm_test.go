// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_aic_acm_test

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/acm"
	terratest_aws "github.com/gruntwork-io/terratest/modules/aws"
	"github.com/gruntwork-io/terratest/modules/terraform"
	steps_aws "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/aws"
	"github.com/open-edge-platform/edge-manageability-framework/installer/targets/aws/iac/utils"
	"github.com/stretchr/testify/suite"
)

type ACMTestSuite struct {
	suite.Suite
	name string
}

func TestACMSuite(t *testing.T) {
	suite.Run(t, new(ACMTestSuite))
}

func (s *ACMTestSuite) SetupTest() {
	s.name = "efs-unit-test-" + strings.ToLower(rand.Text()[0:8])
	terratest_aws.CreateS3Bucket(s.T(), utils.DefaultTestRegion, s.name)
}

func (s *ACMTestSuite) TearDownTest() {
	terratest_aws.EmptyS3Bucket(s.T(), utils.DefaultTestRegion, s.name)
	terratest_aws.DeleteS3Bucket(s.T(), utils.DefaultTestRegion, s.name)
}

func (s *ACMTestSuite) TestApplyModule() {
	testDomain := strings.ToLower(rand.Text()[0:8]) + ".example.com"
	tlsCertPEM, tlsCAPEM, keyPEM, err := steps_aws.GenerateSelfSignedTLSCert(testDomain)
	if err != nil {
		s.NoError(err, "Failed to generate self-signed TLS certificate")
		return
	}

	variables := steps_aws.ACMVariables{
		CertificateBody:  tlsCertPEM,
		CertificateChain: tlsCAPEM,
		PrivateKey:       keyPEM,
		ClusterName:      "test-cluster",
		CustomerTag:      utils.DefaultTestCustomerTag,
		Region:           utils.DefaultTestRegion,
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
			"key":    "acm.tfstate",
		},
		Reconfigure: true,
		Upgrade:     true,
	})

	defer terraform.Destroy(s.T(), terraformOptions)
	terraform.InitAndApply(s.T(), terraformOptions)
	output := terraform.Output(s.T(), terraformOptions, "certArn")
	s.NotEmpty(output, "Certificate ARN should not be empty")
	acmClient := terratest_aws.NewAcmClient(s.T(), utils.DefaultTestRegion)

	result, err := acmClient.GetCertificate(context.Background(), &acm.GetCertificateInput{
		CertificateArn: &output,
	})
	if err != nil {
		s.NoError(err, "Failed to get ACM certificate")
		return
	}
	s.Equal(tlsCertPEM, *result.Certificate, "Certificate body should match the generated TLS certificate")
}
