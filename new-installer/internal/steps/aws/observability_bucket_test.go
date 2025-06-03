// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package steps_aws_test

import (
	"crypto/rand"
	"encoding/json"
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
}

const (
	DeploymentID = "test-deployment-id"
)

func TestObservabilityBucketsStep(t *testing.T) {
	suite.Run(t, new(ObservabilityBucketsStepTest))
}

func (s *ObservabilityBucketsStepTest) SetupTest() {
	rootPath, err := filepath.Abs("../../../../")
	if err != nil {
		s.NoError(err)
		return
	}
	s.randomText = strings.ToLower(rand.Text()[0:8])
	s.logDir = filepath.Join(rootPath, ".logs")
	internal.InitLogger("debug", s.logDir)
	s.config.AWS.Region = "us-west-2"
	s.config.Global.OrchName = "observability-buckets-test"
	s.config.AWS.CustomerTag = "test"
	s.runtimeState.DeploymentID = DeploymentID
	s.runtimeState.LogDir = filepath.Join(rootPath, ".logs")
	s.tfUtility = &MockTerraformUtility{}
	s.step = &steps_aws.ObservabilityBucketsStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: true,
		TerraformUtility:   s.tfUtility,
	}
}

func (s *ObservabilityBucketsStepTest) TestInstallAndUninstallOBservabilityBucket() {
	s.runtimeState.Action = "install"
	s.expectTFUtiliyyCall("install")
	rs, err := steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}
	fmt.Println(rs)

	s.runtimeState.Action = "uninstall"
	s.expectTFUtiliyyCall("uninstall")
	_, err = steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
	}

}

func (s *ObservabilityBucketsStepTest) expectTFUtiliyyCall(action string) {
	input := steps.TerraformUtilityInput{
		Action:             action,
		ModulePath:         filepath.Join(s.step.RootPath, steps_aws.S3ModulePath),
		LogFile:            filepath.Join(s.logDir, "aws_observability_bucket.log"),
		KeepGeneratedFiles: s.step.KeepGeneratedFiles,
		Variables: steps_aws.ObservabilityBucketsVariables{
			Region:        s.config.AWS.Region,
			CustomerTag:   s.config.AWS.CustomerTag,
			S3Prefix:      s.config.Global.OrchName,
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
			Output: map[string]tfexec.OutputMeta{
				"s3": {
					Type:  json.RawMessage(`"list"`),
					Value: json.RawMessage(`[]`),
				},
			},
		}, nil).Once()
	} else {
		s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
			TerraformState: "",
			Output:         map[string]tfexec.OutputMeta{},
		}, nil).Once()
	}
}
