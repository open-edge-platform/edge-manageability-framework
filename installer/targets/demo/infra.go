package demo

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
)

const (
	InfraModulePath = "installer/targets/demo/iac/infra"
)

type DemoInfraStage struct {
	WorkingDir        string
	TerraformExecPath string
}

type DemoInfraStageVariables struct {
	VPCID string `yaml:"vpc_id"`
}

type DemoInfraStageRuntimeState struct {
	Variables     DemoInfraStageVariables `yaml:"variables"`
	BackendConfig AWSBackendConfig        `yaml:"backend_config"`

	// Inherit from previous stage
	Action string `yaml:"action" validate:"required,oneof=install upgrade uninstall"`
	LogDir string `yaml:"log_path"`
}

func (s *DemoInfraStage) Name() string {
	return "DemoInfraStage"
}

func (s *DemoInfraStage) PreStage(ctx context.Context, installerInput internal.OrchInstallerInput, prevStageOutput internal.RuntimeState) (internal.RuntimeState, *internal.OrchInstallerError) {
	infraNetRuntimeState, ok := prevStageOutput.(*DemoInfraNetworkStageRuntimeState)
	if !ok {
		return nil, &internal.OrchInstallerError{
			ErrorCode:  internal.OrchInstallerErrorCodeInternal,
			ErrorStage: s.Name(),
			ErrorStep:  "DemoInfraNetworkStage",
			ErrorMsg:   "failed to cast previous stage output to DemoInfraNetworkStageRuntimeState",
		}
	}

	bucketName := fmt.Sprintf("%s-%s", installerInput.DeploymentName, installerInput.StateStoreBucketPostfix)

	variables := DemoInfraStageVariables{
		VPCID: infraNetRuntimeState.VPCID,
	}
	backendConfig := AWSBackendConfig{
		Bucket: bucketName,
		Key:    "infra",
		Region: installerInput.Region,
	}

	return &DemoInfraStageRuntimeState{
		Action:        infraNetRuntimeState.Action,
		Variables:     variables,
		BackendConfig: backendConfig,
		LogDir:        infraNetRuntimeState.LogDir,
	}, nil
}

func (s *DemoInfraStage) RunStage(ctx context.Context, installerInput internal.OrchInstallerInput, prevStageOutput internal.RuntimeState) (internal.RuntimeState, *internal.OrchInstallerError) {
	prevOutput, ok := prevStageOutput.(*DemoInfraStageRuntimeState)
	if !ok {
		return nil, &internal.OrchInstallerError{
			ErrorCode:  internal.OrchInstallerErrorCodeInternal,
			ErrorStage: s.Name(),
			ErrorStep:  "RunStage",
			ErrorMsg:   "failed to cast previous stage output to DemoInfraStageRuntimeState",
		}
	}
	terraformStep := &steps.OrchInstallerTerraformStep{
		Action:        prevOutput.Action,
		ExecPath:      s.TerraformExecPath,
		ModulePath:    filepath.Join(s.WorkingDir, InfraModulePath),
		Variables:     prevOutput.Variables,
		BackendConfig: prevOutput.BackendConfig,
		LogFile:       filepath.Join(prevOutput.LogDir, "stage-infra-terraform.log"),
	}

	// TODO: Collect outputs from steps
	_, err := terraformStep.Run(ctx)
	return nil, err
}

func (s *DemoInfraStage) PostStage(ctx context.Context, installerInput internal.OrchInstallerInput, prevStageOutput internal.RuntimeState, prevStageError *internal.OrchInstallerError) (internal.RuntimeState, *internal.OrchInstallerError) {
	return prevStageOutput, prevStageError
}
