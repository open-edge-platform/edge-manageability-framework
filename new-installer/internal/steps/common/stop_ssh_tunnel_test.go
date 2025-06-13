// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps_common_test

import (
	"testing"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	steps_common "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/common"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type StopSSHTunnelTestSuite struct {
	suite.Suite
	s                *steps_common.StopSSHTunnelStep
	shellUtilityMock *ShellUtilityMock
}

func TestStopSSHTunnelSuite(t *testing.T) {
	suite.Run(t, new(StopSSHTunnelTestSuite))
}

func (suite *StopSSHTunnelTestSuite) SetupTest() {
	suite.shellUtilityMock = &ShellUtilityMock{}
	suite.s = steps_common.CreateStopSSHTunnelStep(
		suite.shellUtilityMock,
	)
}

func (suite *StopSSHTunnelTestSuite) TestStartStopSSHTunnel() {
	cfg := &config.OrchInstallerConfig{}
	runtimeState := config.OrchInstallerRuntimeState{}
	runtimeState.AWS.JumpHostSocks5TunnelPID = 12345
	suite.shellUtilityMock.On("Run", mock.Anything, steps.ShellUtilityInput{
		Command:         []string{"kill", "12345"},
		Timeout:         10,
		SkipError:       true,
		RunInBackground: false,
	}).Return(&steps.ShellUtilityOutput{}, nil).Once()
	_, err := steps.GoThroughStepFunctions(suite.s, cfg, runtimeState)
	if err != nil {
		suite.NoError(err, "Expected no error during step execution")
	}
}
