// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_test

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
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
	randomPostfix := strings.ToLower(rand.Text()[:4])
	bucketName := "obs3-test-bucket-" + randomPostfix
	aws.CreateS3Bucket(s.T(), "us-west-2", bucketName)
	defer func() {
		aws.EmptyS3Bucket(s.T(), "us-west-2", bucketName)
		aws.DeleteS3Bucket(s.T(), "us-west-2", bucketName)
	}()
	clusterName := "obs3-test-" + randomPostfix
	variables := ObservabilityBucketsVariables{
		Region:        "us-west-2",
		CustomerTag:   "test-customer",
		S3Prefix:      "pre",
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

	clusterName, vpcId, err := CreateTestEKSCluster(clusterName, "us-west-2")
	if err != nil {
		s.T().Fatalf("Failed to create EKS cluster: %v", err)
	}

	fmt.Printf("Created EKS cluster: %s with vpc %s\n", clusterName, vpcId)

	defer func() {
		err = DeleteTestEKSCluster(clusterName, vpcId, "us-west-2")
		if err != nil {
			s.T().Fatalf("Failed to delete EKS cluster: %v", err)
		}
	}()

	err = WaitForClusterInActiveState(clusterName, "us-west-2", time.Minute*10)
	if err != nil {
		s.T().Fatalf("Failed to wait for EKS cluster to be active: %v in %s", err, (time.Minute * 10).String())
	}

	terraformOptions := terraform.WithDefaultRetryableErrors(s.T(), &terraform.Options{
		TerraformDir: "../observability_buckets",
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
