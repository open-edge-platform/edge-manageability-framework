package internal

import "context"

type OrchInstallerStepInterface interface {
	PreStep(ctx *context.Context) (OrchInstallerInputOutput, error)
	RunStep(ctx *context.Context) (OrchInstallerInputOutput, error)
	PostStep(ctx *context.Context) (OrchInstallerInputOutput, error)
}
