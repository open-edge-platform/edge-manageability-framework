// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"

	config "github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
)

type OrchInstallerStage interface {
	Name() string
	// Labels for the stage, We can selectively run a subset of stages by specifying labels.
	Labels() []string
	// PreStage: initialize the stage, such as creating directories, downloading files, etc.
	// It also process the output/runtime-state from previous stage.
	PreStage(ctx context.Context, config *config.OrchInstallerConfig) *OrchInstallerError

	// RunStage: run the stage, such as running terraform, ansible, etc.
	RunStage(ctx context.Context, config *config.OrchInstallerConfig) *OrchInstallerError

	// PostStage: cleanup the stage, such as removing directories, files, etc.
	// It should also handle the error from the previous stage and rollback if needed.
	// It should also return the final output of the stage.
	PostStage(ctx context.Context, config *config.OrchInstallerConfig, prevStageError *OrchInstallerError) *OrchInstallerError
}

func ReverseStages(stages []OrchInstallerStage) []OrchInstallerStage {
	reversed := []OrchInstallerStage{}
	for i := len(stages) - 1; i >= 0; i-- {
		reversed = append(reversed, stages[i])
	}
	return reversed
}

func matchAnyLabel(stageLabels []string, filterLabels []string) bool {
	for _, label := range stageLabels {
		for _, filterLabel := range filterLabels {
			if label == filterLabel {
				return true
			}
		}
	}
	return false
}

func FilterStages(stages []OrchInstallerStage, labels []string) []OrchInstallerStage {
	if len(labels) == 0 {
		return stages
	}
	filtered := []OrchInstallerStage{}
	for _, stage := range stages {
		if matchAnyLabel(stage.Labels(), labels) {
			filtered = append(filtered, stage)
		}
	}
	return filtered
}
