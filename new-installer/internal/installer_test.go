// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package internal_test

import (
	"context"
	"testing"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type OrchInstallerStageMock struct {
	mock.Mock
}

func (m *OrchInstallerStageMock) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *OrchInstallerStageMock) Labels() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

func (m *OrchInstallerStageMock) PreStage(ctx context.Context, config *config.OrchInstallerConfig, rs *config.OrchInstallerRuntimeState) *internal.OrchInstallerError {
	args := m.Called(ctx, config)
	if err, ok := args.Get(0).(*internal.OrchInstallerError); ok {
		return err
	}
	return nil
}

func (m *OrchInstallerStageMock) RunStage(ctx context.Context, config *config.OrchInstallerConfig, rs *config.OrchInstallerRuntimeState) *internal.OrchInstallerError {
	args := m.Called(ctx, config)
	if err, ok := args.Get(0).(*internal.OrchInstallerError); ok {
		return err
	}
	return nil
}

func (m *OrchInstallerStageMock) PostStage(ctx context.Context, config *config.OrchInstallerConfig, rs *config.OrchInstallerRuntimeState, prevStageError *internal.OrchInstallerError) *internal.OrchInstallerError {
	args := m.Called(ctx, config, prevStageError)
	if err, ok := args.Get(0).(*internal.OrchInstallerError); ok {
		return err
	}
	return nil
}

type OrchInstallerTest struct {
	suite.Suite
}

func TestInstallerSuite(t *testing.T) {
	suite.Run(t, new(OrchInstallerTest))
}

func createMockStage(name string, expectToRun bool, labels []string) *OrchInstallerStageMock {
	stage := &OrchInstallerStageMock{}
	stage.On("Name").Return(name)
	stage.On("Labels").Return(labels)
	if expectToRun {
		stage.On("PreStage", mock.Anything, mock.Anything).Return(nil)
		stage.On("RunStage", mock.Anything, mock.Anything).Return(nil)
		stage.On("PostStage", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	}
	return stage
}

// Installer will install all stages if no labels are specified in the config
func (s *OrchInstallerTest) TestOrchInstallerInstallAllStages() {
	ctx := context.Background()
	orchConfig := config.OrchInstallerConfig{}
	runtimeState := config.OrchInstallerRuntimeState{
		Action: "install",
	}
	stage1 := createMockStage("MockStage1", true, []string{"label1", "label2"})
	stage2 := createMockStage("MockStage2", true, []string{"label3", "label4"})
	installer, err := internal.CreateOrchInstaller([]internal.OrchInstallerStage{stage1, stage2})
	if err != nil {
		s.NoError(err)
		return
	}
	installerErr := installer.Run(ctx, orchConfig, &runtimeState)
	if installerErr != nil {
		s.NoError(installerErr)
		return
	}
}

// Installer will not install any stages if no labels are specified in the config
func (s *OrchInstallerTest) TestOrchInstallerInstallNoStages() {
	ctx := context.Background()
	orchConfig := config.OrchInstallerConfig{}
	runtimeState := config.OrchInstallerRuntimeState{
		Action: "install",
	}
	runtimeState.TargetLabels = []string{"something-else"}
	stage1 := createMockStage("MockStage1", true, []string{"label1", "label2"})
	stage2 := createMockStage("MockStage2", true, []string{"label3", "label4"})
	installer, err := internal.CreateOrchInstaller([]internal.OrchInstallerStage{stage1, stage2})
	if err != nil {
		s.NoError(err)
		return
	}
	installerErr := installer.Run(ctx, orchConfig, &runtimeState)
	if installerErr != nil {
		s.NoError(installerErr)
		return
	}
}

// Installer will uninstall all stages if no labels are specified in the config
func (s *OrchInstallerTest) TestOrchInstallerUninstallAllStages() {
	ctx := context.Background()
	orchConfig := config.OrchInstallerConfig{}
	runtimeState := config.OrchInstallerRuntimeState{
		Action: "install",
	}
	stage1 := createMockStage("MockStage1", true, []string{"label1", "label2"})
	stage2 := createMockStage("MockStage2", true, []string{"label3", "label4"})
	installer, err := internal.CreateOrchInstaller([]internal.OrchInstallerStage{stage1, stage2})
	if err != nil {
		s.NoError(err)
		return
	}
	installerErr := installer.Run(ctx, orchConfig, &runtimeState)
	if installerErr != nil {
		s.NoError(installerErr)
		return
	}
}

// Installer will install only the stages that match the labels specified in the config
func (s *OrchInstallerTest) TestOrchInstallerInstallSpecificStagesWithLabel() {
	ctx := context.Background()
	orchConfig := config.OrchInstallerConfig{}
	runtimeState := config.OrchInstallerRuntimeState{
		Action: "install",
	}
	runtimeState.TargetLabels = []string{"label1"}
	stage1 := createMockStage("MockStage1", true, []string{"label1", "label2"})
	stage2 := createMockStage("MockStage2", false, []string{"label3", "label4"})
	installer, err := internal.CreateOrchInstaller([]internal.OrchInstallerStage{stage1, stage2})
	if err != nil {
		s.NoError(err)
		return
	}
	installerErr := installer.Run(ctx, orchConfig, &runtimeState)
	if installerErr != nil {
		s.NoError(installerErr)
		return
	}

	runtimeState.TargetLabels = []string{"label3"}
	stage1 = createMockStage("MockStage1", false, []string{"label1", "label2"})
	stage2 = createMockStage("MockStage2", true, []string{"label3", "label4"})
	installer, err = internal.CreateOrchInstaller([]internal.OrchInstallerStage{stage1, stage2})
	if err != nil {
		s.NoError(err)
		return
	}
	installerErr = installer.Run(ctx, orchConfig, &runtimeState)
	if installerErr != nil {
		s.NoError(installerErr)
		return
	}
}

// Installer will install only the stages that match the labels specified in the config
func (s *OrchInstallerTest) TestOrchInstallerInstallSpecificStagesWithTwoLabels() {
	ctx := context.Background()
	orchConfig := config.OrchInstallerConfig{}
	runtimeState := config.OrchInstallerRuntimeState{
		Action: "install",
	}
	runtimeState.TargetLabels = []string{"label1", "label3"}
	stage1 := createMockStage("MockStage1", true, []string{"label1", "label2"})
	stage2 := createMockStage("MockStage2", true, []string{"label3", "label4"})
	installer, err := internal.CreateOrchInstaller([]internal.OrchInstallerStage{stage1, stage2})
	if err != nil {
		s.NoError(err)
		return
	}
	installerErr := installer.Run(ctx, orchConfig, &runtimeState)
	if installerErr != nil {
		s.NoError(installerErr)
		return
	}
}

func (s *OrchInstallerTest) TestOrchInstallerInvalidArgument() {
	ctx := context.Background()
	orchConfig := config.OrchInstallerConfig{}
	runtimeState := config.OrchInstallerRuntimeState{}
	installer, err := internal.CreateOrchInstaller([]internal.OrchInstallerStage{})
	if err != nil {
		s.NoError(err)
		return
	}
	installerErr := installer.Run(ctx, orchConfig, &runtimeState)
	s.Equal(installerErr, &internal.OrchInstallerError{
		ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
		ErrorMsg:  "action must be specified",
	})

	runtimeState = config.OrchInstallerRuntimeState{
		Action: "invalid",
	}
	installerErr = installer.Run(ctx, orchConfig, &runtimeState)
	s.Equal(installerErr, &internal.OrchInstallerError{
		ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
		ErrorMsg:  "unsupported action: invalid",
	})
}

func (s *OrchInstallerTest) TestUpdateRuntimeState() {
	runtimeState := config.OrchInstallerRuntimeState{}
	newRuntimeState := config.OrchInstallerRuntimeState{
		Action:                   "install",
		LogDir:                   ".log",
		DryRun:                   true,
		DeploymentID:             "random1",
		StateBucketState:         "random2",
		KubeConfig:               "random3",
		TLSCert:                  "random4",
		TLSKey:                   "random5",
		TLSCa:                    "random6",
		CacheRegistry:            "random7",
		VPCID:                    "random8",
		PublicSubnetIDs:          []string{"10.0.0.0/16"},
		PrivateSubnetIDs:         []string{"10.250.0.0/16"},
		JumpHostSSHKeyPublicKey:  "random9",
		JumpHostSSHKeyPrivateKey: "random10",
	}

	err := internal.UpdateRuntimeState(&runtimeState, newRuntimeState)
	if err != nil {
		s.NoError(err)
		return
	}
	s.Equal(runtimeState.DryRun, newRuntimeState.DryRun)
	s.Equal(runtimeState.Action, newRuntimeState.Action)
	s.Equal(runtimeState.LogDir, newRuntimeState.LogDir)
	s.Equal(runtimeState.DeploymentID, newRuntimeState.DeploymentID)
	s.Equal(runtimeState.StateBucketState, newRuntimeState.StateBucketState)
	s.Equal(runtimeState.KubeConfig, newRuntimeState.KubeConfig)
	s.Equal(runtimeState.TLSCert, newRuntimeState.TLSCert)
	s.Equal(runtimeState.TLSKey, newRuntimeState.TLSKey)
	s.Equal(runtimeState.TLSCa, newRuntimeState.TLSCa)
	s.Equal(runtimeState.CacheRegistry, newRuntimeState.CacheRegistry)
	s.Equal(runtimeState.VPCID, newRuntimeState.VPCID)
	s.Equal(runtimeState.PublicSubnetIDs, newRuntimeState.PublicSubnetIDs)
	s.Equal(runtimeState.PrivateSubnetIDs, newRuntimeState.PrivateSubnetIDs)
	s.Equal(runtimeState.JumpHostSSHKeyPublicKey, newRuntimeState.JumpHostSSHKeyPublicKey)
	s.Equal(runtimeState.JumpHostSSHKeyPrivateKey, newRuntimeState.JumpHostSSHKeyPrivateKey)
}
