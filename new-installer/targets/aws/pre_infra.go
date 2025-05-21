// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"context"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
)

type PreInfraStage struct {
	// The root path of the edge-managability-framework repo
	// This is used to find the terraform files and right log path
	RootPath string
	// Keeps the generated files such as Terraform variables and backend config.
	KeepGeneratedFiles bool

	steps []steps.OrchInstallerStep
}

func NewPreInfraStage(rootPath string, keepGeneratedFiles bool) *PreInfraStage {
	return &PreInfraStage{
		RootPath:           rootPath,
		KeepGeneratedFiles: keepGeneratedFiles,
		steps:              []steps.OrchInstallerStep{},
	}
}

func (a *PreInfraStage) Name() string {
	return "PreInfraStage"
}
func (a *PreInfraStage) PreStage(ctx context.Context, config internal.OrchInstallerConfig, runtimeState *internal.OrchInstallerRuntimeState) *internal.OrchInstallerStageError {
	containsError := false
	stepErrors := make([]*internal.OrchInstallerError, len(a.steps))
	for i, step := range a.steps {
		if newRuntimeState, err := step.ConfigStep(ctx, config, *runtimeState); err != nil {
			stepErrors[i] = err
			containsError = true
		} else if err = runtimeState.UpdateRuntimeState(newRuntimeState); err != nil {
			stepErrors[i] = err
			containsError = true
		}
		if newRuntimeState, err := step.PreStep(ctx, config, *runtimeState); err != nil {
			stepErrors[i] = err
			containsError = true
		} else if err = runtimeState.UpdateRuntimeState(newRuntimeState); err != nil {
			stepErrors[i] = err
			containsError = true
		}
	}
	if containsError {
		return &internal.OrchInstallerStageError{
			StepErrors: stepErrors,
		}
	}
	return nil
}

func (a *PreInfraStage) RunStage(ctx context.Context, config internal.OrchInstallerConfig, runtimeState *internal.OrchInstallerRuntimeState) *internal.OrchInstallerStageError {
	containsError := false
	stepErrors := make([]*internal.OrchInstallerError, len(a.steps))
	for i, step := range a.steps {
		if newRuntimeState, err := step.RunStep(ctx, config, *runtimeState); err != nil {
			stepErrors[i] = err
			containsError = true
		} else if err = runtimeState.UpdateRuntimeState(newRuntimeState); err != nil {
			stepErrors[i] = err
			containsError = true
		}
	}
	if containsError {
		return &internal.OrchInstallerStageError{
			StepErrors: stepErrors,
		}
	}
	return nil
}

func (a *PreInfraStage) PostStage(ctx context.Context, config internal.OrchInstallerConfig, runtimeState *internal.OrchInstallerRuntimeState, prevStageError *internal.OrchInstallerStageError) *internal.OrchInstallerStageError {
	containsError := false
	stepErrors := make([]*internal.OrchInstallerError, len(a.steps))
	for i, step := range a.steps {
		stepError := prevStageError.StepErrors[i]
		if newRuntimeState, err := step.PostStep(ctx, config, *runtimeState, stepError); err != nil {
			stepErrors[i] = err
			containsError = true
		} else if err = runtimeState.UpdateRuntimeState(newRuntimeState); err != nil {
			stepErrors[i] = err
			containsError = true
		}
	}
	if containsError {
		return &internal.OrchInstallerStageError{
			StepErrors: stepErrors,
		}
	}
	return nil
}
