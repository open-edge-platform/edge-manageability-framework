// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_state_bucket_test

import (
	"crypto/rand"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/aws"
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
	ClusterName   string `json:"cluster_name" yaml:"cluster_name"`
	CreateTracing bool   `json:"create_tracing" yaml:"create_tracing"`
}

func TestObservabilityBucketsTestSuite(t *testing.T) {
	suite.Run(t, new(ObservabilityBucketsTestSuite))
}

func (s *ObservabilityBucketsTestSuite) TestApplyingModule() {
	randomPostfix := strings.ToLower(rand.Text()[:8])
	bucketName := "observability-buckets-state-test-bucket-" + randomPostfix
	aws.CreateS3Bucket(s.T(), "us-west-2", bucketName)
	defer func() {
		aws.EmptyS3Bucket(s.T(), "us-west-2", bucketName)
		aws.DeleteS3Bucket(s.T(), "us-west-2", bucketName)
	}()
	variables := ObservabilityBucketsVariables{
		Region:        "us-west-2",
		CustomerTag:   "test-customer",
		S3Prefix:      "test-prefix",
		ClusterName:   "test-cluster-" + randomPostfix,
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
		TerraformDir: "../s3",
		VarFiles:     []string{tempFile.Name()},
		BackendConfig: map[string]interface{}{
			"region": "us-west-2",
			"bucket": bucketName,
			"key":    "observability_buckets.tfstate",
		},
		Reconfigure: true,
		Upgrade:     true,
	})
	defer terraform.Destroy(s.T(), terraformOptions)

	terraform.InitAndApply(s.T(), terraformOptions)
	aws.AssertS3BucketExists(s.T(), "us-west-2", bucketName)
}
