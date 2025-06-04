// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_test

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

type KMSTestSuite struct {
	suite.Suite
}

type KMSVariables struct {
	Region      string `json:"region" yaml:"region"`
	CustomerTag string `json:"customer_tag" yaml:"customer_tag"`
	ClusterName string `json:"cluster_name" yaml:"cluster_name"`
}

func TestKMSTestSuite(t *testing.T) {
	suite.Run(t, new(KMSTestSuite))
}

func (s *KMSTestSuite) TestApplyingModule() {
	randomPostfix := strings.ToLower(rand.Text()[:4])
	bucketName := "kms-test-bucket-" + randomPostfix
	aws.CreateS3Bucket(s.T(), "us-west-2", bucketName)
	defer func() {
		aws.EmptyS3Bucket(s.T(), "us-west-2", bucketName)
		aws.DeleteS3Bucket(s.T(), "us-west-2", bucketName)
	}()
	clusterName := "obs3-test-" + randomPostfix
	variables := KMSVariables{
		Region:      "us-west-2",
		CustomerTag: "test-customer",
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
		TerraformDir: "../kms",
		VarFiles:     []string{tempFile.Name()},
		BackendConfig: map[string]interface{}{
			"region": "us-west-2",
			"bucket": bucketName,
			"key":    "kms.tfstate",
		},
		Reconfigure: true,
		Upgrade:     true,
	})
	defer terraform.Destroy(s.T(), terraformOptions)

	terraform.InitAndApply(s.T(), terraformOptions)
	// Verify that the S3 buckets for orch observability are created
	aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-pre-"+"orch-loki-admin")
	aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-pre-"+"orch-loki-chunks")
	aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-pre-"+"orch-loki-ruler")
	aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-pre-"+"orch-mimir-ruler")
	aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-pre-"+"orch-mimir-tsdb")
	// Verify that the S3 buckets for edge node observability are created
	aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-pre-"+"fm-loki-admin")
	aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-pre-"+"fm-loki-chunks")
	aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-pre-"+"fm-loki-ruler")
	aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-pre-"+"fm-mimir-ruler")
	aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-pre-"+"fm-mimir-tsdb")
	// Verify that the S3 buckets for tracing are created if CreateTracing is true
	aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-pre-"+"tempo-traces")
}
