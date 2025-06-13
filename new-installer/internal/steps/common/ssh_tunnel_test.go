// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps_common_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	steps_common "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/common"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type SSHTunnelTestSuite struct {
	suite.Suite
	s                *steps_common.SSHTunnelStep
	shellUtilityMock *ShellUtilityMock
}

func TestSSHTunnelSuite(t *testing.T) {
	suite.Run(t, new(SSHTunnelTestSuite))
}

func (suite *SSHTunnelTestSuite) SetupTest() {
	suite.shellUtilityMock = &ShellUtilityMock{}
	suite.s = steps_common.CreateSSHTunnelStep(
		suite.shellUtilityMock,
	)
}

func (suite *SSHTunnelTestSuite) TestStartSSHTunnel() {
	cfg := &config.OrchInstallerConfig{}
	runtimeState := config.OrchInstallerRuntimeState{}
	runtimeState.AWS.JumpHostIP = "10.0.0.1"
	runtimeState.AWS.JumpHostSSHKeyPrivateKey = "foo"
	suite.shellUtilityMock.On("Run", mock.Anything, mock.Anything).Return(&steps.ShellUtilityOutput{}, nil).Once()

	_, err := steps.GoThroughStepFunctions(suite.s, cfg, runtimeState)
	if err != nil {
		suite.Require().NoError(err, "Expected no error during step execution")
	}
	if !suite.NotEmpty(suite.shellUtilityMock.Calls, "Expected shell utility to be called") {
		return
	}
	shUtilcallArgs := suite.shellUtilityMock.Calls[0].Arguments
	if suite.Len(shUtilcallArgs, 2, "Expected shell utility call to have two arguments") {
		return
	}
	shellInput, ok := shUtilcallArgs[1].(steps.ShellUtilityInput)
	if !ok {
		suite.Fail("Expected second argument to be of type ShellUtilityInput")
		return
	}
	cmdStr := strings.Join(shellInput.Command, " ")
	expectCmdStrRegexp := `ssh -f -N -n -D \d+ -F \w+ jumphost`
	suite.NotEmpty(regexp.MustCompile(expectCmdStrRegexp).FindString(cmdStr), "Expected command to match regex: %s", expectCmdStrRegexp)
}
