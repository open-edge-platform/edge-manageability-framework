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

type StopSshuttletestSuite struct {
	suite.Suite
	s                *steps_common.StopSshuttleStep
	shellUtilityMock *ShellUtilityMock
}

func TestStopSshuttleSuite(t *testing.T) {
	suite.Run(t, new(StopSshuttletestSuite))
}

func (suite *StopSshuttletestSuite) SetupTest() {
	suite.shellUtilityMock = &ShellUtilityMock{}
	suite.s = &steps_common.StopSshuttleStep{
		ShellUtility: suite.shellUtilityMock,
	}
}

func (suite *StopSshuttletestSuite) TestStopSshuttle() {
	cfg := &config.OrchInstallerConfig{}
	runtimeState := config.OrchInstallerRuntimeState{
		SshuttlePID: "12345",
	}

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

	_, err := steps.GoThroughStepFunctions(suite.s, cfg, runtimeState)
	if err != nil {
		suite.NoError(err, "Expected no error during step execution")
	}
}
