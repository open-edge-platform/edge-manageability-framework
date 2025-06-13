// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps_common

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	"go.uber.org/zap"
)

const sshTunnelConfigTemplate = `Host jumphost
	HostName %s
	IdentityFile %s
	User ubuntu
	StrictHostKeyChecking no
	BatchMode yes
	UserKnownHostsFile /dev/null
	ExitOnForwardFailure yes
	ServerAliveCountMax 100
	ServerAliveInterval 20`

type SSHTunnelStep struct {
	ShellUtility steps.ShellUtility
	logger       *zap.SugaredLogger
}

var sshTunnelStepLabels = []string{"common", "ssh-tunnel"}

func CreateSSHTunnelStep(shellUtility steps.ShellUtility) *SSHTunnelStep {
	return &SSHTunnelStep{
		ShellUtility: shellUtility,
		logger:       internal.Logger(),
	}
}

func (s *SSHTunnelStep) Name() string {
	return "SSHTunnelStep"
}

func (s *SSHTunnelStep) Labels() []string {
	return sshTunnelStepLabels
}

func (s *SSHTunnelStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

func (s *SSHTunnelStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if s.ShellUtility == nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  "Shell utility is not initialized.",
		}
	}
	if runtimeState.AWS.JumpHostIP == "" || runtimeState.AWS.JumpHostSSHKeyPrivateKey == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "Jump host IP and SSH private key must be provided in the runtime state.",
		}
	}
	return runtimeState, nil
}

func (s *SSHTunnelStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	s.logger.Info("Establishing SSH tunnel...")
	privateKeyFile, err := os.CreateTemp("", "jumphost-key-*.pem")
	defer os.Remove(privateKeyFile.Name()) // Clean up the temporary file after use
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  "failed to create temporary private key file: " + err.Error(),
		}
	}
	defer privateKeyFile.Close()
	if _, err := privateKeyFile.WriteString(runtimeState.AWS.JumpHostSSHKeyPrivateKey); err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  "failed to write private key to temporary file: " + err.Error(),
		}
	}
	if err := os.Chmod(privateKeyFile.Name(), 0o400); err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  "failed to set permissions on private key file: " + err.Error(),
		}
	}
	sshConfig := fmt.Sprintf(sshTunnelConfigTemplate, runtimeState.AWS.JumpHostIP, privateKeyFile.Name())
	if socksProxy := os.Getenv("SOCKS_PROXY"); socksProxy != "" {
		sshConfig += fmt.Sprintf("\n\tProxyCommand nc -x %s %%h %%p\n", socksProxy)
	}
	sshConfigFile, err := os.CreateTemp("", "ssh-config-*.txt")
	defer os.Remove(sshConfigFile.Name()) // Clean up the temporary file after tunnel is started
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  "failed to create temporary SSH config file: " + err.Error(),
		}
	}
	if err := os.Chmod(sshConfigFile.Name(), 0o400); err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  "failed to set permissions on SSH config file: " + err.Error(),
		}
	}
	if _, err := sshConfigFile.WriteString(sshConfig); err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  "failed to write SSH config to temporary file: " + err.Error(),
		}
	}
	// Find an open port for the SSH tunnel
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  "failed to find an open port for SSH tunnel: " + err.Error(),
		}
	}
	listener.Close()
	socksPort := listener.Addr().(*net.TCPAddr).Port
	input := steps.ShellUtilityInput{
		Command: []string{
			"ssh",
			"-f",                               // Run in the background
			"-N",                               // Do not execute any commands
			"-n",                               // Redirect stdin to /dev/null
			"-D", fmt.Sprintf("%d", socksPort), // Set up a SOCKS proxy on the specified port
			"-F", sshConfigFile.Name(), // Use the temporary SSH config file
			"jumphost", // The alias defined in the SSH config
		},
	}
	_, tunnelErr := s.ShellUtility.Run(ctx, input)
	if tunnelErr != nil {
		return runtimeState, tunnelErr
	}
	runtimeState.AWS.JumpHostSocks5TunnelPort = socksPort
	runtimeState.AWS.JumpHostSocks5TunnelPID = s.ShellUtility.Process().Pid
	s.logger.Infof("SSH socks5 tunnel established on port %d", socksPort)
	return runtimeState, nil
}

func (s *SSHTunnelStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, prevStepError
}
