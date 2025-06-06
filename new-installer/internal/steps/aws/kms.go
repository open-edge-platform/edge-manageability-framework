// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps_aws

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
)

const (
	KMSModulePath       = "new-installer/targets/aws/iac/kms"
	KMSBackendBucketKey = "kms.tfstate"
)

type KMSVariables struct {
	Region      string `json:"region" yaml:"region"`
	CustomerTag string `json:"customer_tag" yaml:"customer_tag"`
	ClusterName string `json:"cluster_name" yaml:"cluster_name"`
}

func NewKMSVariables() KMSVariables {
	return KMSVariables{
		Region:      "",
		CustomerTag: "",
		ClusterName: "",
	}
}

type KMSStep struct {
	variables          KMSVariables
	backendConfig      steps.TerraformAWSBucketBackendConfig
	RootPath           string
	KeepGeneratedFiles bool
	TerraformUtility   steps.TerraformUtility
	StepLabels         []string
}

func (s *KMSStep) Name() string {
	return "AWSKMSStep"
}

func (s *KMSStep) Labels() []string {
	return s.StepLabels
}

func (s *KMSStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	s.variables = NewKMSVariables()
	s.variables.Region = config.AWS.Region
	s.variables.CustomerTag = config.AWS.CustomerTag
	s.variables.ClusterName = config.Global.OrchName
	s.backendConfig = steps.TerraformAWSBucketBackendConfig{
		Bucket: config.Global.OrchName + "-" + runtimeState.DeploymentID,
		Region: config.AWS.Region,
		Key:    KMSBackendBucketKey,
	}
	return runtimeState, nil
}

func (s *KMSStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

func (s *KMSStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	terraformStepInput := steps.TerraformUtilityInput{
		Action:             runtimeState.Action,
		ModulePath:         filepath.Join(s.RootPath, KMSModulePath),
		Variables:          s.variables,
		BackendConfig:      s.backendConfig,
		LogFile:            filepath.Join(runtimeState.LogDir, "aws_kms.log"),
		KeepGeneratedFiles: s.KeepGeneratedFiles,
	}
	fmt.Printf("Running Terraform util %s with input: %+v\n", s.TerraformUtility, terraformStepInput)
	terraformStepOutput, err := s.TerraformUtility.Run(ctx, terraformStepInput)
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeTerraform,
			ErrorMsg:  fmt.Sprintf("failed to run terraform: %v", err),
		}
	}
	if runtimeState.Action == "uninstall" {
		return runtimeState, nil
	}
	if terraformStepOutput.Output != nil {
		fmt.Println("Terraform Output:")
	}
	return runtimeState, nil
}

func (s *KMSStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, prevStepError
}
