// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
)

const DefaultTimeout = 60 // seconds

type ShellUtility interface {
	Run(ctx context.Context, input ShellUtilityInput) (*ShellUtilityOutput, *internal.OrchInstallerError)
	Process() *os.Process
	Kill() error
	Wait() error
}

type shellUtilityImpl struct {
	cmd *exec.Cmd
}

type ShellUtilityInput struct {
	Command         []string
	Timeout         int
	SkipError       bool
	RunInBackground bool
}

type ShellUtilityOutput struct {
	Stdout strings.Builder
	Stderr strings.Builder
	Error  error
}

func CreateShellUtility() ShellUtility {
	return &shellUtilityImpl{}
}

func (s *shellUtilityImpl) Run(ctx context.Context, input ShellUtilityInput) (*ShellUtilityOutput, *internal.OrchInstallerError) {
	logger := internal.Logger()
	logger.Debugf("Running shell command: %s", input.Command)
	if input.Timeout <= 0 {
		input.Timeout = DefaultTimeout
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(input.Timeout)*time.Second)
	defer cancel()

	s.cmd = exec.CommandContext(timeoutCtx, input.Command[0], input.Command[1:]...)

	stderrWriter := strings.Builder{}
	stdoutWriter := strings.Builder{}

	s.cmd.Stdout = &stdoutWriter
	s.cmd.Stderr = &stderrWriter
	var err error
	if input.RunInBackground {
		err = s.cmd.Start()
	} else {
		err = s.cmd.Run()
	}

	output := &ShellUtilityOutput{
		Stdout: stdoutWriter,
		Stderr: stderrWriter,
		Error:  err,
	}

	if err != nil && !input.SkipError {
		return output, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to execute command: %v", err),
		}
	}
	return output, nil
}

func (s *shellUtilityImpl) Process() *os.Process {
	if s.cmd == nil {
		return nil
	}
	return s.cmd.Process
}

func (s *shellUtilityImpl) Kill() error {
	if s.cmd == nil {
		return nil
	}
	if s.cmd.Process == nil {
		return nil
	}
	return s.cmd.Process.Kill()
}

func (s *shellUtilityImpl) Wait() error {
	if s.cmd == nil {
		return nil
	}
	if s.cmd.Process == nil {
		return nil
	}
	return s.cmd.Wait()
}
