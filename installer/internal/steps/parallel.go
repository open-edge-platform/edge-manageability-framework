package steps

import (
	"context"

	. "github.com/open-edge-platform/edge-manageability-framework/installer/internal"
)

type OrchInstallerStepParallel struct {
	Steps []OrchInstallerStepInterface
}

func (s *OrchInstallerStepParallel) PreStep(ctx *context.Context) (OrchInstallerInputOutput, error) {
	// PreStep logic here
	return nil, nil
}

func (s *OrchInstallerStepParallel) PostStep(ctx *context.Context) (OrchInstallerInputOutput, error) {
	// PostStep logic here
	return nil, nil
}

func (s *OrchInstallerStepParallel) DoStep(ctx *context.Context, step OrchInstallerStepInterface, ch chan<- error) {
	// TODO: Error handling
	// TODO: Collect outputs
	_, err := step.PreStep(ctx)
	if err != nil {
		ch <- err
		return
	}
	_, err = step.RunStep(ctx)
	if err != nil {
		ch <- err
		return
	}
	_, err = step.PostStep(ctx)
	if err != nil {
		ch <- err
		return
	}

	ch <- nil // TODO: Add complete step output
}

func (s *OrchInstallerStepParallel) RunStep(ctx *context.Context) error {
	ch := make(chan error)
	for _, step := range s.Steps {
		go s.DoStep(ctx, step, ch)
	}
	// Wait for all steps to complete
	// TODO: Add timeout handling
	for range s.Steps {
		if err := <-ch; err != nil {
			return err
		}
	}
	return nil
}
