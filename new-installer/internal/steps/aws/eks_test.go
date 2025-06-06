// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package steps_aws_test

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	steps_aws "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/aws"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type EKSStepTest struct {
	suite.Suite
	config       config.OrchInstallerConfig
	runtimeState config.OrchInstallerRuntimeState
	step         *steps_aws.EKSStep
	randomText   string
	logDir       string
	tfUtility    *MockTerraformUtility
	awsUtility   *MockAWSUtility
}

func TestEKSStep(t *testing.T) {
	suite.Run(t, new(EKSStepTest))
}

func (s *EKSStepTest) SetupTest() {
	rootPath, err := filepath.Abs("../../../../")
	if err != nil {
		s.NoError(err)
		return
	}
	s.randomText = strings.ToLower(rand.Text()[0:8])
	s.logDir = filepath.Join(rootPath, ".logs")
	internal.InitLogger("debug", s.logDir)
	s.config.AWS.Region = "us-west-2"
	s.config.Global.Scale = config.Scale50
	s.config.Global.OrchName = "test"
	s.config.AWS.CacheRegistry = "test-cache-registry"
	s.runtimeState.DeploymentID = s.randomText
	s.runtimeState.LogDir = filepath.Join(rootPath, ".logs")
	s.runtimeState.VPCID = "vpc-12345678"
	s.runtimeState.PrivateSubnetIDs = []string{"subnet-12345678", "subnet-23456789", "subnet-34567890"}

	if _, err := os.Stat(s.logDir); os.IsNotExist(err) {
		err := os.MkdirAll(s.logDir, os.ModePerm)
		if err != nil {
			s.NoError(err)
			return
		}
	}
	s.tfUtility = &MockTerraformUtility{}
	s.awsUtility = &MockAWSUtility{}
	s.step = &steps_aws.EKSStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: true,
		TerraformUtility:   s.tfUtility,
		AWSUtility:         s.awsUtility,
	}
}

func (s *EKSStepTest) TestInstallAndUninstallEKS() {
	s.runtimeState.Action = "install"
	s.expectUtiliyCall("install")
	// We won't update the runtime state in this test, so we don't check the return value
	_, err := steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}
}

func (s *EKSStepTest) expectUtiliyCall(action string) {

	expectedVariables := steps_aws.EKSVariables{
		EKSVersion: "1.32",
		AddOns: []steps_aws.EKSAddOn{
			{
				Name:    "aws-ebs-csi-driver",
				Version: "v1.39.0-eksbuild.1",
			},
			{
				Name:                "vpc-cni",
				Version:             "v1.19.2-eksbuild.1",
				ConfigurationValues: "{\"enableNetworkPolicy\": \"true\", \"nodeAgent\": {\"healthProbeBindAddr\": \"8163\", \"metricsBindAddr\": \"8162\"}}",
			},
			{
				Name:    "aws-efs-csi-driver",
				Version: "v2.1.4-eksbuild.1",
			},
		},
		UserScriptPreCloudInit:  "",
		UserScriptPostCloudInit: "",
		Name:                    "test",
		Region:                  "us-west-2",
		VPCID:                   "vpc-12345678",
		CustomerTag:             "",
		SubnetIDs:               []string{"subnet-12345678", "subnet-23456789", "subnet-34567890"},
		NodeInstanceType:        "t3.2xlarge",
		DesiredSize:             3,
		MinSize:                 3,
		MaxSize:                 3,
		MaxPods:                 58,
		VolumeSize:              20,
		VolumeType:              "gp3",
		AdditionalNodeGroups: map[string]steps_aws.EKSNodeGroup{
			"observability": {
				InstanceType: "t3.2xlarge",
				DesiredSize:  1,
				MinSize:      1,
				MaxSize:      1,
				VolumeSize:   20,
				VolumeType:   "gp3",
				Taints: map[string]steps_aws.EKSTaint{
					"node.kubernetes.io/custom-rule": {
						Value:  "observability",
						Effect: "NO_SCHEDULE",
					},
				},
				Labels: map[string]string{
					"node.kubernetes.io/custom-rule": "observability",
				},
			},
		},
		EnableCacheRegistry: true,
		CacheRegistry:       "test-cache-registry",
		HTTPProxy:           "",
		HTTPSProxy:          "",
		NoProxy:             "",
	}

	expectedBackendConfig := steps_aws.TerraformAWSBucketBackendConfig{
		Region: "us-west-2",
		Bucket: "test-" + s.randomText,
		Key:    "eks.tfstate",
	}

	s.tfUtility.On("Run", mock.Anything, steps.TerraformUtilityInput{
		Action:             action,
		ModulePath:         filepath.Join(s.step.RootPath, steps_aws.EKSModulePath),
		Variables:          expectedVariables,
		BackendConfig:      expectedBackendConfig,
		LogFile:            filepath.Join(s.runtimeState.LogDir, "aws_eks.log"),
		KeepGeneratedFiles: true,
	}).Return(steps.TerraformUtilityOutput{}, nil).Once()
}
