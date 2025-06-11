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
	s.Equal(&internal.OrchInstallerError{
		ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
		ErrorMsg:  "action must be specified",
	}, installerErr)

	runtimeState = config.OrchInstallerRuntimeState{
		Action: "invalid",
	}
	installerErr = installer.Run(ctx, orchConfig, &runtimeState)
	s.Equal(&internal.OrchInstallerError{
		ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
		ErrorMsg:  "unsupported action: invalid",
	}, installerErr)
}

func (s *OrchInstallerTest) TestUpdateRuntimeState() {
	runtimeState := config.OrchInstallerRuntimeState{}
	newRuntimeState := config.OrchInstallerRuntimeState{
		Action:           "install",
		LogDir:           ".log",
		DryRun:           true,
		DeploymentID:     "random1",
		StateBucketState: "random2",
	}
	newRuntimeState.AWS.KubeConfig = "random3"
	newRuntimeState.AWS.CacheRegistry = "random7"
	newRuntimeState.AWS.VPCID = "random8"
	newRuntimeState.AWS.PublicSubnetIDs = []string{"10.0.0.0/16"}
	newRuntimeState.AWS.PrivateSubnetIDs = []string{"10.250.0.0/16"}
	newRuntimeState.AWS.JumpHostSSHKeyPublicKey = "random9"
	newRuntimeState.AWS.JumpHostSSHKeyPrivateKey = "random10"
	newRuntimeState.Cert.TLSCert = "random4"
	newRuntimeState.Cert.TLSKey = "random5"
	newRuntimeState.Cert.TLSCA = "random6"

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
	s.Equal(runtimeState.AWS.KubeConfig, newRuntimeState.AWS.KubeConfig)
	s.Equal(runtimeState.Cert.TLSCert, newRuntimeState.Cert.TLSCert)
	s.Equal(runtimeState.Cert.TLSKey, newRuntimeState.Cert.TLSKey)
	s.Equal(runtimeState.Cert.TLSCA, newRuntimeState.Cert.TLSCA)
	s.Equal(runtimeState.AWS.CacheRegistry, newRuntimeState.AWS.CacheRegistry)
	s.Equal(runtimeState.AWS.VPCID, newRuntimeState.AWS.VPCID)
	s.Equal(runtimeState.AWS.PublicSubnetIDs, newRuntimeState.AWS.PublicSubnetIDs)
	s.Equal(runtimeState.AWS.PrivateSubnetIDs, newRuntimeState.AWS.PrivateSubnetIDs)
	s.Equal(runtimeState.AWS.JumpHostSSHKeyPublicKey, newRuntimeState.AWS.JumpHostSSHKeyPublicKey)
	s.Equal(runtimeState.AWS.JumpHostSSHKeyPrivateKey, newRuntimeState.AWS.JumpHostSSHKeyPrivateKey)
}
