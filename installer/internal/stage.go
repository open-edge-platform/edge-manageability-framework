package internal

import "context"

type OrchInstallerStageInterface interface {
	PreStage(ctx *context.Context, input OrchInstallerInputOutput) (OrchInstallerInputOutput, error)
	RunStage(ctx *context.Context, input OrchInstallerInputOutput) (OrchInstallerInputOutput, error)
	PostStage(ctx *context.Context, input OrchInstallerInputOutput) (OrchInstallerInputOutput, error)
}
