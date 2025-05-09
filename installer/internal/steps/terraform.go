package steps

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
)

type OrchInstallerTerraformStep struct {
	Action             string
	ExecPath           string
	ModulePath         string
	Variables          any // Any struct to seriaalize to HCL JSON
	BackendConfig      any // Any struct to seriaalize to HCL JSON
	LogFile            string
	KeepGeneratedFiles bool
}

type OrchInstallerTerraformStepOutput struct {
	Output map[string]string `yaml:"output"`
}

func marshalHCL(data any) ([]byte, error) {
	k := koanf.New(".")
	// Using the tag yaml here since we are using the yaml tag in the struct.
	err := k.Load(structs.Provider(data, "yaml"), nil)
	if err != nil {
		return nil, err
	}
	return k.Marshal(json.Parser()) // Terraform accepts json format
}

func (s *OrchInstallerTerraformStep) Run(ctx *context.Context) (*OrchInstallerTerraformStepOutput, *internal.OrchInstallerError) {
	logger := internal.Logger()
	logger.Debugf("Initializing backend and variables files")
	if _, err := os.Stat(fmt.Sprintf("%s/environments", s.ModulePath)); os.IsNotExist(err) {
		err := os.MkdirAll(fmt.Sprintf("%s/environments", s.ModulePath), os.ModePerm)
		if err != nil {
			return nil, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorMsg:  fmt.Sprintf("failed to create environments directory: %v", err),
			}
		}
	}
	backendConfigPath := fmt.Sprintf("%s/environments/backend.tfvars.json", s.ModulePath)
	variableFilePath := fmt.Sprintf("%s/environments/variables.tfvars.json", s.ModulePath)

	variables, err := marshalHCL(s.Variables)
	if err != nil {
		return nil, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to marshal variables: %v", err),
		}
	}

	err = os.WriteFile(variableFilePath, variables, 0644)
	if err != nil {
		return nil, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to write variables file: %v", err),
		}
	}

	backendConfig, err := marshalHCL(s.BackendConfig)
	if err != nil {
		return nil, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to marshal backend config: %v", err),
		}
	}

	err = os.WriteFile(backendConfigPath, backendConfig, 0644)
	if err != nil {
		return nil, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to write backend config file: %v", err),
		}
	}
	logger.Debugf("Backend and variables files created successfully")

	tf, err := tfexec.NewTerraform(s.ModulePath, s.ExecPath)
	if err != nil {
		return nil, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeTerraform,
			ErrorMsg:  fmt.Sprintf("failed to create terraform instance: %v", err),
		}
	}

	logger.Debugf("Initializing Terraform with backend config: %s", backendConfigPath)
	err = tf.Init(*ctx, tfexec.Upgrade(true), tfexec.BackendConfig(backendConfigPath))
	if err != nil {
		return nil, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeTerraform,
			ErrorMsg:  fmt.Sprintf("failed to create terraform instance: %v", err),
		}
	}
	logger.Debugf("Terraform backend initialized successfully")
	fileLogWriter, err := internal.FileLogWriter(s.LogFile)
	if err != nil {
		return nil, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to create file log writer: %v", err),
		}
	}
	if s.Action == "install" || s.Action == "upgrade" {
		logger.Debugf("Applying Terraform with variables file: %s", variableFilePath)
		err = tf.ApplyJSON(*ctx, fileLogWriter, tfexec.VarFile(variableFilePath))
		if err != nil {
			return nil, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  fmt.Sprintf("failed to apply terraform config: %v", err),
			}
		}
		logger.Debugf("Terraform applied successfully")
	} else if s.Action == "uninstall" {
		logger.Debugf("Destroying Terraform with variables file: %s", variableFilePath)
		err = tf.DestroyJSON(*ctx, fileLogWriter, tfexec.VarFile(variableFilePath))
		if err != nil {
			return nil, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  fmt.Sprintf("failed to destroy terraform config: %v", err),
			}
		}
		logger.Debugf("Terraform destroyed successfully")
	} else {
		return nil, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  fmt.Sprintf("unsupported action: %s", s.Action),
		}
	}

	output, err := tf.Output(*ctx)
	if err != nil {
		return nil, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeTerraform,
			ErrorMsg:  fmt.Sprintf("failed to retrieve terraform output: %v", err),
		}
	}

	convertedOutput := make(map[string]string)
	for key, value := range output {
		jsonValue, err := value.Value.MarshalJSON()
		if err != nil {
			return nil, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorMsg:  fmt.Sprintf("failed to marshal terraform output value: %v", err),
			}
		}
		convertedOutput[key] = string(jsonValue)
	}

	if !s.KeepGeneratedFiles {
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

	return &OrchInstallerTerraformStepOutput{
		Output: convertedOutput,
	}, nil
}
