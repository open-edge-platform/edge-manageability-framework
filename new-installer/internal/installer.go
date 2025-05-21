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
	TargetEnvironment string `yaml:"target_environment"`
	// The DeploymentName that will be shared across all the stages.
	// Including but not limited to:
	// - Backend storage name(S3 bucket, Azure storage, dir, ...)
	// - VPC/VPN
	// - EKS/AKS/RKE2 cluster name
	// - Database name
	// ...
	DeploymentName string `yaml:"deployment_name"`
	CustomerTag    string `yaml:"customer_tag"`

	// Cloud deployment specific fields
	NetworkCIDR             string   `yaml:"network_cidr"`
	Region                  string   `yaml:"region"`
	StateStoreBucketPostfix string   `yaml:"state_store_bucket_postfix"`
	JumpHostIPAllowList     []string `yaml:"jumphost_ip_allow_list"`
}

// Runtime state that will be shared across all the stages
type OrchInstallerRuntimeState struct {
	Mutex *sync.Mutex
	// Schema version of the runtime state
	RuntimeStateVersion string `yaml:"config_version"`
	OrchVersion         string `yaml:"orch_version"`
	// The Action that will be performed
	// This can be one of the following:
	// - install
	// - upgrade
	// - uninstall
	Action string `yaml:"action"`
	// The directory where the logs will be saved
	LogDir            string `yaml:"log_dir"`
	DryRun            bool   `yaml:"dry_run"`
	TerraformExecPath string `yaml:"terraform_exec_path"`

	// Infra-specific runtime state
	// VPC(AWS) or VPN(Azure) ID
	VPCID                    string   `yaml:"vpc_id"`
	PublicSubnetIds          []string `yaml:"public_subnet_ids"`
	PrivateSubnetIds         []string `yaml:"private_subnet_ids"`
	JumpHostSSHKeyPublicKey  string   `yaml:"jump_host_ssh_key_public_key"`
	JumpHostSSHKeyPrivateKey string   `yaml:"jump_host_ssh_key_private_key"`
}

func (rs *OrchInstallerRuntimeState) UpdateRuntimeState(source OrchInstallerRuntimeState) *OrchInstallerError {
	rs.Mutex.Lock()
	defer rs.Mutex.Unlock()
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
		var err *OrchInstallerStageError
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
			return &OrchInstallerError{
				ErrorCode: OrchInstallerErrorCodeInternal,
				ErrorMsg:  BuildErrorMessage(name, err),
			}
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

func BuildErrorMessage(stageName string, err *OrchInstallerStageError) string {
	if err == nil {
		return ""
	}
	msg := "Stage: " + stageName + "\n"
	for i, stepErr := range err.StepErrors {
		if stepErr != nil {
			msg += fmt.Sprintf("Step: %d\n", i)
			msg += fmt.Sprintf("Error: %s\n", stepErr.ErrorMsg)
		}
	}
	return msg
}
