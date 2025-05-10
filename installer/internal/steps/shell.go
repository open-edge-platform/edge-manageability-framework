package steps

import (
	"context"
	"fmt"

	"github.com/bitfield/script"
	. "github.com/open-edge-platform/edge-manageability-framework/installer/internal"
)

type OrchInstallerShellStep struct {
	// Command to be executed
	Command string
	// Timeout for the command
	Timeout int
	// Output of the command
	Output string
}

func (s *OrchInstallerShellStep) PreStep(ctx *context.Context) (OrchInstallerInputOutput, error) {
	// PreStep logic here
	return nil, nil
}
func (s *OrchInstallerShellStep) RunStep(ctx *context.Context) (OrchInstallerInputOutput, error) {
	// RunStep logic here
	// TODO: add timeout
	// TODO: Store errors so we can handle it later in post step?
	fmt.Printf("Running command: %s\n", s.Command)
	var err error
	s.Output, err = script.Exec(s.Command).String()
	return nil, err
}

func (s *OrchInstallerShellStep) PostStep(ctx *context.Context) (OrchInstallerInputOutput, error) {
	// PostStep logic here
	return nil, nil
}
