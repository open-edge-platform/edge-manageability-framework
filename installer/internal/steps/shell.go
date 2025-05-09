package steps

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
)

type OrchInstallerShellStep struct {
	Command          string
	Timeout          int
	SkipError        bool
	RunInBackeground bool

	cmd *exec.Cmd
}

type OrchInstallerShellStepOutput struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Error  error  `json:"error"`
}

func (s *OrchInstallerShellStep) Run(ctx *context.Context) (*OrchInstallerShellStepOutput, *internal.OrchInstallerError) {
	// RunStep logic here
	// TODO: add timeout
	// TODO: Store *internal.OrchInstallerErrors so we can handle it later in post step?
	logger := internal.Logger()
	logger.Debugf("Running shell command: %s", s.Command)

	s.cmd = exec.CommandContext(*ctx, "sh", "-c", s.Command)

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

	output := &OrchInstallerShellStepOutput{
		Stdout: stdoutWriter.String(),
		Stderr: stderrWriter.String(),
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

func (s *OrchInstallerShellStep) Process() *os.Process {
	if s.cmd == nil {
		return nil
	}
	return s.cmd.Process
}

func (s *OrchInstallerShellStep) Kill() error {
	if s.cmd == nil {
		return nil
	}
	if s.cmd.Process == nil {
		return nil
	}
	return s.cmd.Process.Kill()
}

func (s *OrchInstallerShellStep) Wait() error {
	if s.cmd == nil {
		return nil
	}
	if s.cmd.Process == nil {
		return nil
	}
	return s.cmd.Wait()
}
