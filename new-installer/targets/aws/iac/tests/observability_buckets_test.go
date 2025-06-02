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
	"time"

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
	clusterName := "observability-test-cluster-" + randomPostfix
	variables := ObservabilityBucketsVariables{
		Region:        "us-west-2",
		CustomerTag:   "test-customer",
		S3Prefix:      "test-prefix",
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

	clusterName, subnets, vpcId, err := CreateTestEKSCluster(clusterName, "us-west-2")
	if err != nil {
		s.T().Fatalf("Failed to create EKS cluster: %v", err)
	}

	defer DeleteTestEKSCluster(clusterName, subnets, vpcId, "us-west-2")
	time.Sleep(360 * time.Second) // Wait for EKS cluster to be ready
	// eksCluster, err := GetEKSCluster(clusterName, "us-west-2")
	// if err != nil {
	// 	s.T().Fatalf("Failed to get EKS cluster: %v", err)
	// }
	// s.NotNil(eksCluster, "EKS cluster should not be nil")
	// s.NotNil(eksCluster.Identity, "Identity shouldn't be nil")
	// s.NotNil(eksCluster.Identity.Oidc, "OIDC shouldn't be nil")
	// s.NotNil(eksCluster.Identity.Oidc.Issuer, "OIDC issuer shouldn't be nil")
	// s.NotNil(nil, "nil shouldn't be nil")

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
	// Verify that the S3 buckets for orch observability are created
	aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-test-prefix-"+"orch-loki-admin")
	aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-test-prefix-"+"orch-loki-chunks")
	aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-test-prefix-"+"orch-loki-ruler")
	aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-test-prefix-"+"orch-mimir-ruler")
	aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-test-prefix-"+"orch-mimir-tsdb")
	// Verify that the S3 buckets for edge node observability are created
	aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-test-prefix-"+"fm-loki-admin")
	aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-test-prefix-"+"fm-loki-chunks")
	aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-test-prefix-"+"fm-loki-ruler")
	aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-test-prefix-"+"fm-mimir-ruler")
	aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-test-prefix-"+"fm-mimir-tsdb")
	// Verify that the S3 buckets for tracing are created if CreateTracing is true
	aws.AssertS3BucketExists(s.T(), "us-west-2", clusterName+"-test-prefix-"+"tempo-traces")

}
