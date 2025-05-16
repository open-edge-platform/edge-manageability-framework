// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"fmt"
	"sync"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

const (
	OrchConfigVersion   = "0.0.1-dev"
	RuntimeStateVersion = "0.0.1-dev"
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
	// Schema version of the config
	ConfigVersion string `yaml:"config_version"`
	OrchVersion   string `yaml:"orch_version"`
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
	mutex *sync.Mutex
	// Schema version of the runtime state
	RuntimeStateVersion string `yaml:"config_version"`
	OrchVersion         string `yaml:"orch_version"`
	// The Action that will be performed
	// This can be one of the following:
	// - install
	// - upgrade
	// - uninstall
	Action string `yaml:"action" validate:"required,oneof=install upgrade uninstall"`
	// The directory where the logs will be saved
	LogDir string `yaml:"log_dir"`
	DryRun bool   `yaml:"dry_run"`

	// Infra-specific runtime state
	VPCID string `yaml:"vpc_id" validate:"required"`
}

func (rs *OrchInstallerRuntimeState) UpdateRuntimeState(source OrchInstallerRuntimeState) *OrchInstallerError {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()
	srcK := koanf.New(".")
	srcK.Load(structs.Provider(source, "yaml"), nil)
	dstK := koanf.New(".")
	dstK.Load(structs.Provider(rs, "yaml"), nil)
	dstK.Merge(srcK)

	dstData, err := dstK.Marshal(yaml.Parser())
	if err != nil {
		return &OrchInstallerError{
			ErrorCode: OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to marshal runtime state: %v", err),
		}
	}

	err = DeserializeFromYAML(rs, dstData)
	if err != nil {
		return &OrchInstallerError{
			ErrorCode: OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to unmarshal runtime state: %v", err),
		}
	}
	return nil
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
		reversed = append(reversed, stages[i])
	}
	return reversed
}

func (o *OrchInstaller) Run(ctx context.Context, input OrchInstallerConfig, runtimeState *OrchInstallerRuntimeState) *OrchInstallerError {
	logger := Logger()
	// TODO: Add error handling and logging
	// TODO: collect runtimeStates from stages
	// TODO: Handle final runtimeState?

	if runtimeState.Action == "" {
		return &OrchInstallerError{
			ErrorCode: OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "action must be specified",
		}
	}

	if runtimeState.Action != "install" && runtimeState.Action != "upgrade" && runtimeState.Action != "uninstall" {
		return &OrchInstallerError{
			ErrorCode: OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  fmt.Sprintf("unsupported action: %s", runtimeState.Action),
		}
	}
	if runtimeState.Action == "uninstall" {
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
		err = stage.PreStage(ctx, input, runtimeState)

		// We will skip to run the stage if the previous stage failed
		if err == nil {
			err = stage.RunStage(ctx, input, runtimeState)
		}

		// But we will always run the post stage, the post stage should
		// handle the error and rollback if needed.
		err = stage.PostStage(ctx, input, runtimeState, err)
		if err != nil {
			err.ErrorStage = name
			return err
		}
	}
	return nil
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
