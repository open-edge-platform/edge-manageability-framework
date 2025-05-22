// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps

import (
	"context"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
)

type OrchInstallerStep interface {
	// The name of the step
	Name() string

	// Configure the step, such as generating configuration files or setting up the environment.
	ConfigStep(ctx context.Context, config internal.OrchInstallerConfig) (internal.OrchInstallerRuntimeState, *internal.OrchInstallerError)

	// PreStep is called before the main step logic. It can be used to perform any necessary setup or checks
	// For example, running some script before upgrade from previous version.
	PreStep(ctx context.Context, config internal.OrchInstallerConfig) (internal.OrchInstallerRuntimeState, *internal.OrchInstallerError)

	// RunStep is the main logic of the step. It should perform the core functionality of the step.
	// For example, running a script to install or configure something.
	RunStep(ctx context.Context, config internal.OrchInstallerConfig) (internal.OrchInstallerRuntimeState, *internal.OrchInstallerError)

	// PostStep is called after the main step logic. It can be used to perform any necessary cleanup or finalization.
	// This step will always be called, even if the config, pre, or main step logic fails.
	// It should handle errors gracefully before returning.
	PostStep(ctx context.Context, config internal.OrchInstallerConfig, prevStepError *internal.OrchInstallerError) (internal.OrchInstallerRuntimeState, *internal.OrchInstallerError)
}
