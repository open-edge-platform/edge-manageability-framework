// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps

import (
	"context"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
)

type OrchInstallerStep interface {
	// The name of the step
	Name() string

	// Labels for the step. We can selectively run a subset of steps by specifying labels.
	Labels() []string

	// Configure the step, such as generating configuration files or setting up the environment.
	ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError)

	// PreStep is called before the main step logic. It can be used to perform any necessary setup or checks
	// For example, running some script before upgrade from previous version.
	PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError)

	// RunStep is the main logic of the step. It should perform the core functionality of the step.
	// For example, running a script to install or configure something.
	RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError)

	// PostStep is called after the main step logic. It can be used to perform any necessary cleanup or finalization.
	// This step will always be called, even if the config, pre, or main step logic fails.
	// It should handle errors gracefully before returning.
	PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError)
}

func matchAnyLabel(stepLabels []string, filterLabels []string) bool {
	for _, label := range stepLabels {
		for _, filterLabel := range filterLabels {
			if label == filterLabel {
				return true
			}
		}
	}
	return false
}

func FilterSteps(steps []OrchInstallerStep, labels []string) []OrchInstallerStep {
	if len(labels) == 0 {
		return steps
	}
	var filteredSteps []OrchInstallerStep
	for _, step := range steps {
		if matchAnyLabel(step.Labels(), labels) {
			filteredSteps = append(filteredSteps, step)
		}
	}
	return filteredSteps
}

func ReverseSteps(steps []OrchInstallerStep) []OrchInstallerStep {
	var reversedSteps []OrchInstallerStep
	for i := len(steps) - 1; i >= 0; i-- {
		reversedSteps = append(reversedSteps, steps[i])
	}
	return reversedSteps
}
