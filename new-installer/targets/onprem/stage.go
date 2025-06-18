// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package onprem

import (
	"context"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
)

type OnPremStage struct {
	steps                  []steps.OrchInstallerStep
	name                   string
	labels                 []string
	orchConfigReaderWriter config.OrchConfigReaderWriter
}

func NewOnPremStage(name string, steps []steps.OrchInstallerStep, labels []string, orchConfigReaderWriter config.OrchConfigReaderWriter) internal.OrchInstallerStage {
	return &OnPremStage{
		steps:                  steps,
		name:                   name,
		labels:                 labels,
		orchConfigReaderWriter: orchConfigReaderWriter,
	}
}

func (a *OnPremStage) Name() string {
	return a.name
}

func (a *OnPremStage) Labels() []string {
	return a.labels
}

func (a *OnPremStage) PreStage(ctx context.Context, config *config.OrchInstallerConfig, runtimeState *config.OrchInstallerRuntimeState) *internal.OrchInstallerError {
	return nil
}

func (a *OnPremStage) RunStage(ctx context.Context, config *config.OrchInstallerConfig, runtimeState *config.OrchInstallerRuntimeState) *internal.OrchInstallerError {
	logger := internal.Logger()
	if config == nil {
		return &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "OrchInstallerConfig is nil",
		}
	}
	if runtimeState.Action == "uninstall" {
		a.steps = steps.ReverseSteps(a.steps)
	}
	a.steps = steps.FilterSteps(a.steps, runtimeState.TargetLabels)
	if len(a.steps) == 0 {
		return nil
	}

	for _, step := range a.steps {
		logger.Debugf("ConfigStep %s", step.Name())
		stepErr := func() *internal.OrchInstallerError {
			if newRuntimeState, err := step.ConfigStep(ctx, *config, *runtimeState); err != nil {
				return err
			} else if err = internal.UpdateRuntimeState(runtimeState, newRuntimeState); err != nil {
				return err
			}
			logger.Debugf("PreStep %s", step.Name())
			if newRuntimeState, err := step.PreStep(ctx, *config, *runtimeState); err != nil {
				return err
			} else if err = internal.UpdateRuntimeState(runtimeState, newRuntimeState); err != nil {
				return err
			}
			logger.Debugf("RunStep %s", step.Name())
			if newRuntimeState, err := step.RunStep(ctx, *config, *runtimeState); err != nil {
				return err
			} else if err = internal.UpdateRuntimeState(runtimeState, newRuntimeState); err != nil {
				return err
			}
			return nil
		}()

		logger.Debugf("PostStep %s", step.Name())
		if newRuntimeState, err := step.PostStep(ctx, *config, *runtimeState, stepErr); err != nil {
			return err
		} else if err = internal.UpdateRuntimeState(runtimeState, newRuntimeState); err != nil {
			return err
		}
	}
	return nil
}

func (a *OnPremStage) PostStage(ctx context.Context, config *config.OrchInstallerConfig, runtimeState *config.OrchInstallerRuntimeState, prevStageError *internal.OrchInstallerError) *internal.OrchInstallerError {
	return nil
}
