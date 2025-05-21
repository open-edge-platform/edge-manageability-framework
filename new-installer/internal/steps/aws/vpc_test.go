// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package steps_aws_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	steps_aws "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/aws"

	"github.com/stretchr/testify/suite"
)

type VPCStepTest struct {
	suite.Suite
	config       internal.OrchInstallerConfig
	runtimeState internal.OrchInstallerRuntimeState
	step         *steps_aws.AWSVPCStep
	randomText   string
	logDir       string
}

func TestVPCStep(t *testing.T) {
	suite.Run(t, new(VPCStepTest))
}

func (s *VPCStepTest) SetupTest() {
	s.randomText = strings.ToLower(rand.Text()[0:8])
	s.config = internal.OrchInstallerConfig{
		DeploymentName:          fmt.Sprintf("test-%s", s.randomText),
		Region:                  "us-west-2",
		NetworkCIDR:             "10.250.0.0/16",
		StateStoreBucketPostfix: s.randomText,
		JumpHostIPAllowList:     []string{"10.250.0.0/16"},
		CustomerTag:             "unit-test",
	}

	// Create a temporary S3 bucket to store the terraform state
	bucketName := fmt.Sprintf("test-%s-%s", s.randomText, s.randomText)
	err := createOrDeleteS3Bucket(bucketName, "create")
	if err != nil {
		s.NoError(err)
		return
	}

	rootPath, err := filepath.Abs("../../../../")
	if err != nil {
		s.NoError(err)
		return
	}
	s.logDir = filepath.Join(rootPath, ".logs")
	s.runtimeState = internal.OrchInstallerRuntimeState{
		Mutex:  &sync.Mutex{},
		Action: "install",
		LogDir: s.logDir,
	}
	if _, err := os.Stat(s.logDir); os.IsNotExist(err) {
		err := os.MkdirAll(s.logDir, os.ModePerm)
		if err != nil {
			s.NoError(err)
			return
		}
	}

	s.step = &steps_aws.AWSVPCStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: true,
	}
}

func (s *VPCStepTest) TearDownTest() {
	// We will always uninstall VPC module
	s.runtimeState.Action = "uninstall"
	s.goThroughStepFunctions()

	bucketName := fmt.Sprintf("test-%s-%s", s.randomText, s.randomText)
	err := createOrDeleteS3Bucket(bucketName, "delete")
	if err != nil {
		s.NoError(err)
		return
	}
}

func (s *VPCStepTest) TestInstallVPC() {
	s.goThroughStepFunctions()
}

func (s *VPCStepTest) goThroughStepFunctions() {
	ctx := context.Background()
	newRS, err := s.step.ConfigStep(ctx, s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}
	err = s.runtimeState.UpdateRuntimeState(newRS)
	if err != nil {
		s.NoError(err)
		return
	}

	newRS, err = s.step.PreStep(ctx, s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}
	err = s.runtimeState.UpdateRuntimeState(newRS)
	if err != nil {
		s.NoError(err)
		return
	}

	newRS, err = s.step.RunStep(ctx, s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}
	err = s.runtimeState.UpdateRuntimeState(newRS)
	if err != nil {
		s.NoError(err)
		return
	}

	newRS, err = s.step.PostStep(ctx, s.config, s.runtimeState, err)
	if err != nil {
		s.NoError(err)
		return
	}
	err = s.runtimeState.UpdateRuntimeState(newRS)
	if err != nil {
		s.NoError(err)
		return
	}
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
