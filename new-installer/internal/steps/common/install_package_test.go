// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps_common_test

import (
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
}

func TestInstallPackageSuite(t *testing.T) {
	suite.Run(t, new(InstallPackageTestSuite))
}

func (suite *InstallPackageTestSuite) SetupTest() {
	// Setup code for each test can go here
	suite.shellUtilityMock = &ShellUtilityMock{}
	suite.s = &steps_common.InstallPackagesStep{
		ShellUtility: suite.shellUtilityMock,
	}
}

func (suite *InstallPackageTestSuite) TestInstallPackagesOnDeb() {
	cfg := &config.OrchInstallerConfig{}
	runtimeState := config.OrchInstallerRuntimeState{}
	expectCallsForCmdExists(suite.shellUtilityMock, "sudo", true)
	expectCallsForCmdExists(suite.shellUtilityMock, "sshuttle", true)
	expectCallsForCmdExists(suite.shellUtilityMock, "apt-get", true)
	suite.shellUtilityMock.On("Run", mock.Anything, steps.ShellUtilityInput{
		Command:         []string{"sudo", "apt-get", "install", "-y", "sshuttle"},
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

func (suite *InstallPackageTestSuite) TestInstallPackagesOnMacOS() {
	cfg := &config.OrchInstallerConfig{}
	runtimeState := config.OrchInstallerRuntimeState{}
	expectCallsForCmdExists(suite.shellUtilityMock, "sudo", true)
	expectCallsForCmdExists(suite.shellUtilityMock, "sshuttle", true)
	expectCallsForCmdExists(suite.shellUtilityMock, "apt-get", false)
	expectCallsForCmdExists(suite.shellUtilityMock, "brew", true)
	suite.shellUtilityMock.On("Run", mock.Anything, steps.ShellUtilityInput{
		Command:         []string{"brew", "install", "sshuttle"},
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
