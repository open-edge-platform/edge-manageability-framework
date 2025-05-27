// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"context"
	"fmt"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
)

type AWSStage struct {
	steps                  []steps.OrchInstallerStep
	name                   string
	labels                 []string
	orchConfigReaderWriter config.OrchConfigReaderWriter
}

func NewAWSStage(name string, steps []steps.OrchInstallerStep, labels []string, orchConfigReaderWriter config.OrchConfigReaderWriter) *AWSStage {
	return &AWSStage{
		steps:                  steps,
		name:                   name,
		labels:                 labels,
		orchConfigReaderWriter: orchConfigReaderWriter,
	}
}
func (a *AWSStage) Name() string {
	return a.name
}

func (a *AWSStage) Labels() []string {
	return a.labels
}

func (a *AWSStage) PreStage(ctx context.Context, config *config.OrchInstallerConfig) *internal.OrchInstallerError {
	return nil
}

func (a *AWSStage) RunStage(ctx context.Context, config *config.OrchInstallerConfig) *internal.OrchInstallerError {
	logger := internal.Logger()
	if config == nil {
		return &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "OrchInstallerConfig is nil",
		}
	}
	if config.Generated.Action == "uninstall" {
		a.steps = steps.ReverseSteps(a.steps)
	}
	a.steps = steps.FilterSteps(a.steps, config.Advanced.TargetLabels)
	if len(a.steps) == 0 {
		return nil
	}

	for _, step := range a.steps {
		logger.Debugf("ConfigStep %s", step.Name())
		stepErr := func() *internal.OrchInstallerError {
			if newRuntimeState, err := step.ConfigStep(ctx, *config); err != nil {
				return err
			} else if err = internal.UpdateRuntimeState(&config.Generated, newRuntimeState); err != nil {
				return err
			}
			if uploadError := a.orchConfigReaderWriter.WriteOrchConfig(*config); uploadError != nil {
				return &internal.OrchInstallerError{
					ErrorCode: internal.OrchInstallerErrorCodeStateUploadFailed,
					ErrorMsg:  fmt.Sprintf("Failed to write state: %v", uploadError),
				}
			}
			logger.Debugf("PreStep %s", step.Name())
			if newRuntimeState, err := step.PreStep(ctx, *config); err != nil {
				return err
			} else if err = internal.UpdateRuntimeState(&config.Generated, newRuntimeState); err != nil {
				return err
			}
			if uploadError := a.orchConfigReaderWriter.WriteOrchConfig(*config); uploadError != nil {
				return &internal.OrchInstallerError{
					ErrorCode: internal.OrchInstallerErrorCodeStateUploadFailed,
					ErrorMsg:  fmt.Sprintf("Failed to write state: %v", uploadError),
				}
			}
			logger.Debugf("RunStep %s", step.Name())
			if newRuntimeState, err := step.RunStep(ctx, *config); err != nil {
				return err
			} else if err = internal.UpdateRuntimeState(&config.Generated, newRuntimeState); err != nil {
				return err
			}
			if uploadError := a.orchConfigReaderWriter.WriteOrchConfig(*config); uploadError != nil {
				return &internal.OrchInstallerError{
					ErrorCode: internal.OrchInstallerErrorCodeStateUploadFailed,
					ErrorMsg:  fmt.Sprintf("Failed to write state: %v", uploadError),
				}
			}
			return nil
		}()

		logger.Debugf("PostStep %s", step.Name())
		if newRuntimeState, err := step.PostStep(ctx, *config, stepErr); err != nil {
			return err
		} else if err = internal.UpdateRuntimeState(&config.Generated, newRuntimeState); err != nil {
			return err
		}
		if uploadError := a.orchConfigReaderWriter.WriteOrchConfig(*config); uploadError != nil {
			return &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeStateUploadFailed,
				ErrorMsg:  fmt.Sprintf("Failed to write state: %v", uploadError),
			}
		}
	}
	return nil
}

func (a *AWSStage) PostStage(ctx context.Context, config *config.OrchInstallerConfig, prevStageError *internal.OrchInstallerError) *internal.OrchInstallerError {
	return nil
}
