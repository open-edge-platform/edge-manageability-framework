// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package steps_aws_test

import (
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	terratest_aws "github.com/gruntwork-io/terratest/modules/aws"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	steps_aws "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/aws"

	"github.com/stretchr/testify/suite"
)

type StateBucketTest struct {
	suite.Suite
	config internal.OrchInstallerConfig

	step              *steps_aws.CreateAWSStateBucket
	randomText        string
	terraformExecPath string
}

func TestCreateAWSStateBucket(t *testing.T) {
	suite.Run(t, new(StateBucketTest))
}

func (s *StateBucketTest) SetupTest() {
	s.randomText = strings.ToLower(rand.Text()[0:8])
	s.config = internal.OrchInstallerConfig{}
	s.config.Aws.Region = "us-west-2"
	s.config.Global.OrchName = "test"
	s.config.Generated.DeploymentId = s.randomText
	var err error
	s.terraformExecPath, err = steps.InstallTerraformAndGetExecPath()
	if err != nil {
		s.NoError(err)
		return
	}
	rootPath, err := filepath.Abs("../../../../")
	if err != nil {
		s.NoError(err)
		return
	}
	s.step = &steps_aws.CreateAWSStateBucket{
		RootPath:           rootPath,
		KeepGeneratedFiles: false,
		TerraformExecPath:  s.terraformExecPath,
	}
}

func (s *StateBucketTest) TearDownTest() {
	s.config.Generated.Action = "uninstall"
	ctx := context.Background()
	newRS, err := s.step.ConfigStep(ctx, s.config)
	if err != nil {
		s.NoError(err)
		return
	}
	err = internal.UpdateRuntimeState(&s.config.Generated, newRS)
	if err != nil {
		s.NoError(err)
		return
	}
	newRS, err = s.step.PreStep(ctx, s.config)
	if err != nil {
		s.NoError(err)
		return
	}
	err = internal.UpdateRuntimeState(&s.config.Generated, newRS)
	if err != nil {
		s.NoError(err)
		return
	}
	newRS, err = s.step.RunStep(ctx, s.config)
	if err != nil {
		s.NoError(err)
		return
	}
	if !s.NotEmpty(newRS.StateBucketState) {
		return
	}
	err = internal.UpdateRuntimeState(&s.config.Generated, newRS)
	if err != nil {
		s.NoError(err)
		return
	}
	newRS, err = s.step.PostStep(ctx, s.config, err)
	if err != nil {
		s.NoError(err)
		return
	}
	err = internal.UpdateRuntimeState(&s.config.Generated, newRS)
	if err != nil {
		s.NoError(err)
		return
	}
	if _, err := os.Stat(s.terraformExecPath); err == nil {
		err = os.Remove(s.terraformExecPath)
		if err != nil {
			s.NoError(err)
			return
		}
	}
}

func (s *StateBucketTest) TestInstall() {
	s.config.Generated.Action = "install"
	ctx := context.Background()
	newRS, err := s.step.ConfigStep(ctx, s.config)
	if err != nil {
		s.NoError(err)
		return
	}
	err = internal.UpdateRuntimeState(&s.config.Generated, newRS)
	if err != nil {
		s.NoError(err)
		return
	}
	newRS, err = s.step.PreStep(ctx, s.config)
	if err != nil {
		s.NoError(err)
		return
	}
	err = internal.UpdateRuntimeState(&s.config.Generated, newRS)
	if err != nil {
		s.NoError(err)
		return
	}
	newRS, err = s.step.RunStep(ctx, s.config)
	if err != nil {
		s.NoError(err)
		return
	}
	if !s.NotEmpty(newRS.StateBucketState) {
		return
	}
	err = internal.UpdateRuntimeState(&s.config.Generated, newRS)
	if err != nil {
		s.NoError(err)
		return
	}
	newRS, err = s.step.PostStep(ctx, s.config, err)
	if err != nil {
		s.NoError(err)
		return
	}
	err = internal.UpdateRuntimeState(&s.config.Generated, newRS)
	if err != nil {
		s.NoError(err)
		return
	}

	expectBucketName := s.config.Global.OrchName + "-" + s.config.Generated.DeploymentId
	terratest_aws.AssertS3BucketExists(s.T(), s.config.Aws.Region, expectBucketName)
}
