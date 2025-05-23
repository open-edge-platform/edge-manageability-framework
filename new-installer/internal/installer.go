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
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
)

type OrchInstaller struct {
	Stages []OrchInstallerStage

	mutex     *sync.Mutex
	cancelled bool
}

func UpdateRuntimeState(dest *config.OrchInstallerRuntimeState, source config.OrchInstallerRuntimeState) *OrchInstallerError {
	srcK := koanf.New(".")
	srcK.Load(structs.Provider(source, "yaml"), nil)
	dstK := koanf.New(".")
	dstK.Load(structs.Provider(dest, "yaml"), nil)
	dstK.Merge(srcK)

	dstData, err := dstK.Marshal(yaml.Parser())
	if err != nil {
		return &OrchInstallerError{
			ErrorCode: OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to marshal runtime state: %v", err),
		}
	}

	err = config.DeserializeFromYAML(dest, dstData)
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

func (o *OrchInstaller) Run(ctx context.Context, config config.OrchInstallerConfig) *OrchInstallerError {
	logger := Logger()
	action := config.Generated.Action
	if action == "" {
		return &OrchInstallerError{
			ErrorCode: OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "action must be specified",
		}
	}

	if action != "install" && action != "upgrade" && action != "uninstall" {
		return &OrchInstallerError{
			ErrorCode: OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  fmt.Sprintf("unsupported action: %s", action),
		}
	}
	if action == "uninstall" {
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
		err = stage.PreStage(ctx, &config)

		// We will skip to run the stage if the previous stage failed
		if err == nil {
			err = stage.RunStage(ctx, &config)
		}

		// But we will always run the post stage, the post stage should
		// handle the error and rollback if needed.
		err = stage.PostStage(ctx, &config, err)
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
	for name, stepErr := range err.StepErrors {
		if stepErr != nil {
			msg += fmt.Sprintf("Step: %s\n", name)
			msg += fmt.Sprintf("Error: %s\n", stepErr.ErrorMsg)
		}
	}
	return msg
}
