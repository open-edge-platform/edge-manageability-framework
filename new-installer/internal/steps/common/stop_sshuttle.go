// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps_common

import (
	"context"
	"fmt"
	"log"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
)

type StopSshuttleStep struct {
	ShellUtility steps.ShellUtility
}

var stopSshuttleStepLabels = []string{"common", "stop-sshuttle"}

func (s *StopSshuttleStep) Name() string {
	return "StopSshuttleStep"
}

func (s *StopSshuttleStep) Labels() []string {
	return stopSshuttleStepLabels
}

func (s *StopSshuttleStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

func (s *StopSshuttleStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if s.ShellUtility == nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  "Shell utility is not initialized.",
		}
	}
	return runtimeState, nil
}

func (s *StopSshuttleStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if runtimeState.SshuttlePID == "" {
		// No existing sshuttle process to stop
		return runtimeState, nil
	}
	_, err := s.ShellUtility.Run(ctx, steps.ShellUtilityInput{
		Command:         []string{"sudo", "kill", runtimeState.SshuttlePID},
		Timeout:         10,
		SkipError:       false,
		RunInBackground: false,
	})
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to stop existing sshuttle process: %v", err),
		}
	}
	log.Println("Stopped existing sshuttle process.")
	return runtimeState, nil
}

func (s *StopSshuttleStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}
