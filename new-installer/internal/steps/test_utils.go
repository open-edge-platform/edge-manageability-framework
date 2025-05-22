// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps

import (
	"context"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
)

func GoThroughStepFunctions(step OrchInstallerStep, config *config.OrchInstallerConfig) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	ctx := context.Background()
	newRS, err := step.ConfigStep(ctx, *config)
	if err != nil {
		return newRS, err
	}
	err = internal.UpdateRuntimeState(&config.Generated, newRS)
	if err != nil {
		return newRS, err
	}

	newRS, err = step.PreStep(ctx, *config)
	if err != nil {
		return newRS, err
	}
	err = internal.UpdateRuntimeState(&config.Generated, newRS)
	if err != nil {
		return newRS, err
	}

	newRS, err = step.RunStep(ctx, *config)
	if err != nil {
		return newRS, err
	}
	err = internal.UpdateRuntimeState(&config.Generated, newRS)
	if err != nil {
		return newRS, err
	}

	newRS, err = step.PostStep(ctx, *config, err)
	if err != nil {
		return newRS, err
	}
	err = internal.UpdateRuntimeState(&config.Generated, newRS)
	if err != nil {
		return newRS, err
	}
	return newRS, nil
}
