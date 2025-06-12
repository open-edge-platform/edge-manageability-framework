// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package steps_aws_test

import (
	"crypto/rand"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	steps_aws "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/aws"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type StateBucketTest struct {
	suite.Suite
	config       config.OrchInstallerConfig
	runtimeState config.OrchInstallerRuntimeState

	step       *steps_aws.StateBucketStep
	randomText string
	tfUtility  *MockTerraformUtility
	awsUtility *MockAWSUtility
}

func TestCreateAWSStateBucket(t *testing.T) {
	suite.Run(t, new(StateBucketTest))
}

func (s *StateBucketTest) SetupTest() {
	rootPath, err := filepath.Abs("../../../../")
	if err != nil {
		s.NoError(err)
		return
	}
	s.randomText = strings.ToLower(rand.Text()[0:8])
	s.config = config.OrchInstallerConfig{}
	s.config.AWS.Region = "us-west-2"
	s.config.Global.OrchName = "test"
	s.runtimeState.DeploymentID = s.randomText
	s.tfUtility = &MockTerraformUtility{}
	s.awsUtility = &MockAWSUtility{}

	s.step = &steps_aws.StateBucketStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: false,
		TerraformUtility:   s.tfUtility,
		AWSUtility:         s.awsUtility,
	}
}

func (s *StateBucketTest) TestInstallAndUninstall() {
	s.runtimeState.Action = "install"
	s.expectTFUtiliyCall("install")
	_, err := steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}

	s.runtimeState.Action = "uninstall"
	s.expectTFUtiliyCall("uninstall")
	s.expectAWSUtiliyCall("uninstall")
	_, err = steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
	}
}

func (s *StateBucketTest) expectTFUtiliyCall(action string) {
	input := steps.TerraformUtilityInput{
		Action: action,
		Variables: steps_aws.StateBucketVariables{
			Region:   s.config.AWS.Region,
			OrchName: s.config.Global.OrchName,
			Bucket:   s.config.Global.OrchName + "-" + s.runtimeState.DeploymentID,
		},
		ModulePath:         filepath.Join(s.step.RootPath, steps_aws.StateBucketModulePath),
		LogFile:            filepath.Join(s.step.RootPath, ".logs", "aws_state_bucket.log"),
		KeepGeneratedFiles: s.step.KeepGeneratedFiles,
		TerraformState:     "",
	}
	if action == "install" {
		s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
			TerraformState: "some state",
		}, nil).Once()
	} else {
		s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
			TerraformState: "",
		}, nil).Once()
	}
}

func (s *StateBucketTest) expectAWSUtiliyCall(action string) {
	if action == "uninstall" {
		s.awsUtility.On("EmptyS3Bucket", s.config.AWS.Region, s.config.Global.OrchName+"-"+s.runtimeState.DeploymentID).
			Return(nil).Once()
	}
}
