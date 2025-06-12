// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps_common_test

import (
	"fmt"
	"regexp"
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
	rootPath         string
}

func TestSshuttleSuite(t *testing.T) {
	suite.Run(t, new(SshuttletestSuite))
}

func (suite *SshuttletestSuite) SetupTest() {
	suite.shellUtilityMock = &ShellUtilityMock{}
	suite.rootPath = suite.T().TempDir()
	suite.s = steps_common.CreateSshuttleStep(
		suite.rootPath,
		suite.shellUtilityMock,
	)
}

func (suite *SshuttletestSuite) TestRunSshuttle() {
	cfg := &config.OrchInstallerConfig{}
	runtimeState := config.OrchInstallerRuntimeState{}
	runtimeState.AWS.JumpHostIP = "10.0.0.1"
	runtimeState.AWS.JumpHostSSHKeyPrivateKey = "foo"

	expectCallsForCmdExists(suite.shellUtilityMock, "sudo", true)
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
	// Since the PID and jumphost key files are random generated, we cannot assert the exact command.
	// Instead, we can check that the last command executed was the expected one.
	lastCall := suite.shellUtilityMock.Calls[len(suite.shellUtilityMock.Calls)-1]
	suite.Len(lastCall.Arguments, 2, "Expected the last command to have two arguments")
	if secondArg, ok := lastCall.Arguments[1].(steps.ShellUtilityInput); ok {
		if suite.Len(secondArg.Command, 3, "Expected the last command to have three parts") {
			script := secondArg.Command[2]
			regex := fmt.Sprintf("source %s/.deploy/venv/bin/activate && sshuttle --pidfile .* -D -r ubuntu@10.0.0.1 --ssh-cmd 'ssh -i .* -o StrictHostKeyChecking=no' 10.250.0.0/16", suite.rootPath)
			matched, err := regexp.MatchString(regex, script)
			suite.NoError(err, "Error while matching the script with regex")
			suite.True(matched, "The script does not match the expected pattern")
		}
	} else {
		suite.Fail("Expected the last command to be of type ShellUtilityInput")
	}
}
