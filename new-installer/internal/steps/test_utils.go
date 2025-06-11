// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps

import (
	"context"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
)

func GoThroughStepFunctions(step OrchInstallerStep, config *config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	ctx := context.Background()
	newRS, err := step.ConfigStep(ctx, *config, runtimeState)
	if err != nil {
		return newRS, err
	}
	err = internal.UpdateRuntimeState(&runtimeState, newRS)
	if err != nil {
		return newRS, err
	}

	newRS, err = step.PreStep(ctx, *config, runtimeState)
	if err != nil {
		return newRS, err
	}
	err = internal.UpdateRuntimeState(&runtimeState, newRS)
	if err != nil {
		return newRS, err
	}

	newRS, err = step.RunStep(ctx, *config, runtimeState)
	if err != nil {
		return newRS, err
	}
	err = internal.UpdateRuntimeState(&runtimeState, newRS)
	if err != nil {
		return newRS, err
	}

	newRS, err = step.PostStep(ctx, *config, runtimeState, err)
	if err != nil {
		return newRS, err
	}
	err = internal.UpdateRuntimeState(&runtimeState, newRS)
	if err != nil {
		return newRS, err
	}
	return newRS, nil
}
