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

	variables     DemoInfraStageVariables
	backendConfig AWSBackendConfig
}

type DemoInfraStageVariables struct {
	VPCID string `yaml:"vpc_id"`
}

func (s *DemoInfraStage) Name() string {
	return "DemoInfraStage"
}

func (s *DemoInfraStage) PreStage(ctx context.Context, config internal.OrchInstallerConfig, runtimeState *internal.OrchInstallerRuntimeState) *internal.OrchInstallerError {
	s.variables = DemoInfraStageVariables{
		VPCID: runtimeState.VPCID,
	}
	s.backendConfig = AWSBackendConfig{
		Bucket: fmt.Sprintf("%s-%s", config.DeploymentName, config.StateStoreBucketPostfix),
		Key:    "infra",
		Region: config.Region,
	}
	return nil
}

func (s *DemoInfraStage) RunStage(ctx context.Context, config internal.OrchInstallerConfig, runtimeState *internal.OrchInstallerRuntimeState) *internal.OrchInstallerError {
	terraformStep := &steps.OrchInstallerTerraformStep{
		Action:        runtimeState.Action,
		ExecPath:      s.TerraformExecPath,
		ModulePath:    filepath.Join(s.WorkingDir, InfraModulePath),
		Variables:     s.variables,
		BackendConfig: s.backendConfig,
		LogFile:       filepath.Join(runtimeState.LogDir, "stage-infra-terraform.log"),
	}
	// TODO: Collect outputs from steps
	_, err := terraformStep.Run(ctx)
	return err
}

func (s *DemoInfraStage) PostStage(ctx context.Context,
	config internal.OrchInstallerConfig,
	runtimeState *internal.OrchInstallerRuntimeState,
	prevStageError *internal.OrchInstallerError) *internal.OrchInstallerError {
	return prevStageError
}
