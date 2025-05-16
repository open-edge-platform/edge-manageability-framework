// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps

import (
	"context"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
)

type OrchInstallerStep interface {
	Name() string
	ConfigStep(ctx context.Context, config internal.OrchInstallerConfig, runtimeState internal.OrchInstallerRuntimeState) (internal.OrchInstallerRuntimeState, *internal.OrchInstallerError)
	PreSetp(ctx context.Context, config internal.OrchInstallerConfig, runtimeState internal.OrchInstallerRuntimeState) (internal.OrchInstallerRuntimeState, *internal.OrchInstallerError)
	RunStep(ctx context.Context, config internal.OrchInstallerConfig, runtimeState internal.OrchInstallerRuntimeState) (internal.OrchInstallerRuntimeState, *internal.OrchInstallerError)
	PostStep(ctx context.Context, config internal.OrchInstallerConfig, runtimeState internal.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (internal.OrchInstallerRuntimeState, *internal.OrchInstallerError)
}

func RunSteps(steps []OrchInstallerStep, ctx context.Context, config internal.OrchInstallerConfig, runtimeState *internal.OrchInstallerRuntimeState) *internal.OrchInstallerError {
	for _, step := range steps {
		if newRuntimeState, err := step.ConfigStep(ctx, config, *runtimeState); err != nil {
			return err
		} else {
			if updateErr := runtimeState.UpdateRuntimeState(newRuntimeState); updateErr != nil {
				return updateErr
			}
		}
		newRuntimeState, err := step.PreSetp(ctx, config, *runtimeState)
		if err == nil {
			if updateErr := runtimeState.UpdateRuntimeState(newRuntimeState); updateErr != nil {
				return updateErr
			}
			newRuntimeState, err = step.RunStep(ctx, config, *runtimeState)
			if updateErr := runtimeState.UpdateRuntimeState(newRuntimeState); updateErr != nil {
				return updateErr
			}
		}
		if newRuntimeState, err = step.PostStep(ctx, config, *runtimeState, err); err != nil {
			return err
		} else {
			if updateErr := runtimeState.UpdateRuntimeState(newRuntimeState); updateErr != nil {
				return updateErr
			}
		}
	}
	return nil
}
