// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_kms_test

import (
	"crypto/rand"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	aws_sdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/gruntwork-io/terratest/modules/aws"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/suite"
)

func policyString(t *testing.T, clusterName string) string {
	t.Helper()
	parsedAccId := strings.ReplaceAll(policy, "${local.account_id}", aws.GetAccountId(t))
	return strings.ReplaceAll(parsedAccId, "${aws_iam_user.vault.name}", "vault-"+clusterName)
}

const policy string = `{
    "Id": "vault",
    "Statement": [
        {
            "Sid": "Enable IAM User Permissions",
            "Effect": "Allow",
            "Principal": {
                "AWS": "arn:aws:iam::${local.account_id}:root"
            },
            "Action": "kms:*",
            "Resource": "*"
        },
        {
            "Sid": "Allow use of the key",
            "Action": [
                "kms:Encrypt",
                "kms:Decrypt",
                "kms:ReEncrypt*",
                "kms:GenerateDataKey*",
                "kms:DescribeKey"
            ],
            "Effect": "Allow",
            "Principal": {
                "AWS": "arn:aws:iam::${local.account_id}:user/${aws_iam_user.vault.name}"
            },
            "Resource": "*"
        }
    ],
    "Version": "2012-10-17"
}`

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
	clusterName := "kms-test-" + randomPostfix
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
	time.Sleep(time.Second * 300)

	// Verify that the IAM User for Vault was created
	iamClient, err := aws.NewIamClientE(s.T(), "us-west-2")
	s.Require().NoError(err, "Failed to create IAM client")
	iamUserOutput, err := iamClient.GetUser(s.T().Context(), &iam.GetUserInput{
		UserName: aws_sdk.String("vault-" + clusterName),
	})
	s.Require().NoError(err, "IAM User for Vault should be created")
	s.NotNil(iamUserOutput, "IAM User for Vault should be created")

	// Verify that the KMS Key was created
	kmsClient, err := aws.NewKmsClientE(s.T(), "us-west-2")
	s.Require().NoError(err, "Failed to create KMS client")
	kmsKeyOutput, err := kmsClient.DescribeKey(s.T().Context(), &kms.DescribeKeyInput{
		KeyId: aws_sdk.String("alias/vault-kms-unseal-" + clusterName),
	})
	s.Require().NoError(err, "KMS Key should be created")
	s.NotNil(kmsKeyOutput, "KMS Key should be created")
	keyPolicies, err := kmsClient.ListKeyPolicies(s.T().Context(), &kms.ListKeyPoliciesInput{
		KeyId: aws_sdk.String(*kmsKeyOutput.KeyMetadata.KeyId),
	})
	s.Require().NoError(err, "Failed to list key policies for KMS Key")
	s.Require().Len(keyPolicies.PolicyNames, 1, "There should be one key policy for the KMS Key")
	keyPolicy, err := kmsClient.GetKeyPolicy(s.T().Context(), &kms.GetKeyPolicyInput{
		KeyId:      aws_sdk.String(*kmsKeyOutput.KeyMetadata.KeyId),
		PolicyName: aws_sdk.String("default"),
	})
	s.Require().NoError(err, "Failed to get key policy for KMS Key")
	s.JSONEq(policyString(s.T(), clusterName),
		*keyPolicy.Policy,
		"The KMS Key policy should match the expected policy")
}
