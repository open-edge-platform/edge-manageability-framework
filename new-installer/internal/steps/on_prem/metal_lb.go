// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps_on_prem

import (
	"context"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
)

type OnPremMetalLBStep struct {
}

// The name of the step
func (s *OnPremMetalLBStep) Name() string {
	return "OnPremMetalLBStep"
}

// Labels for the step. We can selectively run a subset of steps by specifying labels.
func (s *OnPremMetalLBStep) Labels() []string {
	return []string{"on-prem", "vm"}
}

// Configure the step, such as generating configuration files or setting up the environment.
func (s *OnPremMetalLBStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

// PreStep is called before the main step logic. It can be used to perform any necessary setup or checks
// For example, running some script before upgrade from previous version.
func (s *OnPremMetalLBStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

// RunStep is the main logic of the step. It should perform the core functionality of the step.
// For example, running a script to install or configure something.
func (s *OnPremMetalLBStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

// PostStep is called after the main step logic. It can be used to perform any necessary cleanup or finalization.
// This step will always be called, even if the config, pre, or main step logic fails.
// It should handle errors gracefully before returning.
func (s *OnPremMetalLBStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}
