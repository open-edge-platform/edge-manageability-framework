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

type KMSStepTest struct {
	suite.Suite
	config       config.OrchInstallerConfig
	runtimeState config.OrchInstallerRuntimeState
	step         *steps_aws.KMSStep
	randomText   string
	logDir       string
	tfUtility    *MockTerraformUtility
}

func TestKMSStep(t *testing.T) {
	suite.Run(t, new(KMSStepTest))
}

func (s *KMSStepTest) SetupTest() {
	rootPath, err := filepath.Abs("../../../../")
	s.Require().NoError(err, "Failed to get absolute path")
	s.randomText = strings.ToLower(rand.Text()[0:8])
	s.logDir = filepath.Join(rootPath, ".logs")
	err = internal.InitLogger("debug", s.logDir)
	s.Require().NoError(err, "Failed to initialize logger")
	s.config.AWS.Region = "us-west-2"
	s.config.Global.OrchName = "kms-test"
	s.config.AWS.CustomerTag = "test"
	s.runtimeState.DeploymentID = "test-deployment-id"
	s.runtimeState.LogDir = filepath.Join(rootPath, ".logs")
	s.tfUtility = &MockTerraformUtility{}
	s.step = &steps_aws.KMSStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: true,
		TerraformUtility:   s.tfUtility,
	}
}

func (s *KMSStepTest) TestInstallAndUninstallKMS() {
	s.runtimeState.Action = "install"
	s.expectTFUtiliyCall("install")
	rs, err := steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}
	fmt.Println(rs)

	s.runtimeState.Action = "uninstall"
	s.expectTFUtiliyCall("uninstall")
	_, err = steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
	}
}

func (s *KMSStepTest) expectTFUtiliyCall(action string) {
	input := steps.TerraformUtilityInput{
		Action:             action,
		ModulePath:         filepath.Join(s.step.RootPath, steps_aws.KMSModulePath),
		LogFile:            filepath.Join(s.logDir, "aws_kms.log"),
		KeepGeneratedFiles: s.step.KeepGeneratedFiles,
		Variables: steps_aws.KMSVariables{
			Region:      s.config.AWS.Region,
			CustomerTag: s.config.AWS.CustomerTag,
			ClusterName: s.config.Global.OrchName,
		},
		BackendConfig: steps.TerraformAWSBucketBackendConfig{
			Region: s.config.AWS.Region,
			Bucket: fmt.Sprintf("%s-%s", s.config.Global.OrchName, s.runtimeState.DeploymentID),
			Key:    steps_aws.KMSBackendBucketKey,
		},
		TerraformState: "",
	}
	s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
		TerraformState: "",
		Output:         map[string]tfexec.OutputMeta{},
	}, nil).Once()
}
