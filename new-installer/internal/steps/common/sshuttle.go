// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps_common

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	steps_aws "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/aws"
)

type SshuttleStep struct {
	ShellUtility steps.ShellUtility
}

var sshuttleStepLabels = []string{"common", "sshuttle"}

func (s *SshuttleStep) Name() string {
	return "SshuttleStep"
}

func (s *SshuttleStep) Labels() []string {
	return sshuttleStepLabels
}

func (s *SshuttleStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

func (s *SshuttleStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if s.ShellUtility == nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  "Shell utility is not initialized.",
		}
	}
	if !commandExists(ctx, s.ShellUtility, "sudo") {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  "sudo command is not available. Please install sudo.",
		}
	}
	if !commandExists(ctx, s.ShellUtility, "sshuttle") {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  "sshuttle command is not available. Please install sshuttle.",
		}
	}
	if !commandExists(ctx, s.ShellUtility, "nc") {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  "nc command is not available. Please install sshuttle.",
		}
	}
	pgrepOut, _ := s.ShellUtility.Run(ctx, steps.ShellUtilityInput{
		Command:         []string{"pgrep", "-x", "sshuttle"},
		Timeout:         10,
		SkipError:       true,
		RunInBackground: false,
	})
	sshuttlePID := strings.TrimSpace(pgrepOut.Stdout.String())
	if sshuttlePID != "" {
		_, err := s.ShellUtility.Run(ctx, steps.ShellUtilityInput{
			Command:         []string{"sudo", "kill", sshuttlePID},
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
	}
	if runtimeState.AWS.JumpHostSSHKeyPrivateKey == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  "Jump host SSH private key is not set in the runtime state.",
		}
	}
	return runtimeState, nil
}

func (s *SshuttleStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	// Create a temporary file for the private key
	privateKeyFile, err := os.CreateTemp("", "jumphost-key-*.pem")
	defer os.Remove(privateKeyFile.Name()) // Clean up the temporary file after sshuttle started
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to create temporary private key file: %v", err),
		}
	}
	if _, err := privateKeyFile.WriteString(runtimeState.AWS.JumpHostSSHKeyPrivateKey); err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to write private key to temporary file: %v", err),
		}
	}
	if err := privateKeyFile.Close(); err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to close temporary private key file: %v", err),
		}
	}
	// Set the file permissions to read for the owner only
	if err := os.Chmod(privateKeyFile.Name(), 0o400); err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to set permissions on temporary private key file: %v", err),
		}
	}
	pidFile, err := os.CreateTemp("", "sshuttle-pid-*.txt")
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to create temporary PID file: %v", err),
		}
	}
	err = pidFile.Close() // Close the file, we will use it later
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to close temporary PID file: %v", err),
		}
	}

	if config.Proxy.SOCKSProxy != "" {
		_, err := s.ShellUtility.Run(ctx, steps.ShellUtilityInput{
			Command: []string{
				"sshuttle",
				"--pidfile",
				pidFile.Name(),
				"-D",
				"-e",
				fmt.Sprintf(
					"ssh -o ProxyCommand='nc -x %s %%h %%p' -i %s -o StrictHostKeyChecking=no",
					config.Proxy.SOCKSProxy, privateKeyFile.Name(),
				),
				"-r",
				fmt.Sprintf("ubuntu@%s", runtimeState.AWS.JumpHostIP),
				steps_aws.DefaultNetworkCIDR,
			},
			Timeout:         60,
			SkipError:       false,
			RunInBackground: false, // We use -D flag to run in the background
		})
		if err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorMsg:  fmt.Sprintf("failed to start sshuttle command proxy with socks proxy: %v", err),
			}
		}
	} else {
		_, err := s.ShellUtility.Run(ctx, steps.ShellUtilityInput{
			Command: []string{
				"sshuttle",
				"--pidfile",
				pidFile.Name(),
				"-D",
				"-r",
				fmt.Sprintf("ubuntu@%s", runtimeState.AWS.JumpHostIP),
				"--ssh-cmd",
				fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no", privateKeyFile.Name()),
				steps_aws.DefaultNetworkCIDR,
			},
			Timeout:         60,
			SkipError:       false,
			RunInBackground: false, // We use -D flag to run in the background
		})
		if err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorMsg:  fmt.Sprintf("failed to start sshuttle command proxy: %v", err),
			}
		}
	}

	time.Sleep(5 * time.Second) // Wait for sshuttle to establish the connection
	// Print the PID of the sshuttle process
	pid, err := os.ReadFile(pidFile.Name())
	if err != nil {
		log.Printf("Failed to read sshuttle PID file: %v", err)
	} else {
		log.Printf("sshuttle is running with PID: %s", strings.TrimSpace(string(pid)))
		runtimeState.SshuttlePID = strings.TrimSpace(string(pid))
	}
	return runtimeState, nil
}

func (s *SshuttleStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

func StopSshuttle() error {
	// Read the PID from the sshuttle PID file
	pidFile := "/tmp/sshuttle.pid"
	pidData, err := os.ReadFile(pidFile)
	if err != nil {
		// Failed to read it, assume the process is not running
		return nil
	}

	// Convert the PID to an integer
	pid := strings.TrimSpace(string(pidData))
	cmd := exec.Command("kill", pid)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to terminate sshuttle process with PID %s: %w", pid, err)
	}

	log.Println("Stopped sshuttle successfully")
	return nil
}
