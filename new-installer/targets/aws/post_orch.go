package aws

import (
	"context"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
)

type PostOrchStage struct {
	// The root path of the edge-managability-framework repo
	// This is used to find the terraform files and right log path
	RootPath string
	// Keeps the generated files such as Terraform variables and backend config.
	KeepGeneratedFiles bool

	steps []steps.OrchInstallerStep
}

func NewPostOrchStage(rootPath string, keepGeneratedFiles bool) *PostOrchStage {
	return &PostOrchStage{
		RootPath:           rootPath,
		KeepGeneratedFiles: keepGeneratedFiles,
		steps:              []steps.OrchInstallerStep{},
	}
}

func (a *PostOrchStage) Name() string {
	return "PostOrchStage"
}
func (a *PostOrchStage) PreStage(ctx context.Context, config internal.OrchInstallerConfig, runtimeState *internal.OrchInstallerRuntimeState) *internal.OrchInstallerError {
	for _, step := range a.steps {

		if newRuntimeState, err := step.ConfigStep(ctx, config, *runtimeState); err != nil {
			return err
		} else if err = runtimeState.UpdateRuntimeState(newRuntimeState); err != nil {
			return err
		}

		if newRuntimeState, err := step.PreSetp(ctx, config, *runtimeState); err != nil {
			return err
		} else if err = runtimeState.UpdateRuntimeState(newRuntimeState); err != nil {
			return err
		}
	}
	return nil
}

func (a *PostOrchStage) RunStage(ctx context.Context, config internal.OrchInstallerConfig, runtimeState *internal.OrchInstallerRuntimeState) *internal.OrchInstallerError {
	for _, step := range a.steps {
		if newRuntimeState, err := step.RunStep(ctx, config, *runtimeState); err != nil {
			return err
		} else if err = runtimeState.UpdateRuntimeState(newRuntimeState); err != nil {
			return err
		}
	}
	return nil
}

func (a *PostOrchStage) PostStage(ctx context.Context, config internal.OrchInstallerConfig, runtimeState *internal.OrchInstallerRuntimeState, prevStageError *internal.OrchInstallerError) *internal.OrchInstallerError {
	for _, step := range a.steps {
		if newRuntimeState, err := step.PostStep(ctx, config, *runtimeState, prevStageError); err != nil {
			return err
		} else if err = runtimeState.UpdateRuntimeState(newRuntimeState); err != nil {
			return err
		}
	}
	return nil
}
