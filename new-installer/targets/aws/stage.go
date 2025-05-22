// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"context"
	"fmt"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
)

type AWSStage struct {
	steps []steps.OrchInstallerStep
	name  string
}

func NewAWSStage(name string, steps []steps.OrchInstallerStep) *AWSStage {
	return &AWSStage{
		steps: steps,
		name:  name,
	}
}
func (a *AWSStage) Name() string {
	return a.name
}

func (a *AWSStage) PreStage(ctx context.Context, config *internal.OrchInstallerConfig) *internal.OrchInstallerStageError {
	logger := internal.Logger()
	containsError := false
	var stepErrors map[string]*internal.OrchInstallerError = make(map[string]*internal.OrchInstallerError)
	for _, step := range a.steps {
		logger.Debugf("ConfigStep %s", step.Name())
		if newRuntimeState, err := step.ConfigStep(ctx, *config); err != nil {
			stepErrors[step.Name()] = err
			containsError = true
		} else if err = internal.UpdateRuntimeState(&config.Generated, newRuntimeState); err != nil {
			stepErrors[step.Name()] = err
			containsError = true
		}
		logger.Debug("Uploading runtime state to S3 after ConfigStep %s", step.Name())
		if uploadError := UploadStateToS3(*config); uploadError != nil {
			return &internal.OrchInstallerStageError{
				StepErrors: stepErrors,
				ErrorCode:  internal.OrchInstallerErrorCodeStateUploadFailed,
				ErrorMsg:   fmt.Sprintf("Failed to upload runtime state to S3: %v", uploadError),
			}
		}
		logger.Debugf("PreStep %s", step.Name())
		if newRuntimeState, err := step.PreStep(ctx, *config); err != nil {
			stepErrors[step.Name()] = err
			containsError = true
		} else if err = internal.UpdateRuntimeState(&config.Generated, newRuntimeState); err != nil {
			stepErrors[step.Name()] = err
			containsError = true
		}
		logger.Debug("Uploading runtime state to S3 after PreStep %s", step.Name())
		if uploadError := UploadStateToS3(*config); uploadError != nil {
			return &internal.OrchInstallerStageError{
				StepErrors: stepErrors,
				ErrorCode:  internal.OrchInstallerErrorCodeStateUploadFailed,
				ErrorMsg:   fmt.Sprintf("Failed to upload runtime state to S3: %v", uploadError),
			}
		}
	}
	if containsError {
		return &internal.OrchInstallerStageError{
			StepErrors: stepErrors,
		}
	}
	return nil
}

func (a *AWSStage) RunStage(ctx context.Context, config *internal.OrchInstallerConfig) *internal.OrchInstallerStageError {
	logger := internal.Logger()
	containsError := false
	var stepErrors map[string]*internal.OrchInstallerError = make(map[string]*internal.OrchInstallerError)
	for _, step := range a.steps {
		logger.Debugf("RunStep %s", step.Name())
		if newRuntimeState, err := step.RunStep(ctx, *config); err != nil {
			stepErrors[step.Name()] = err
			containsError = true
		} else if err = internal.UpdateRuntimeState(&config.Generated, newRuntimeState); err != nil {
			stepErrors[step.Name()] = err
			containsError = true
		}
		logger.Debug("Uploading runtime state to S3 after RunStep %s", step.Name())
		if uploadError := UploadStateToS3(*config); uploadError != nil {
			return &internal.OrchInstallerStageError{
				StepErrors: stepErrors,
				ErrorCode:  internal.OrchInstallerErrorCodeStateUploadFailed,
				ErrorMsg:   fmt.Sprintf("Failed to upload runtime state to S3: %v", uploadError),
			}
		}
	}
	if containsError {
		return &internal.OrchInstallerStageError{
			StepErrors: stepErrors,
		}
	}
	return nil
}

func (a *AWSStage) PostStage(ctx context.Context, config *internal.OrchInstallerConfig, prevStageError *internal.OrchInstallerStageError) *internal.OrchInstallerStageError {
	logger := internal.Logger()
	containsError := false
	var stepErrors map[string]*internal.OrchInstallerError = make(map[string]*internal.OrchInstallerError)
	for _, step := range a.steps {
		var stepError *internal.OrchInstallerError = nil
		if prevStageError != nil {
			stepError = prevStageError.StepErrors[step.Name()]
		}
		logger.Debugf("PostStep %s", step.Name())
		if newRuntimeState, err := step.PostStep(ctx, *config, stepError); err != nil {
			stepErrors[step.Name()] = err
			containsError = true
		} else if err = internal.UpdateRuntimeState(&config.Generated, newRuntimeState); err != nil {
			stepErrors[step.Name()] = err
			containsError = true
		}
		logger.Debug("Uploading runtime state to S3 after PostStep %s", step.Name())
		if uploadError := UploadStateToS3(*config); uploadError != nil {
			return &internal.OrchInstallerStageError{
				StepErrors: stepErrors,
				ErrorCode:  internal.OrchInstallerErrorCodeStateUploadFailed,
				ErrorMsg:   fmt.Sprintf("Failed to upload runtime state to S3: %v", uploadError),
			}
		}
	}
	if containsError {
		return &internal.OrchInstallerStageError{
			StepErrors: stepErrors,
		}
	}
	return nil
}
