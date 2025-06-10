// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
)

const (
	TerraformVersion = "1.9.5"
)

type TerraformUtility interface {
	// Apply or destroy
	Run(ctx context.Context, input TerraformUtilityInput) (TerraformUtilityOutput, *internal.OrchInstallerError)
	MoveStates(ctx context.Context, input TerraformUtilityMoveStatesInput) *internal.OrchInstallerError
	RemoveStates(ctx context.Context, input TerraformUtilityRemoveStatesInput) *internal.OrchInstallerError
}

type TerraformUtilityInput struct {
	Action     string
	ModulePath string
	Variables  any // Any struct to serialize to HCL JSON
	// Either use backend config or backend state. Cannot use both.
	BackendConfig      any // Any struct to serialize to HCL JSON
	TerraformState     string
	LogFile            string
	KeepGeneratedFiles bool
}

type TerraformUtilityOutput struct {
	Output         map[string]tfexec.OutputMeta `json:"output"`
	TerraformState string                       `json:"terraform_state"`
}

type TerraformUtilityMoveStatesInput struct {
	ModulePath string
	States     map[string]string
}

type TerraformUtilityRemoveStatesInput struct {
	ModulePath string
	States     []string
}

type TerraformAWSBucketBackendConfig struct {
	Region string `json:"region"`
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
}

func marshalHCLJSON(data any) ([]byte, error) {
	return json.Marshal(data)
}

func validateInput(input TerraformUtilityInput) *internal.OrchInstallerError {
	if input.Action == "" {
		return &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "action must be specified",
		}
	}
	if input.ModulePath == "" {
		return &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "module path must be specified",
		}
	}
	if input.Variables == nil {
		return &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "variables must be specified",
		}
	}
	if input.LogFile == "" {
		return &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "log file must be specified",
		}
	}
	if input.Action != "install" && input.Action != "upgrade" && input.Action != "uninstall" {
		return &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  fmt.Sprintf("unsupported action: %s", input.Action),
		}
	}
	if input.BackendConfig != nil && input.TerraformState != "" {
		return &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "either backend config or terraform state must be specified, not both",
		}
	}
	return nil
}

type terraformUtilityImpl struct {
	ExecPath string
}

func CreateTerraformUtility(terraformCommandPath string) (TerraformUtility, *internal.OrchInstallerError) {
	if terraformCommandPath == "" {
		return nil, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "exec path must be specified",
		}
	}
	return &terraformUtilityImpl{
		ExecPath: terraformCommandPath,
	}, nil
}

func (tfUtil *terraformUtilityImpl) Run(ctx context.Context, input TerraformUtilityInput) (TerraformUtilityOutput, *internal.OrchInstallerError) {
	logger := internal.Logger()
	validationErr := validateInput(input)
	if validationErr != nil {
		return TerraformUtilityOutput{}, validationErr
	}

	logger.Debugf("Initializing backend and variables files")
	envPath := filepath.Join(input.ModulePath, "environments")
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		err := os.MkdirAll(envPath, os.ModePerm)
		if err != nil {
			return TerraformUtilityOutput{}, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorMsg:  fmt.Sprintf("failed to create environments directory: %v", err),
			}
		}
	}

	variableFilePath := filepath.Join(input.ModulePath, "environments", "variables.tfvars.json")
	variables, err := marshalHCLJSON(input.Variables)
	if err != nil {
		return TerraformUtilityOutput{}, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to marshal variables: %v", err),
		}
	}
	err = os.WriteFile(variableFilePath, variables, 0o644)
	if err != nil {
		return TerraformUtilityOutput{}, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to write variables file: %v", err),
		}
	}

	tf, err := tfexec.NewTerraform(input.ModulePath, tfUtil.ExecPath)
	if err != nil {
		return TerraformUtilityOutput{}, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeTerraform,
			ErrorMsg:  fmt.Sprintf("failed to create terraform instance: %v", err),
		}
	}
	if input.BackendConfig != nil {
		backendConfigPath := filepath.Join(input.ModulePath, "environments", "backend.tfvars.json")
		backendConfig, err := marshalHCLJSON(input.BackendConfig)
		if err != nil {
			return TerraformUtilityOutput{}, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorMsg:  fmt.Sprintf("failed to marshal backend config: %v", err),
			}
		}
		err = os.WriteFile(backendConfigPath, backendConfig, 0o644)
		if err != nil {
			return TerraformUtilityOutput{}, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorMsg:  fmt.Sprintf("failed to write backend config file: %v", err),
			}
		}
		logger.Debugf("Backend and variables files created successfully")
		logger.Debugf("Initializing Terraform with backend config: %s", backendConfigPath)
		err = tf.Init(ctx, tfexec.Upgrade(true), tfexec.BackendConfig(backendConfigPath), tfexec.Reconfigure(true))
		if err != nil {
			return TerraformUtilityOutput{}, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  fmt.Sprintf("failed to initialize Terraform backend: %v", err),
			}
		}
	} else {
		// Since we already validated that either backend config or terraform state is provided,
		// we can safely assume that terraform state is provided.
		terraformStatePath := filepath.Join(input.ModulePath, "terraform.tfstate")
		if _, err := os.Stat(terraformStatePath); err == nil {
			logger.Debug("Terraform state file exists, deleting it")
			if err := os.Remove(terraformStatePath); err != nil {
				return TerraformUtilityOutput{}, &internal.OrchInstallerError{
					ErrorCode: internal.OrchInstallerErrorCodeInternal,
					ErrorMsg:  fmt.Sprintf("failed to delete existing terraform state file: %v", err),
				}
			}
			logger.Debug("Successfully deleted existing terraform state file")
		}
		logger.Debug("Initializing Terraform with no backend config")
		err = tf.Init(ctx, tfexec.Upgrade(true), tfexec.Backend(false), tfexec.Reconfigure(true))
		if err != nil {
			return TerraformUtilityOutput{}, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  fmt.Sprintf("failed to create terraform instance: %v", err),
			}
		}
		if input.TerraformState != "" {
			logger.Debug("Loading state bucket state from runtime state")
			// We already have a state bucket state. Need to load it to the module before init.
			if err := os.WriteFile(terraformStatePath, []byte(input.TerraformState), 0o644); err != nil {
				return TerraformUtilityOutput{}, &internal.OrchInstallerError{
					ErrorCode: internal.OrchInstallerErrorCodeInternal,
					ErrorMsg:  fmt.Sprintf("failed to write terraform state file: %v", err),
				}
			}
			logger.Debug("Successfully load state bucket state from runtime state")
		}
	}
	logger.Debugf("Terraform backend initialized successfully")
	fileLogWriter, err := internal.FileLogWriter(input.LogFile)
	if err != nil {
		return TerraformUtilityOutput{}, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to create file log writer: %v", err),
		}
	}
	if input.Action == "install" || input.Action == "upgrade" {
		logger.Debugf("Applying Terraform with variables file: %s", variableFilePath)
		err = tf.ApplyJSON(ctx, fileLogWriter, tfexec.VarFile(variableFilePath))
		if err != nil {
			return TerraformUtilityOutput{}, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  fmt.Sprintf("failed to apply terraform config: %v", err),
			}
		}
		logger.Debugf("Terraform applied successfully")
	} else if input.Action == "uninstall" {
		logger.Debugf("Destroying Terraform with variables file: %s", variableFilePath)
		err = tf.DestroyJSON(ctx, fileLogWriter, tfexec.VarFile(variableFilePath), tfexec.Refresh(false))
		if err != nil {
			return TerraformUtilityOutput{}, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  fmt.Sprintf("failed to destroy terraform config: %v", err),
			}
		}
		logger.Debugf("Terraform destroyed successfully")
	} else {
		return TerraformUtilityOutput{}, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  fmt.Sprintf("unsupported action: %s", input.Action),
		}
	}

	output, err := tf.Output(ctx)
	if err != nil {
		return TerraformUtilityOutput{}, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeTerraform,
			ErrorMsg:  fmt.Sprintf("failed to retrieve terraform output: %v", err),
		}
	}

	// Preserve terraform state in the runtime state
	var terraformState string
	if input.BackendConfig == nil {
		stateJson, err := os.ReadFile(filepath.Join(input.ModulePath, "terraform.tfstate"))
		if err != nil {
			return TerraformUtilityOutput{}, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorMsg:  fmt.Sprintf("failed to read terraform state file: %v", err),
			}
		}
		terraformState = string(stateJson)
	}

	if !input.KeepGeneratedFiles {
		variableFilePath := filepath.Join(input.ModulePath, "environments", "variables.tfvars.json")
		backendConfigPath := filepath.Join(input.ModulePath, "environments", "backend.tfvars.json")
		if _, err := os.Stat(backendConfigPath); err == nil {
			logger.Debugf("Deleting backend config file: %s", backendConfigPath)
			if err := os.Remove(backendConfigPath); err != nil {
				logger.Warnf("failed to delete backend config file %s: %v", backendConfigPath, err)
			}
		}
		if _, err := os.Stat(variableFilePath); err == nil {
			logger.Debugf("Deleting variables file: %s", variableFilePath)
			if err := os.Remove(variableFilePath); err != nil {
				logger.Warnf("failed to delete variables file %s: %v", variableFilePath, err)
			}
		}
	}

	return TerraformUtilityOutput{
		Output:         output,
		TerraformState: terraformState,
	}, nil
}

func InstallTerraformAndGetExecPath() (string, error) {
	installer := &releases.ExactVersion{
		Product: product.Terraform,
		Version: version.Must(version.NewVersion(TerraformVersion)),
	}
	return installer.Install(context.Background())
}

func (tfUtil *terraformUtilityImpl) MoveStates(ctx context.Context, input TerraformUtilityMoveStatesInput) *internal.OrchInstallerError {
	tf, err := tfexec.NewTerraform(input.ModulePath, tfUtil.ExecPath)
	if err != nil {
		return &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeTerraform,
			ErrorMsg:  fmt.Sprintf("failed to create terraform instance: %v", err),
		}
	}
	for oldStateName, newStateName := range input.States {
		err = tf.StateMv(ctx, oldStateName, newStateName)
		if err != nil {
			return &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  fmt.Sprintf("failed to move terraform state: %v", err),
			}
		}
	}
	return nil
}

func (tfUtil *terraformUtilityImpl) RemoveStates(ctx context.Context, input TerraformUtilityRemoveStatesInput) *internal.OrchInstallerError {
	tf, err := tfexec.NewTerraform(input.ModulePath, tfUtil.ExecPath)
	if err != nil {
		return &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeTerraform,
			ErrorMsg:  fmt.Sprintf("failed to create terraform instance: %v", err),
		}
	}
	for _, stateName := range input.States {
		err = tf.StateRm(ctx, stateName)
		if err != nil {
			return &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  fmt.Sprintf("failed to delete terraform state: %v", err),
			}
		}
	}
	return nil
}
