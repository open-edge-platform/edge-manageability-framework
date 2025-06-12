// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps_common_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	steps_common "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/common"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type InstallPackageTestSuite struct {
	suite.Suite
	s                *steps_common.InstallPackagesStep
	shellUtilityMock *ShellUtilityMock
	rootDir          string
}

func TestInstallPackageSuite(t *testing.T) {
	suite.Run(t, new(InstallPackageTestSuite))
}

func (suite *InstallPackageTestSuite) SetupTest() {
	suite.rootDir = suite.T().TempDir()
	suite.shellUtilityMock = &ShellUtilityMock{}
	suite.s = steps_common.CreateInstallPackagesStep(
		suite.rootDir,
		suite.shellUtilityMock,
	)
}

func (suite *InstallPackageTestSuite) TestInstallPackages() {
	cfg := &config.OrchInstallerConfig{}
	runtimeState := config.OrchInstallerRuntimeState{}
	expectCallsForCmdExists(suite.shellUtilityMock, "sudo", true)
	expectCallsForCmdExists(suite.shellUtilityMock, "python3", true)
	suite.shellUtilityMock.On("Run", mock.Anything, steps.ShellUtilityInput{
		Command:         []string{"python3", "-m", "venv", fmt.Sprintf("%s/.deploy/venv", suite.rootDir)},
		Timeout:         60,
		SkipError:       false,
		RunInBackground: false,
	}).Return(&steps.ShellUtilityOutput{
		Stdout: strings.Builder{},
		Stderr: strings.Builder{},
		Error:  nil,
	}, nil).Once()
	suite.shellUtilityMock.On("Run", mock.Anything, steps.ShellUtilityInput{
		Command:         []string{"bash", "-c", fmt.Sprintf("source %s/.deploy/venv/bin/activate && pip3 install sshuttle==1.3.1", suite.rootDir)},
		Timeout:         60,
		SkipError:       false,
		RunInBackground: false,
	}).Return(&steps.ShellUtilityOutput{
		Stdout: strings.Builder{},
		Stderr: strings.Builder{},
		Error:  nil,
	}, nil).Once()
	_, err := steps.GoThroughStepFunctions(suite.s, cfg, runtimeState)
	if err != nil {
		suite.NoError(err, "Expected no error during step execution")
	}
}
