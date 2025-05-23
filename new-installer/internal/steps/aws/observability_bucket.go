// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps_aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
)

const (
	ModulePath = "pod-configs/module/s3"
)

type AWSObservabilityBucketsVariables struct{
	AWSAccountID                  string                    `json:"aws_accountid" yaml:"aws_accountid"`
	S3Prefix				   string                      `json:"s3_prefix" yaml:"s3_prefix"`
	ClusterName				   string                   `json:"cluster_name" yaml:"cluster_name"`
	CreateTracing	   bool                     `json:"create_tracing" yaml:"create_tracing"`
	ImportBuckets	   bool                     `json:"import_buckets" yaml:"import_buckets"`

}

func NewAWSObservabilityBucketsVariables() AWSObservabilityBucketsVariables {
	return AWSObservabilityBucketsVariables{
		AWSAccountID: "",
		S3Prefix:     "",
		ClusterName:  "",
		CreateTracing: false,
		ImportBuckets: false,
	}
}

type AWSObservabilityBucketsStep struct {
	variables AWSObservabilityBucketsVariables
	backendConfig      TerraformAWSBucketBackendConfig
	RootPath           string
	KeepGeneratedFiles bool
}

func (s *AWSObservabilityBucketsStep) Name() string {
	return "AWSObservabilityBucketsStep"
}

func (s *AWSObservabilityBucketsStep) ConfigStep(ctx context.Context, config internal.OrchInstallerConfig, runtimeState internal.OrchInstallerRuntimeState) (internal.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	s.variables = NewAWSObservabilityBucketsVariables()
	s.variables.AWSAccountID = config.

	s.backendConfig = TerraformAWSBucketBackendConfig{
		Region: config.Region,
		Bucket: config.DeploymentName + "-" + config.StateStoreBucketPostfix,
		Key:    config.Region + "/vpc/" + config.DeploymentName,
	}
	return runtimeState, nil
}

func (s *AWSObservabilityBucketsStep) PreStep(ctx context.Context, config internal.OrchInstallerConfig, runtimeState internal.OrchInstallerRuntimeState) (internal.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	terraformExecPath, err := steps.InstallTerraformAndGetExecPath()
	runtimeState.TerraformExecPath = terraformExecPath
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to get terraform exec path: %v", err),
		}
	}
	return runtimeState, nil
}

func (s *AWSObservabilityBucketsStep) RunStep(ctx context.Context, config internal.OrchInstallerConfig, runtimeState internal.OrchInstallerRuntimeState) (internal.OrchInstallerRuntimeState, *internal.OrchInstallerError) {

	return runtimeState, nil
}

func (s *AWSObservabilityBucketsStep) PostStep(ctx context.Context, config internal.OrchInstallerConfig, runtimeState internal.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (internal.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}