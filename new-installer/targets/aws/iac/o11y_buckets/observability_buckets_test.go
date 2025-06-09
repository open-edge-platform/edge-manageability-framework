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
	"github.com/stretchr/testify/suite"
)

type ObservabilityBucketsTestSuite struct {
	suite.Suite
}

type ObservabilityBucketsVariables struct {
	Region        string `json:"region" yaml:"region"`
	CustomerTag   string `json:"customer_tag" yaml:"customer_tag"`
	S3Prefix      string `json:"s3_prefix" yaml:"s3_prefix"`
	OIDCIssuer    string `json:"oidc_issuer" yaml:"oidc_issuer"`
	ClusterName   string `json:"cluster_name" yaml:"cluster_name"`
	CreateTracing bool   `json:"create_tracing" yaml:"create_tracing"`
}

func TestObservabilityBucketsTestSuite(t *testing.T) {
	suite.Run(t, new(ObservabilityBucketsTestSuite))
}

func (s *ObservabilityBucketsTestSuite) TestApplyingModule() {
	randomPostfix := strings.ToLower(rand.Text()[:4])
	bucketName := "obs3-test-bucket-" + randomPostfix
	terratest_aws.CreateS3Bucket(s.T(), "us-west-2", bucketName)
	defer func() {
		terratest_aws.EmptyS3Bucket(s.T(), "us-west-2", bucketName)
		terratest_aws.DeleteS3Bucket(s.T(), "us-west-2", bucketName)
	}()
	clusterName := "obs3-test-" + randomPostfix
	variables := ObservabilityBucketsVariables{
		Region:        "us-west-2",
		CustomerTag:   "test-customer",
		S3Prefix:      "pre",
		OIDCIssuer:    "https://oidc.eks.us-west-2.amazonaws.com/id/test-oidc-id",
		ClusterName:   clusterName,
		CreateTracing: true,
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
		TerraformDir: "../o11y_buckets",
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
	terratest_aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-pre-"+"orch-loki-admin")
	terratest_aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-pre-"+"orch-loki-chunks")
	terratest_aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-pre-"+"orch-loki-ruler")
	terratest_aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-pre-"+"orch-mimir-ruler")
	terratest_aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-pre-"+"orch-mimir-tsdb")
	// Verify that the S3 buckets for edge node observability are created
	terratest_aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-pre-"+"fm-loki-admin")
	terratest_aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-pre-"+"fm-loki-chunks")
	terratest_aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-pre-"+"fm-loki-ruler")
	terratest_aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-pre-"+"fm-mimir-ruler")
	terratest_aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-pre-"+"fm-mimir-tsdb")
	// Verify that the S3 buckets for tracing are created if CreateTracing is true
	terratest_aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-pre-"+"tempo-traces")
}
