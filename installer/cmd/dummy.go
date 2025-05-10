package main

import (
	"bytes"
	"context"
	"fmt"

	. "github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	. "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	"github.com/spf13/viper"
)

type OrchInstallerDummyBase struct {
	Steps  []OrchInstallerStepInterface
	Output OrchInstallerInputOutput
}

type OrchInstallerDummyStage1 struct {
	OrchInstallerDummyBase
}

type OrchInstallerDummyStage1Input struct {
	Name string
}

func (i *OrchInstallerDummyStage1Input) SerializeToYAML() ([]byte, error) {
	if err := i.Validate(); err != nil {
		return nil, err
	}
	input := viper.New()
	input.Set("name", i.Name)
	writer := &bytes.Buffer{}
	if err := input.WriteConfigTo(writer); err != nil {
		return nil, err
	}
	return writer.Bytes(), nil
}

func (i *OrchInstallerDummyStage1Input) DeserializeFromYAML(data []byte) error {
	input := viper.New()
	if err := input.ReadConfig(bytes.NewReader(data)); err != nil {
		return err
	}
	if !input.IsSet("name") {
		return &OrchInstallerError{ErrorMsg: "name is not set"}
	}
	i.Name = input.GetString("name")
	return nil
}

func (i *OrchInstallerDummyStage1Input) Validate() error {
	if i.Name == "" {
		return &OrchInstallerError{ErrorMsg: "name is not set"}
	}
	return nil
}

type OrchInstallerDummyStage1Output struct {
}

func (o *OrchInstallerDummyStage1Output) SerializeToYAML() ([]byte, error) {
	if err := o.Validate(); err != nil {
		return nil, err
	}
	output := viper.New()
	writer := &bytes.Buffer{}
	if err := output.WriteConfigTo(writer); err != nil {
		return nil, err
	}
	return writer.Bytes(), nil
}
func (o *OrchInstallerDummyStage1Output) DeserializeFromYAML(data []byte) error {
	output := viper.New()
	if err := output.ReadConfig(bytes.NewReader(data)); err != nil {
		return err
	}
	return nil
}
func (o *OrchInstallerDummyStage1Output) Validate() error {
	return nil
}

func CreateDummyStages(input OrchInstallerInputOutput) []OrchInstallerStageInterface {
	if i, ok := (input).(*OrchInstallerDummyStage1Input); ok {
		return []OrchInstallerStageInterface{
			&OrchInstallerDummyStage1{
				OrchInstallerDummyBase: OrchInstallerDummyBase{
					Steps: []OrchInstallerStepInterface{
						&OrchInstallerShellStep{
							Command: fmt.Sprintf("echo 'hello %s'", i.Name),
						},
						&OrchInstallerShellStep{
							Command: fmt.Sprintf("echo 'hello %s 2'", i.Name),
						},
					},
				},
			},
		}
	} else {
		return nil
	}

}

func (s *OrchInstallerDummyStage1) PreStage(ctx *context.Context, input OrchInstallerInputOutput) (OrchInstallerInputOutput, error) {
	return nil, nil
}

func (s *OrchInstallerDummyStage1) PostStage(ctx *context.Context, input OrchInstallerInputOutput) (OrchInstallerInputOutput, error) {
	// PostStage logic here
	return nil, nil
}
func (s *OrchInstallerDummyStage1) RunStage(ctx *context.Context, input OrchInstallerInputOutput) (OrchInstallerInputOutput, error) {
	fmt.Println("Running stage 1")
	// TODO: Error handling
	// TODO: Collect steps output and merge them to stage output
	for _, step := range s.Steps {
		_, err := step.PreStep(ctx)
		if err != nil {
			return nil, err
		}
		_, err = step.RunStep(ctx)
		if err != nil {
			return nil, err
		}
		_, err = step.PostStep(ctx)
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}
