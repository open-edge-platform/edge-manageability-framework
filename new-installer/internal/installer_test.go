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

func (m *OrchInstallerStageMock) PreStage(ctx context.Context, config *config.OrchInstallerConfig) *internal.OrchInstallerError {
	args := m.Called(ctx, config)
	if err, ok := args.Get(0).(*internal.OrchInstallerError); ok {
		return err
	}
	return nil
}

func (m *OrchInstallerStageMock) RunStage(ctx context.Context, config *config.OrchInstallerConfig) *internal.OrchInstallerError {
	args := m.Called(ctx, config)
	if err, ok := args.Get(0).(*internal.OrchInstallerError); ok {
		return err
	}
	return nil
}

func (m *OrchInstallerStageMock) PostStage(ctx context.Context, config *config.OrchInstallerConfig, prevStageError *internal.OrchInstallerError) *internal.OrchInstallerError {
	args := m.Called(ctx, config, prevStageError)
	if err, ok := args.Get(0).(*internal.OrchInstallerError); ok {
		return err
	}
	return nil
}

type OrchInstallerTest struct {
	suite.Suite
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(OrchInstallerTest))
}

func createMockStage(name string, extectedToRun bool, labels []string) *OrchInstallerStageMock {
	stage := &OrchInstallerStageMock{}
	stage.On("Name").Return(name)
	stage.On("Labels").Return(labels)
	if extectedToRun {
		stage.On("PreStage", mock.Anything, mock.Anything).Return(nil)
		stage.On("RunStage", mock.Anything, mock.Anything).Return(nil)
		stage.On("PostStage", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	}
	return stage
}

// Installer will install all stages if no labels are specified in the config
func (s *OrchInstallerTest) TestOrchInstallerInstallAllStages() {
	ctx := context.Background()
	orchConfig := config.OrchInstallerConfig{
		Generated: config.OrchInstallerRuntimeState{
			Action: "install",
		},
	}
	stage1 := createMockStage("MockStage1", true, []string{"label1", "label2"})
	stage2 := createMockStage("MockStage2", true, []string{"label3", "label4"})
	installer, err := internal.CreateOrchInstaller([]internal.OrchInstallerStage{stage1, stage2})
	if err != nil {
		s.NoError(err)
		return
	}
	installerErr := installer.Run(ctx, orchConfig)
	if installerErr != nil {
		s.NoError(installerErr)
		return
	}
}

// Installer will not install any stages if no labels are specified in the config
func (s *OrchInstallerTest) TestOrchInstallerInstallNoStages() {
	ctx := context.Background()
	orchConfig := config.OrchInstallerConfig{
		Generated: config.OrchInstallerRuntimeState{
			Action: "install",
		},
	}
	orchConfig.Advanced.TargetLabels = []string{"something-else"}
	stage1 := createMockStage("MockStage1", true, []string{"label1", "label2"})
	stage2 := createMockStage("MockStage2", true, []string{"label3", "label4"})
	installer, err := internal.CreateOrchInstaller([]internal.OrchInstallerStage{stage1, stage2})
	if err != nil {
		s.NoError(err)
		return
	}
	installerErr := installer.Run(ctx, orchConfig)
	if installerErr != nil {
		s.NoError(installerErr)
		return
	}
}

// Installer will uninstall all stages if no labels are specified in the config
func (s *OrchInstallerTest) TestOrchInstallerUninstallAllStages() {
	ctx := context.Background()
	orchConfig := config.OrchInstallerConfig{
		Generated: config.OrchInstallerRuntimeState{
			Action: "uninstall",
		},
	}
	stage1 := createMockStage("MockStage1", true, []string{"label1", "label2"})
	stage2 := createMockStage("MockStage2", true, []string{"label3", "label4"})
	installer, err := internal.CreateOrchInstaller([]internal.OrchInstallerStage{stage1, stage2})
	if err != nil {
		s.NoError(err)
		return
	}
	installerErr := installer.Run(ctx, orchConfig)
	if installerErr != nil {
		s.NoError(installerErr)
		return
	}
}

// Installer will install only the stages that match the labels specified in the config
func (s *OrchInstallerTest) TestOrchInstallerInstallSpecificStagesWithLabel() {
	ctx := context.Background()
	orchConfig := config.OrchInstallerConfig{
		Generated: config.OrchInstallerRuntimeState{
			Action: "install",
		},
	}
	orchConfig.Advanced.TargetLabels = []string{"label1"}
	stage1 := createMockStage("MockStage1", true, []string{"label1", "label2"})
	stage2 := createMockStage("MockStage2", false, []string{"label3", "label4"})
	installer, err := internal.CreateOrchInstaller([]internal.OrchInstallerStage{stage1, stage2})
	if err != nil {
		s.NoError(err)
		return
	}
	installerErr := installer.Run(ctx, orchConfig)
	if installerErr != nil {
		s.NoError(installerErr)
		return
	}

	orchConfig.Advanced.TargetLabels = []string{"label3"}
	stage1 = createMockStage("MockStage1", false, []string{"label1", "label2"})
	stage2 = createMockStage("MockStage2", true, []string{"label3", "label4"})
	installer, err = internal.CreateOrchInstaller([]internal.OrchInstallerStage{stage1, stage2})
	if err != nil {
		s.NoError(err)
		return
	}
	installerErr = installer.Run(ctx, orchConfig)
	if installerErr != nil {
		s.NoError(installerErr)
		return
	}
}

// Installer will install only the stages that match the labels specified in the config
func (s *OrchInstallerTest) TestOrchInstallerInstallSpecificStagesWithTwoLabels() {
	ctx := context.Background()
	orchConfig := config.OrchInstallerConfig{
		Generated: config.OrchInstallerRuntimeState{
			Action: "install",
		},
	}
	orchConfig.Advanced.TargetLabels = []string{"label1", "label3"}
	stage1 := createMockStage("MockStage1", true, []string{"label1", "label2"})
	stage2 := createMockStage("MockStage2", true, []string{"label3", "label4"})
	installer, err := internal.CreateOrchInstaller([]internal.OrchInstallerStage{stage1, stage2})
	if err != nil {
		s.NoError(err)
		return
	}
	installerErr := installer.Run(ctx, orchConfig)
	if installerErr != nil {
		s.NoError(installerErr)
		return
	}
}

func (s *OrchInstallerTest) TestOrchInstallerInvalidArgument() {
	ctx := context.Background()
	orchConfig := config.OrchInstallerConfig{
		Generated: config.OrchInstallerRuntimeState{
			Action: "",
		},
	}
	installer, err := internal.CreateOrchInstaller([]internal.OrchInstallerStage{})
	if err != nil {
		s.NoError(err)
		return
	}
	installerErr := installer.Run(ctx, orchConfig)
	s.Equal(installerErr, &internal.OrchInstallerError{
		ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
		ErrorMsg:  "action must be specified",
	})

	orchConfig.Generated.Action = "invalid"
	installerErr = installer.Run(ctx, orchConfig)
	s.Equal(installerErr, &internal.OrchInstallerError{
		ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
		ErrorMsg:  "unsupported action: invalid",
	})
}

func (s *OrchInstallerTest) TestUpdateRuntimeState() {
	orchConfig := config.OrchInstallerConfig{
		Generated: config.OrchInstallerRuntimeState{},
	}
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

	err := internal.UpdateRuntimeState(&orchConfig.Generated, newRuntimeState)
	if err != nil {
		s.NoError(err)
		return
	}
	s.Equal(orchConfig.Generated.DryRun, newRuntimeState.DryRun)
	s.Equal(orchConfig.Generated.Action, newRuntimeState.Action)
	s.Equal(orchConfig.Generated.LogDir, newRuntimeState.LogDir)
	s.Equal(orchConfig.Generated.DeploymentID, newRuntimeState.DeploymentID)
	s.Equal(orchConfig.Generated.StateBucketState, newRuntimeState.StateBucketState)
	s.Equal(orchConfig.Generated.KubeConfig, newRuntimeState.KubeConfig)
	s.Equal(orchConfig.Generated.TLSCert, newRuntimeState.TLSCert)
	s.Equal(orchConfig.Generated.TLSKey, newRuntimeState.TLSKey)
	s.Equal(orchConfig.Generated.TLSCa, newRuntimeState.TLSCa)
	s.Equal(orchConfig.Generated.CacheRegistry, newRuntimeState.CacheRegistry)
	s.Equal(orchConfig.Generated.VPCID, newRuntimeState.VPCID)
	s.Equal(orchConfig.Generated.PublicSubnetIDs, newRuntimeState.PublicSubnetIDs)
	s.Equal(orchConfig.Generated.PrivateSubnetIDs, newRuntimeState.PrivateSubnetIDs)
	s.Equal(orchConfig.Generated.JumpHostSSHKeyPublicKey, newRuntimeState.JumpHostSSHKeyPublicKey)
	s.Equal(orchConfig.Generated.JumpHostSSHKeyPrivateKey, newRuntimeState.JumpHostSSHKeyPrivateKey)
}
