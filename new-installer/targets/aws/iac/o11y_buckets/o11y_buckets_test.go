// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_observability_buckets_test

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

type ObservabilityBucketsTestSuite struct {
	suite.Suite
}

func TestObservabilityBucketsTestSuite(t *testing.T) {
	suite.Run(t, new(ObservabilityBucketsTestSuite))
}

func (s *ObservabilityBucketsTestSuite) TestApplyingModule() {
	randomText := strings.ToLower(rand.Text()[:4])
	bucketName := "o11y-test-bucket-" + randomText
	terratest_aws.CreateS3Bucket(s.T(), "us-west-2", bucketName)
	defer func() {
		terratest_aws.EmptyS3Bucket(s.T(), "us-west-2", bucketName)
		terratest_aws.DeleteS3Bucket(s.T(), "us-west-2", bucketName)
	}()
	clusterName := "o11y-test"
	variables := steps_aws.ObservabilityBucketsVariables{
		Region:      utils.DefaultTestRegion,
		CustomerTag: utils.DefaultTestCustomerTag,
		S3Prefix:    randomText,
		OIDCIssuer:  "https://oidc.eks.us-west-2.amazonaws.com/id/test-oidc-id",
		ClusterName: clusterName,
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
			"region": "us-west-2",
			"bucket": bucketName,
			"key":    "o11y_buckets.tfstate",
		},
		Reconfigure: true,
		Upgrade:     true,
	})
	defer terraform.Destroy(s.T(), terraformOptions)

	terraform.InitAndApply(s.T(), terraformOptions)

	// Verify that the S3 buckets for orch observability are created
	terratest_aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-"+randomText+"-orch-loki-admin")
	terratest_aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-"+randomText+"-orch-loki-chunks")
	terratest_aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-"+randomText+"-orch-loki-ruler")
	terratest_aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-"+randomText+"-orch-mimir-ruler")
	terratest_aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-"+randomText+"-orch-mimir-tsdb")
	// Verify that the S3 buckets for edge node observability are created
	terratest_aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-"+randomText+"-fm-loki-admin")
	terratest_aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-"+randomText+"-fm-loki-chunks")
	terratest_aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-"+randomText+"-fm-loki-ruler")
	terratest_aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-"+randomText+"-fm-mimir-ruler")
	terratest_aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-"+randomText+"-fm-mimir-tsdb")
	terratest_aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-"+randomText+"-tempo-traces")
}
