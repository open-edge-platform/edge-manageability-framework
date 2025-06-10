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

type SshuttletestSuite struct {
	suite.Suite
	s                *steps_common.SshuttleStep
	shellUtilityMock *ShellUtilityMock
}

func TestSshuttleSuite(t *testing.T) {
	suite.Run(t, new(SshuttletestSuite))
}

func (suite *SshuttletestSuite) SetupTest() {
	suite.shellUtilityMock = &ShellUtilityMock{}
	suite.s = &steps_common.SshuttleStep{
		ShellUtility: suite.shellUtilityMock,
	}
}

func (suite *SshuttletestSuite) TestRunSshuttle() {
	cfg := &config.OrchInstallerConfig{}
	runtimeState := config.OrchInstallerRuntimeState{}
	runtimeState.AWS.JumpHostIP = "10.0.0.1"
	runtimeState.AWS.JumpHostSSHKeyPrivateKey = "foo"

	expectCallsForCmdExists(suite.shellUtilityMock, "sudo", true)
	expectCallsForCmdExists(suite.shellUtilityMock, "sshuttle", true)
	expectCallsForCmdExists(suite.shellUtilityMock, "nc", true)

	pgrepOut := strings.Builder{}
	pgrepOut.WriteString("12345\n")

	suite.shellUtilityMock.On("Run", mock.Anything, steps.ShellUtilityInput{
		Command:         []string{"pgrep", "-x", "sshuttle"},
		Timeout:         10,
		SkipError:       true,
		RunInBackground: false,
	}).Return(&steps.ShellUtilityOutput{
		Stdout: pgrepOut,
		Stderr: strings.Builder{},
		Error:  nil,
	}, nil).Once()

	suite.shellUtilityMock.On("Run", mock.Anything, steps.ShellUtilityInput{
		Command:         []string{"sudo", "kill", "12345"},
		Timeout:         10,
		SkipError:       false,
		RunInBackground: false,
	}).Return(&steps.ShellUtilityOutput{
		Stdout: strings.Builder{},
		Stderr: strings.Builder{},
		Error:  nil,
	}, nil).Once()

	suite.shellUtilityMock.On("Run", mock.Anything, mock.Anything).Return(&steps.ShellUtilityOutput{
		Stdout: strings.Builder{},
		Stderr: strings.Builder{},
		Error:  nil,
	}, nil).Once()

	_, err := steps.GoThroughStepFunctions(suite.s, cfg, runtimeState)
	if err != nil {
		suite.NoError(err, "Expected no error during step execution")
	}
}
