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

type ShellUtility struct {
	Command          []string
	Timeout          int
	SkipError        bool
	RunInBackeground bool

	cmd *exec.Cmd
}

type ShellUtilityOutput struct {
	Stdout strings.Builder
	Stderr strings.Builder
	Error  error
}

func (s *ShellUtility) Run(ctx context.Context) (*ShellUtilityOutput, *internal.OrchInstallerError) {
	logger := internal.Logger()
	logger.Debugf("Running shell command: %s", s.Command)
	if s.Timeout <= 0 {
		s.Timeout = DefaultTimeout
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(s.Timeout)*time.Second)
	defer cancel()

	s.cmd = exec.CommandContext(timeoutCtx, s.Command[0], s.Command[1:]...)

	stderrWriter := strings.Builder{}
	stdoutWriter := strings.Builder{}

	s.cmd.Stdout = &stdoutWriter
	s.cmd.Stderr = &stderrWriter
	var err error
	if s.RunInBackeground {
		err = s.cmd.Start()
	} else {
		err = s.cmd.Run()
	}

	output := &ShellUtilityOutput{
		Stdout: stdoutWriter,
		Stderr: stderrWriter,
		Error:  err,
	}

	if err != nil && !s.SkipError {
		return output, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to execute command: %v", err),
		}
	}
	return output, nil
}

func (s *ShellUtility) Process() *os.Process {
	if s.cmd == nil {
		return nil
	}
	return s.cmd.Process
}

func (s *ShellUtility) Kill() error {
	if s.cmd == nil {
		return nil
	}
	if s.cmd.Process == nil {
		return nil
	}
	return s.cmd.Process.Kill()
}

func (s *ShellUtility) Wait() error {
	if s.cmd == nil {
		return nil
	}
	if s.cmd.Process == nil {
		return nil
	}
	return s.cmd.Wait()
}
