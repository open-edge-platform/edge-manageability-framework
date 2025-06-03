// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_test

import (
	"context"
	"testing"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	"github.com/open-edge-platform/edge-manageability-framework/installer/targets/aws"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type OrchInstallerStepMock struct {
	mock.Mock
}

func (m *OrchInstallerStepMock) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *OrchInstallerStepMock) ConfigStep(ctx context.Context, installerConfig config.OrchInstallerConfig, rs config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	args := m.Called(ctx, installerConfig)
	err, _ := args.Get(1).(*internal.OrchInstallerError)
	newRS, _ := args.Get(0).(*config.OrchInstallerRuntimeState)
	return *newRS, err
}

func (m *OrchInstallerStepMock) PreStep(ctx context.Context, installerConfig config.OrchInstallerConfig, rs config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	args := m.Called(ctx, installerConfig)
	err, _ := args.Get(1).(*internal.OrchInstallerError)
	newRS, _ := args.Get(0).(*config.OrchInstallerRuntimeState)
	return *newRS, err
}

func (m *OrchInstallerStepMock) RunStep(ctx context.Context, installerConfig config.OrchInstallerConfig, rs config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	args := m.Called(ctx, installerConfig)
	err, _ := args.Get(1).(*internal.OrchInstallerError)
	newRS, _ := args.Get(0).(*config.OrchInstallerRuntimeState)
	return *newRS, err
}

func (m *OrchInstallerStepMock) PostStep(ctx context.Context, installerConfig config.OrchInstallerConfig, rs config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	args := m.Called(ctx, installerConfig)
	err, _ := args.Get(1).(*internal.OrchInstallerError)
	newRS, _ := args.Get(0).(*config.OrchInstallerRuntimeState)
	return *newRS, err
}

func (m *OrchInstallerStepMock) Labels() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

type DummyOrchConfigReaderWriter struct{}

func (DummyOrchConfigReaderWriter) WriteOrchConfig(orchConfig config.OrchInstallerConfig) error {
	return nil
}

func (DummyOrchConfigReaderWriter) ReadOrchConfig() (config.OrchInstallerConfig, error) {
	return config.OrchInstallerConfig{}, nil
}

func (DummyOrchConfigReaderWriter) WriteRuntimeState(runtimeState config.OrchInstallerRuntimeState) error {
	return nil
}

func (DummyOrchConfigReaderWriter) ReadRuntimeState() (config.OrchInstallerRuntimeState, error) {
	return config.OrchInstallerRuntimeState{}, nil
}

type OrchInstallerStageTest struct {
	suite.Suite
}

func TestStageSuite(t *testing.T) {
	suite.Run(t, new(OrchInstallerStageTest))
}

func createMockStep(name string, expectToRun bool, labels []string) *OrchInstallerStepMock {
	step := &OrchInstallerStepMock{}
	step.On("Name").Return(name)
	step.On("Labels").Return(labels)
	if expectToRun {
		newRS := &config.OrchInstallerRuntimeState{}
		step.On("ConfigStep", mock.Anything, mock.Anything).Return(newRS, nil)
		step.On("PreStep", mock.Anything, mock.Anything).Return(newRS, nil)
		step.On("RunStep", mock.Anything, mock.Anything).Return(newRS, nil)
		step.On("PostStep", mock.Anything, mock.Anything).Return(newRS, nil)
	}
	return step
}

// Should run all steps when no labels are provided
func (s *OrchInstallerStageTest) TestRunAllSteps() {
	ctx := context.Background()
	orchConfig := config.OrchInstallerConfig{}
	runtimeState := config.OrchInstallerRuntimeState{
		Action: "install",
	}
	steps := []steps.OrchInstallerStep{
		createMockStep("step1", true, []string{"label1", "label2"}),
		createMockStep("step2", true, []string{"label2", "label3"}),
	}
	stage := aws.NewAWSStage("stage1", steps, []string{"stage1"}, &DummyOrchConfigReaderWriter{})

	err := stage.PreStage(ctx, &orchConfig, &runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}

	err = stage.RunStage(ctx, &orchConfig, &runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}

	err = stage.PostStage(ctx, &orchConfig, &runtimeState, nil)
	if err != nil {
		s.NoError(err)
		return
	}
}

// Should run filtered steps when labels are provided
func (s *OrchInstallerStageTest) TestRunFilteredSteps() {
	ctx := context.Background()
	orchConfig := config.OrchInstallerConfig{}
	runtimeState := config.OrchInstallerRuntimeState{
		Action: "install",
	}
	orchConfig.Advanced.TargetLabels = []string{"label1"}
	steps := []steps.OrchInstallerStep{
		createMockStep("step1", true, []string{"label1", "label2"}),
		createMockStep("step2", false, []string{"label2", "label3"}),
	}
	stage := aws.NewAWSStage("stage1", steps, []string{"stage1"}, &DummyOrchConfigReaderWriter{})

	err := stage.PreStage(ctx, &orchConfig, &runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}

	err = stage.RunStage(ctx, &orchConfig, &runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}

	err = stage.PostStage(ctx, &orchConfig, &runtimeState, nil)
	if err != nil {
		s.NoError(err)
		return
	}
}
