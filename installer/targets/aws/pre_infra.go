package aws

import (
	"context"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
)

type PreInfraStage struct {
	// The root path of the edge-managability-framework repo
	// This is used to find the terraform files and right log path
	RootPath string
	// Keeps the generated files such as Terraform variables and backend config.
	KeepGeneratedFiles bool
}

func (a *PreInfraStage) Name() string {
	return "PreInfraStage"
}
func (a *PreInfraStage) PreStage(ctx context.Context, config internal.OrchInstallerConfig, runtimeState *internal.OrchInstallerRuntimeState) *internal.OrchInstallerError {
	createAWSStateBucketStep := &steps.CreateAWSStateBucket{}
	err := func() *internal.OrchInstallerError {
		if newRuntimeState, err := createAWSStateBucketStep.ConfigStep(ctx, config, *runtimeState); err != nil {
			return err
		} else if err = runtimeState.UpdateRuntimeState(newRuntimeState); err != nil {
			return err
		}

		if newRuntimeState, err := createAWSStateBucketStep.PreSetp(ctx, config, *runtimeState); err != nil {
			return err
		} else if err = runtimeState.UpdateRuntimeState(newRuntimeState); err != nil {
			return err
		}
		if newRuntimeState, err := createAWSStateBucketStep.RunStep(ctx, config, *runtimeState); err != nil {
			return err
		} else if err = runtimeState.UpdateRuntimeState(newRuntimeState); err != nil {
			return err
		}
		return nil
	}()
	if newRuntimeState, err := createAWSStateBucketStep.PostStep(ctx, config, *runtimeState, err); err != nil {
		return err
	} else {
		return runtimeState.UpdateRuntimeState(newRuntimeState)
	}
}

func (a *PreInfraStage) RunStage(ctx context.Context, config internal.OrchInstallerConfig, runtimeState *internal.OrchInstallerRuntimeState) *internal.OrchInstallerError {
	vpcStep := &steps.AWSVPCStep{
		RootPath:           a.RootPath,
		KeepGeneratedFiles: a.KeepGeneratedFiles,
	}
	err := func() *internal.OrchInstallerError {
		if newRuntimeState, err := vpcStep.ConfigStep(ctx, config, *runtimeState); err != nil {
			return err
		} else if err = runtimeState.UpdateRuntimeState(newRuntimeState); err != nil {
			return err
		}

		if newRuntimeState, err := vpcStep.PreSetp(ctx, config, *runtimeState); err != nil {
			return err
		} else if err = runtimeState.UpdateRuntimeState(newRuntimeState); err != nil {
			return err
		}

		if newRuntimeState, err := vpcStep.RunStep(ctx, config, *runtimeState); err != nil {
			return err
		} else if err = runtimeState.UpdateRuntimeState(newRuntimeState); err != nil {
			return err
		}
		return nil
	}()
	if newRuntimeState, err := vpcStep.PostStep(ctx, config, *runtimeState, err); err != nil {
		return err
	} else {
		return runtimeState.UpdateRuntimeState(newRuntimeState)
	}
}

func (a *PreInfraStage) PostStage(ctx context.Context, config internal.OrchInstallerConfig, runtimeState *internal.OrchInstallerRuntimeState, prevStageError *internal.OrchInstallerError) *internal.OrchInstallerError {
	return prevStageError
}
