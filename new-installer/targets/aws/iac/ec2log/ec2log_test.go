// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_test

import (
	"context"
	"encoding/json"
	"math/rand"
	"os"
	"strings"
	"testing"

	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	terratest_aws "github.com/gruntwork-io/terratest/modules/aws"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/open-edge-platform/edge-manageability-framework/installer/targets/aws/iac/utils"
	"github.com/stretchr/testify/suite"
)

type EC2LogTestSuite struct {
	suite.Suite
	name     string
	roleName string
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandomString(n int) string {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteByte(letterBytes[rand.Intn(len(letterBytes))])
	}
	return sb.String()
}

func TestEC2LogTestSuite(t *testing.T) {
	suite.Run(t, new(EC2LogTestSuite))
}

func (s *EC2LogTestSuite) SetupTest() {
	s.name = "ec2log-unit-test-" + strings.ToLower(RandomString(8))
	terratest_aws.CreateS3Bucket(s.T(), utils.DefaultTestRegion, s.name)

	// Create an AWS IAM role for testing
	iamClient := terratest_aws.NewIamClient(s.T(), utils.DefaultTestRegion)
	s.roleName = s.name + "-role"
	trustPolicy := `{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Principal": {
					"Service": "ec2.amazonaws.com"
				},
				"Action": "sts:AssumeRole"
			}
		]
	}`

	// Create the IAM role
	input := &iam.CreateRoleInput{
		RoleName:                 &s.roleName,
		AssumeRolePolicyDocument: &trustPolicy,
	}
	result, err := iamClient.CreateRole(context.Background(), input)
	if err != nil {
		fmt.Println("Failed to create role:", err)
	} else {
		fmt.Println("Successfully created role:", *result.Role.RoleName)
	}

}

func (s *EC2LogTestSuite) TearDownTest() {
	terratest_aws.EmptyS3Bucket(s.T(), utils.DefaultTestRegion, s.name)
	terratest_aws.DeleteS3Bucket(s.T(), utils.DefaultTestRegion, s.name)
	iamClient := terratest_aws.NewIamClient(s.T(), utils.DefaultTestRegion)
	_, err := iamClient.DeleteRole(context.Background(), &iam.DeleteRoleInput{
		RoleName: &s.roleName,
	})
	if err != nil {
		s.T().Logf("Failed to delete IAM role %s: %v", s.roleName, err)
	} else {
		s.T().Logf("Successfully deleted IAM role %s", s.roleName)
	}
}

func (s *EC2LogTestSuite) TestApplyEC2LogModule() {
	vars := map[string]interface{}{
		"cluster_name":   s.name,
		"s3_prefix":      "",
		"nodegroup_role": s.roleName,
		"region":         utils.DefaultTestRegion,
	}

	jsonData, err := json.Marshal(vars)
	s.NoError(err, "Failed to marshal variables")

	tempFile, err := os.CreateTemp("", "ec2log-vars-*.tfvars.json")
	s.NoError(err, "Failed to create temp file")
	defer os.Remove(tempFile.Name())
	_, err = tempFile.Write(jsonData)
	s.NoError(err, "Failed to write to temp file")

	terraformOptions := terraform.WithDefaultRetryableErrors(s.T(), &terraform.Options{
		TerraformDir: ".",
		VarFiles:     []string{tempFile.Name()},
		BackendConfig: map[string]interface{}{
			"region": utils.DefaultTestRegion,
			"bucket": s.name,
			"key":    "ec2log.tfstate",
		},
		Reconfigure: true,
		Upgrade:     true,
	})

	defer terraform.Destroy(s.T(), terraformOptions)
	terraform.InitAndApply(s.T(), terraformOptions)

	// Check S3 bucket exists
	bucket := terraform.Output(s.T(), terraformOptions, "logs_bucket_name")
	s.NotEmpty(bucket, "S3 bucket name should not be empty")
	terratest_aws.AssertS3BucketExists(s.T(), utils.DefaultTestRegion, bucket)

	// Check SSM document exists
	ssmDoc := terraform.Output(s.T(), terraformOptions, "ssm_document_name")
	s.NotEmpty(ssmDoc, "SSM document name should not be empty")
	ssmInput := &ssm.GetDocumentInput{
		Name: aws.String(ssmDoc),
	}
	ssmClient := terratest_aws.NewSsmClient(s.T(), utils.DefaultTestRegion)
	_, err = ssmClient.GetDocument(context.Background(), ssmInput)
	s.NoError(err, "SSM document should exist")

	// Check Lambda function exists
	lambdaName := terraform.Output(s.T(), terraformOptions, "lambda_function_name")
	s.NotEmpty(lambdaName, "Lambda function name should not be empty")
	lambdaClient := terratest_aws.NewLambdaClient(s.T(), utils.DefaultTestRegion)
	lambdaInput := &lambda.GetFunctionInput{
		FunctionName: aws.String(lambdaName),
	}
	_, err = lambdaClient.GetFunction(context.Background(), lambdaInput)
	s.NoError(err, "Lambda function should exist")
}
