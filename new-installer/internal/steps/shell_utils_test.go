// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps_test

import (
	"context"
	"testing"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	"github.com/stretchr/testify/suite"
)

type ShellUtilityTest struct {
	suite.Suite
}

func TestShellUtility(t *testing.T) {
	suite.Run(t, new(ShellUtilityTest))
}

func (s *ShellUtilityTest) TestBasicCmd() {
	shellUtil := steps.CreateShellUtility()
	ctx := context.Background()
	output, err := shellUtil.Run(ctx, steps.ShellUtilityInput{
		Command:         []string{"echo", "Hello, World!"},
		Timeout:         5,
		SkipError:       false,
		RunInBackground: false,
	})

	if err != nil {
		s.NoError(err)
		return
	}
	s.Equal("Hello, World!\n", output.Stdout.String())
	s.Equal("", output.Stderr.String())
	s.Equal(nil, output.Error)
}

func (s *ShellUtilityTest) TestBasicCmdStderr() {
	shellUtil := steps.CreateShellUtility()
	ctx := context.Background()
	output, err := shellUtil.Run(ctx, steps.ShellUtilityInput{
		Command:         []string{"sh", "-c", "echo 'Hello, World!' >&2"},
		Timeout:         5,
		SkipError:       false,
		RunInBackground: false,
	})

	if err != nil {
		s.NoError(err)
		return
	}
	s.Equal("", output.Stdout.String())
	s.Equal("Hello, World!\n", output.Stderr.String())
	s.Equal(nil, output.Error)
}

func (s *ShellUtilityTest) TestBasicCmdError() {
	shellUtil := steps.CreateShellUtility()
	ctx := context.Background()
	output, err := shellUtil.Run(ctx, steps.ShellUtilityInput{
		Command:         []string{"sh", "-c", "exit 1"},
		Timeout:         5,
		SkipError:       false,
		RunInBackground: false,
	})

	if err == nil {
		s.Fail("Expected error, but got nil")
		return
	}
	s.Equal("", output.Stdout.String())
	s.Equal("", output.Stderr.String())
	s.Equal("exit status 1", err.Error())
	s.Equal("exit status 1", output.Error.Error())
}

func (s *ShellUtilityTest) TestBasicCmdSkipError() {
	shellUtil := steps.CreateShellUtility()
	ctx := context.Background()
	output, err := shellUtil.Run(ctx, steps.ShellUtilityInput{
		Command:         []string{"sh", "-c", "exit 1"},
		Timeout:         5,
		SkipError:       true,
		RunInBackground: false,
	})
	if err != nil {
		s.NoError(err)
		return
	}
	s.Equal("", output.Stdout.String())
	s.Equal("", output.Stderr.String())
	// The "Run" method should not return an error if SkipError is true
	// but the error should still be present in the output
	s.Equal("exit status 1", output.Error.Error())
}

func (s *ShellUtilityTest) TestBasicCmdTimeout() {
	shellUtil := steps.CreateShellUtility()
	ctx := context.Background()
	output, err := shellUtil.Run(ctx, steps.ShellUtilityInput{
		Command:         []string{"sh", "-c", "sleep 10"},
		Timeout:         1,
		SkipError:       false,
		RunInBackground: false,
	})

	if err == nil {
		s.Fail("Expected error, but got nil")
		return
	}
	s.Equal("", output.Stdout.String())
	s.Equal("", output.Stderr.String())
	s.Equal("failed to execute command: signal: killed", err.Error())
	s.Equal("signal: killed", output.Error.Error())
}

func (s *ShellUtilityTest) TestBasicCmdBackground() {
	shellUtil := steps.CreateShellUtility()
	ctx := context.Background()
	output, err := shellUtil.Run(ctx, steps.ShellUtilityInput{
		Command:         []string{"sh", "-c", "sleep 10"},
		Timeout:         1,
		SkipError:       false,
		RunInBackground: true,
	})

	if err != nil {
		s.NoError(err)
		return
	}
	waitErr := shellUtil.Wait()
	s.Equal("", output.Stdout.String())
	s.Equal("", output.Stderr.String())
	s.Equal(nil, output.Error)
	s.Equal("signal: killed", waitErr.Error())
}
