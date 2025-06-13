// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps_common

import (
	"context"
	"fmt"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	"go.uber.org/zap"
)

type StopSSHTunnelStep struct {
	ShellUtility steps.ShellUtility
	logger       *zap.SugaredLogger
}

var StopsshTunnelStepLabels = []string{"common", "stop-ssh-tunnel"}

func CreateStopSSHTunnelStep(shellUtility steps.ShellUtility) *StopSSHTunnelStep {
	return &StopSSHTunnelStep{
		ShellUtility: shellUtility,
		logger:       internal.Logger(),
	}
}

func (s *StopSSHTunnelStep) Name() string {
	return "StopSSHTunnelStep"
}

func (s *StopSSHTunnelStep) Labels() []string {
	return StopsshTunnelStepLabels
}

func (s *StopSSHTunnelStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

func (s *StopSSHTunnelStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

func (s *StopSSHTunnelStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if runtimeState.Action == "uninstall" {
		// No need to stop ssh tunnel during uninstall
		return runtimeState, nil
	}
	if runtimeState.AWS.JumpHostSocks5TunnelPID == 0 {
		s.logger.Debug("No SSH tunnel to stop.")
		return runtimeState, nil
	}
	s.logger.Debugf("Stopping SSH tunnel with PID %d", runtimeState.AWS.JumpHostSocks5TunnelPID)
	if runtimeState.AWS.JumpHostSocks5TunnelPID < 0 {
		// Skip if it is invalid
		runtimeState.AWS.JumpHostSocks5TunnelPID = 0 // Reset the PID if it's invalid
		return runtimeState, nil
	}

	input := steps.ShellUtilityInput{
		Command:         []string{"kill", fmt.Sprintf("%d", runtimeState.AWS.JumpHostSocks5TunnelPID)},
		Timeout:         10,   // Set a short timeout for killing the process
		SkipError:       true, // Skip error if the process is already dead
		RunInBackground: false,
	}
	_, _ = s.ShellUtility.Run(ctx, input)
	runtimeState.AWS.JumpHostSocks5TunnelPID = 0 // Reset the PID after stopping the tunnel
	return runtimeState, nil
}

func (s *StopSSHTunnelStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, prevStepError
}
