package internal

import "context"

type OrchInstaller struct {
	ctx    *context.Context
	Stages []OrchInstallerStageInterface
	Input  OrchInstallerInputOutput
	Output OrchInstallerInputOutput
}

func CreateOrchInstaller() OrchInstaller {
	return OrchInstaller{}
}

func (o *OrchInstaller) Run() error {
	// TODO: Add error handling and logging
	// TODO: collect outputs from stages
	for _, stage := range o.Stages {
		_, err := stage.PreStage(o.ctx, o.Input)
		if err != nil {
			return err
		}
		_, err = stage.RunStage(o.ctx, o.Input)
		if err != nil {
			return err
		}
		_, err = stage.PostStage(o.ctx, o.Input)
		if err != nil {
			return err
		}
	}
	return nil
}
