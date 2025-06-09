// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package steps_aws_test

import (
	"crypto/rand"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	steps_aws "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/aws"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ObservabilityBucketsStepTest struct {
	suite.Suite
	config       config.OrchInstallerConfig
	runtimeState config.OrchInstallerRuntimeState
	step         *steps_aws.ObservabilityBucketsStep
	randomText   string
	logDir       string
	tfUtility    *MockTerraformUtility
	awsUtility   *MockAWSUtility
}

const (
	DeploymentID = "test-deployment-id"
)

func TestObservabilityBucketsStep(t *testing.T) {
	suite.Run(t, new(ObservabilityBucketsStepTest))
}

func (s *ObservabilityBucketsStepTest) SetupTest() {
	rootPath, err := filepath.Abs("../../../../")
	s.Require().NoError(err, "Failed to get absolute path")
	s.randomText = strings.ToLower(rand.Text()[0:8])
	s.logDir = filepath.Join(rootPath, ".logs")
	err = internal.InitLogger("debug", s.logDir)
	s.Require().NoError(err, "Failed to initialize logger")
	s.config.AWS.Region = "us-west-2"
	s.config.Global.OrchName = "observability-buckets-test"
	s.config.AWS.CustomerTag = "test"
	s.runtimeState.DeploymentID = DeploymentID
	s.runtimeState.AWS.EKSOIDCIssuer = "https://oidc.eks.us-west-2.amazonaws.com/id/test-oidc-id"
	s.runtimeState.LogDir = filepath.Join(rootPath, ".logs")
	s.tfUtility = &MockTerraformUtility{}
	s.awsUtility = &MockAWSUtility{}
	s.step = &steps_aws.ObservabilityBucketsStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: true,
		TerraformUtility:   s.tfUtility,
		AWSUtility:         s.awsUtility,
	}
}

func (s *ObservabilityBucketsStepTest) TestInstallAndUninstallObservabilityBucket() {
	s.runtimeState.Action = "install"
	s.expectTFUtiliyyCall("install")
	_, err := steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}

	s.runtimeState.Action = "uninstall"
	s.expectTFUtiliyyCall("uninstall")
	_, err = steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
	}
}

func (s *ObservabilityBucketsStepTest) TestUpgradeObservabilityBucket() {
	s.runtimeState.Action = "upgrade"
	s.config.AWS.PreviousS3StateBucket = "old-bucket-name"
	s.expectTFUtiliyyCall("upgrade")
	_, err := steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}
}

func (s *ObservabilityBucketsStepTest) expectTFUtiliyyCall(action string) {
	input := steps.TerraformUtilityInput{
		Action:             action,
		ModulePath:         filepath.Join(s.step.RootPath, steps_aws.ObservabilityBucketsModulePath),
		LogFile:            filepath.Join(s.logDir, "aws_observability_bucket.log"),
		KeepGeneratedFiles: s.step.KeepGeneratedFiles,
		Variables: steps_aws.ObservabilityBucketsVariables{
			Region:        s.config.AWS.Region,
			CustomerTag:   s.config.AWS.CustomerTag,
			S3Prefix:      s.runtimeState.DeploymentID,
			OIDCIssuer:    s.runtimeState.AWS.EKSOIDCIssuer,
			ClusterName:   s.config.Global.OrchName,
			CreateTracing: false,
		},
		BackendConfig: steps.TerraformAWSBucketBackendConfig{
			Region: s.config.AWS.Region,
			Bucket: fmt.Sprintf("%s-%s", s.config.Global.OrchName, s.runtimeState.DeploymentID),
			Key:    steps_aws.ObservabilityBucketsBackendBucketKey,
		},
		TerraformState: "",
	}
	if action == "install" {
		s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
			TerraformState: "",
			Output:         map[string]tfexec.OutputMeta{},
		}, nil).Once()
	}
	if action == "uninstall" {
		s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
			TerraformState: "",
			Output:         map[string]tfexec.OutputMeta{},
		}, nil).Once()
	}
	if action == "upgrade" {
		s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
			TerraformState: "",
			Output:         map[string]tfexec.OutputMeta{},
		}, nil).Once()

		s.awsUtility.On("S3CopyToS3",
			s.config.AWS.Region,
			s.config.AWS.PreviousS3StateBucket,
			fmt.Sprintf("%s/o11y_buckets/%s", s.config.AWS.Region, s.config.Global.OrchName),
			s.config.AWS.Region,
			s.config.Global.OrchName+"-"+s.runtimeState.DeploymentID,
			"o11y_buckets.tfstate",
		).Return(nil).Once()

		s.tfUtility.On("MoveStates", mock.Anything, steps.TerraformUtilityMoveStatesInput{
			ModulePath: filepath.Join(s.step.RootPath, steps_aws.ObservabilityBucketsModulePath),
			States: map[string]string{
				"module.s3.aws_iam_policy.s3_policy":                             "aws_iam_policy.s3_policy",
				"module.s3.aws_iam_role.s3_role":                                 "aws_iam_role.s3_role",
				"module.s3.aws_s3_bucket.bucket":                                 "aws_s3_bucket.bucket",
				"module.s3.aws_s3_bucket_lifecycle_configuration.bucket_config":  "aws_s3_bucket_lifecycle_configuration.bucket_config",
				"module.s3.aws_s3_bucket_policy.bucket_policy":                   "aws_s3_bucket_policy.bucket_policy",
				"module.s3.aws_s3_bucket.tracing":                                "aws_s3_bucket.tracing",
				"module.s3.aws_s3_bucket_lifecycle_configuration.tracing_config": "aws_s3_bucket_lifecycle_configuration.tracing_config",
				"module.s3.aws_s3_bucket_policy.tracing_policy":                  "aws_s3_bucket_policy.tracing_policy",
			},
		}).Return(nil).Once()
	}
}
