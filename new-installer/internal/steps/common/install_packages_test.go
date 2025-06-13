// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps_common_test

import (
	"path/filepath"
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
	suite.shellUtilityMock.On("Run", mock.Anything, steps.ShellUtilityInput{
		Command: []string{
			"curl", "-Lo", filepath.Join(suite.rootDir, ".deploy/bin/kubectl"),
			"https://dl.k8s.io/release/v1.32.5/bin/linux/amd64/kubectl",
		},
		Timeout: 1800,
	}).Return(&steps.ShellUtilityOutput{}, nil).Once()
	_, err := steps.GoThroughStepFunctions(suite.s, cfg, runtimeState)
	if err != nil {
		suite.NoError(err, "Expected no error during step execution")
	}
}
