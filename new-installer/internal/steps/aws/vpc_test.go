// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package steps_aws_test

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	terratest_aws "github.com/gruntwork-io/terratest/modules/aws"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	steps_aws "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/aws"

	"github.com/stretchr/testify/suite"
)

type VPCStepTest struct {
	suite.Suite
	config            config.OrchInstallerConfig
	step              *steps_aws.AWSVPCStep
	randomText        string
	logDir            string
	terraformExecPath string
}

func TestVPCStep(t *testing.T) {
	suite.Run(t, new(VPCStepTest))
}

func (s *VPCStepTest) SetupTest() {
	rootPath, err := filepath.Abs("../../../../")
	if err != nil {
		s.NoError(err)
		return
	}
	s.randomText = strings.ToLower(rand.Text()[0:8])
	s.logDir = filepath.Join(rootPath, ".logs")
	internal.InitLogger("debug", s.logDir)
	s.config.AWS.Region = "us-west-2"
	s.config.Global.OrchName = "test"
	s.config.Generated.DeploymentID = s.randomText
	s.config.AWS.JumpHostWhitelist = []string{"10.250.0.0/16"}
	s.config.Generated.LogDir = filepath.Join(rootPath, ".logs")

	// Create a temporary S3 bucket to store the terraform state
	bucketName := fmt.Sprintf("%s-%s", s.config.Global.OrchName, s.randomText)
	err = createOrDeleteS3Bucket(bucketName, "create")
	if err != nil {
		s.NoError(err)
		return
	}
	if _, err := os.Stat(s.logDir); os.IsNotExist(err) {
		err := os.MkdirAll(s.logDir, os.ModePerm)
		if err != nil {
			s.NoError(err)
			return
		}
	}
	s.terraformExecPath, err = steps.InstallTerraformAndGetExecPath()
	if err != nil {
		s.NoError(err)
		return
	}

	s.step = &steps_aws.AWSVPCStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: true,
		TerraformExecPath:  s.terraformExecPath,
	}
}

func (s *VPCStepTest) TearDownTest() {
	// We will always uninstall VPC module
	s.config.Generated.Action = "uninstall"
	_, err := steps.GoThroughStepFunctions(s.step, &s.config)
	if err != nil {
		s.NoError(err)
	}

	bucketName := fmt.Sprintf("test-%s-%s", s.randomText, s.randomText)
	s3Err := createOrDeleteS3Bucket(bucketName, "delete")
	if s3Err != nil {
		s.NoError(err)
	}
	if _, err := os.Stat(s.terraformExecPath); err == nil {
		err = os.Remove(s.terraformExecPath)
		if err != nil {
			s.NoError(err)
		}
	}
}

func (s *VPCStepTest) TestInstallVPC() {
	s.config.Generated.Action = "install"
	rs, err := steps.GoThroughStepFunctions(s.step, &s.config)
	if err != nil {
		s.NoError(err)
		return
	}

	vpc := terratest_aws.GetVpcById(s.T(), rs.VPCID, s.config.AWS.Region)
	if vpc == nil {
		s.NotNil(vpc)
		return
	}
	s.Equal(vpc.Name, s.config.Global.OrchName)
	s.Equal(vpc.CidrBlock, steps_aws.DefaultNetworkCIDR)
	s.Equal(vpc.Tags["Name"], s.config.Global.OrchName)
}

func createOrDeleteS3Bucket(bucketName string, action string) error {
	session, err := session.NewSession(&aws.Config{
		Region: aws.String("us-west-2"),
	})
	if err != nil {
		return err
	}
	s3Client := s3.New(session)
	s3Input := &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	}
	if action == "create" {
		_, err = s3Client.CreateBucket(s3Input)
		if err != nil {
			if aerr, ok := err.(s3.RequestFailure); ok && aerr.Code() == s3.ErrCodeBucketAlreadyOwnedByYou {
				// no-op, bucket already exists
			} else {
				return err
			}
		}
	} else {
		_, err = s3Client.DeleteBucket(&s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
		if aerr, ok := err.(s3.RequestFailure); ok && aerr.Code() == s3.ErrCodeNoSuchBucket {
			// no-op, bucket already exists
		} else {
			return err
		}
	}
	return err
}
