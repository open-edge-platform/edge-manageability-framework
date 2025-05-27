// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_state_bucket_test

import (
	"crypto/rand"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/aws"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/suite"
)

type StateBucketTestSuite struct {
	suite.Suite
}

func TestStateBucketTestSuite(t *testing.T) {
	suite.Run(t, new(StateBucketTestSuite))
}

func (s *StateBucketTestSuite) TestApplyingModule() {
	randomPostfix := strings.ToLower(rand.Text()[:8])
	bucketName := "test-bucket-" + randomPostfix
	terraformOptions := terraform.WithDefaultRetryableErrors(s.T(), &terraform.Options{
		TerraformDir: "../state_bucket",
		Vars: map[string]any{
			"region":    "us-west-2",
			"orch_name": "test",
			"bucket":    bucketName,
		},
	})
	defer terraform.Destroy(s.T(), terraformOptions)
	terraform.InitAndApply(s.T(), terraformOptions)
	aws.AssertS3BucketExists(s.T(), "us-west-2", bucketName)
}
