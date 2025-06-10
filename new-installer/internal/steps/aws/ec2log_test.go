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

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	steps_aws "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/aws"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

const (
	DefaultCustomerTag   = "default-customer-tag"
	DefaultNodeGroupRole = "default-node-group-role"
	DefaultS3Prefix      = "default-s3-prefix"
)

type EC2LogStepTest struct {
	suite.Suite
	config       config.OrchInstallerConfig
	runtimeState config.OrchInstallerRuntimeState
	step         *steps_aws.EC2LogStep
	randomText   string
	logDir       string
	tfUtility    *MockTerraformUtility
	awsUtility   *MockAWSUtility
}

func TestEC2LogStep(t *testing.T) {
	suite.Run(t, new(EC2LogStepTest))
}

func (s *EC2LogStepTest) SetupTest() {
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
	s.runtimeState.AWS.NodeGroupRole = DefaultNodeGroupRole
	s.config.AWS.CustomerTag = DefaultCustomerTag
	s.runtimeState.LogDir = filepath.Join(rootPath, ".logs")

	if _, err := os.Stat(s.logDir); os.IsNotExist(err) {
		err := os.MkdirAll(s.logDir, os.ModePerm)
		if err != nil {
			s.NoError(err)
			return
		}
	}
	s.tfUtility = &MockTerraformUtility{}
	s.awsUtility = &MockAWSUtility{}
	s.step = &steps_aws.EC2LogStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: true,
		TerraformUtility:   s.tfUtility,
		AWSUtility:         s.awsUtility,
	}
}

func (s *EC2LogStepTest) TestInstallAndUninstallEC2Log() {
	s.runtimeState.Action = "install"
	s.expectTFUtiliyCall("install")
	_, err := steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}

	s.runtimeState.Action = "uninstall"
	s.expectTFUtiliyCall("uninstall")
	_, err = steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
	}
}

func (s *EC2LogStepTest) expectTFUtiliyCall(action string) {
	input := steps.TerraformUtilityInput{
		Action:             action,
		ModulePath:         filepath.Join(s.step.RootPath, steps_aws.EC2LogModulePath),
		LogFile:            filepath.Join(s.logDir, "aws_ec2log.log"),
		KeepGeneratedFiles: s.step.KeepGeneratedFiles,
		Variables: steps_aws.EC2logVariables{
			ClusterName:   s.config.Global.OrchName,
			Region:        s.config.AWS.Region,
			NodeGroupRole: s.runtimeState.AWS.NodeGroupRole,
			S3Prefix:      s.runtimeState.DeploymentID,
			CustomerTag:   s.config.AWS.CustomerTag,
		},
		BackendConfig: steps_aws.TerraformAWSBucketBackendConfig{
			Region: s.config.AWS.Region,
			Bucket: fmt.Sprintf("%s-%s", s.config.Global.OrchName, s.runtimeState.DeploymentID),
			Key:    "ec2log.tfstate",
		},
		TerraformState: "",
	}
	s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
		TerraformState: "",
	}, nil).Once()
}
