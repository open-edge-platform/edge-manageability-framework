package internal

import (
	"context"
	"fmt"
	"sync"
)

type OrchInstaller struct {
	Stages []OrchInstallerStage

	mutex     *sync.Mutex
	cancelled bool
}

// The top level installer input defined here
// This must be as general as possible
// Note that we are using
type OrchInstallerConfig struct {
	Version string `yaml:"version"`
	// The target environment that will be used
	TargetEnvironment string `yaml:"target_environment" validate:"required,oneof=aws azure on-prem demo"`
	// The DeploymentName that will be shared across all the stages.
	// Including but not limited to:
	// - Backend storage name(S3 bucket, Azure storage, dir, ...)
	// - VPC/VPN
	// - EKS/AKS/RKE2 cluster name
	// - Database name
	// ...
	DeploymentName string   `yaml:"deployment_name" validate:"required"`
	NetworkCIDR    string   `yaml:"network_cidr"`
	SubnetCIDRs    []string `yaml:"subnet_cidrs"`

	// Cloud deployment specific fields
	Region                  string   `yaml:"region"`
	AvailabilityZones       []string `yaml:"availability_zones"`
	StateStoreBucketPostfix string   `yaml:"state_store_bucket_postfix"`
}

// The data that will pass to the first stage
type OrchInstallerRuntimeState struct {
	// The Action that will be performed
	// This can be one of the following:
	// - install
	// - upgrade
	// - uninstall
	Action string `yaml:"action" validate:"required,oneof=install upgrade uninstall"`
	// The directory where the logs will be saved
	LogDir string `yaml:"log_path"`
	DryRun bool   `yaml:"dry_run"`
}

func CreateOrchInstaller(stages []OrchInstallerStage) (*OrchInstaller, error) {
	return &OrchInstaller{
		Stages:    stages,
		mutex:     &sync.Mutex{},
		cancelled: false,
	}, nil
}

func reverseStages(stages []OrchInstallerStage) []OrchInstallerStage {
	reversed := []OrchInstallerStage{}
	for i := len(stages) - 1; i >= 0; i-- {
		reversed[i] = stages[i]
	}
	return reversed
}

func (o *OrchInstaller) Run(ctx context.Context, action string, input OrchInstallerConfig, logDir string) (RuntimeState, *OrchInstallerError) {
	logger := Logger()
	// TODO: Add error handling and logging
	// TODO: collect runtimeStates from stages
	// TODO: Handle final runtimeState?

	if action == "" {
		return nil, &OrchInstallerError{
			ErrorCode: OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "action must be specified",
		}
	}

	var runtimeState RuntimeState = &OrchInstallerRuntimeState{
		Action: action,
		LogDir: logDir,
	}

	if action != "install" && action != "upgrade" && action != "uninstall" {
		return nil, &OrchInstallerError{
			ErrorCode: OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  fmt.Sprintf("unsupported action: %s", action),
		}
	}
	if action == "uninstall" {
		o.Stages = reverseStages(o.Stages)
	}
	for _, stage := range o.Stages {
		var err *OrchInstallerError
		if o.Cancelled() {
			logger.Info("Installation cancelled")
			break
		}
		name := stage.Name()
		logger.Infof("Running stage: %s", name)
		runtimeState, err = stage.PreStage(ctx, input, runtimeState)

		// We will skip to run the stage if the previous stage failed
		if err == nil {
			runtimeState, err = stage.RunStage(ctx, input, runtimeState)
		}

		// But we will always run the post stage, the post stage should
		// handle the error and rollback if needed.
		runtimeState, err = stage.PostStage(ctx, input, runtimeState, err)
		if err != nil {
			err.ErrorStage = name
			return runtimeState, err
		}
	}
	return runtimeState, nil
}

func (o *OrchInstaller) CancelInstallation() {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	o.cancelled = true
}

func (o *OrchInstaller) Cancelled() bool {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	return o.cancelled
}
