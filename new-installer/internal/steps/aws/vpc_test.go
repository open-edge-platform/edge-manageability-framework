// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package steps_aws_test

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
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

type VPCStepTest struct {
	suite.Suite
	config       config.OrchInstallerConfig
	runtimeState config.OrchInstallerRuntimeState
	step         *steps_aws.AWSVPCStep
	randomText   string
	logDir       string
	tfUtility    *MockTerraformUtility
	awsUtility   *MockAWSUtility
}

func TestVPCStep(t *testing.T) {
	suite.Run(t, new(VPCStepTest))
}

func (s *VPCStepTest) SetupTest() {
	rootPath, err := filepath.Abs("../../../../")
	if err != nil {
		s.NoError(err)
		return
	}
	s.randomText = strings.ToLower(rand.Text()[0:8])
	s.logDir = filepath.Join(rootPath, ".logs")
	internal.InitLogger("debug", s.logDir)
	s.config.AWS.Region = "us-west-2"
	s.config.Global.OrchName = "test"
	s.runtimeState.DeploymentID = s.randomText
	s.config.AWS.JumpHostWhitelist = []string{"10.250.0.0/16"}
	s.runtimeState.LogDir = filepath.Join(rootPath, ".logs")
	s.runtimeState.JumpHostSSHKeyPrivateKey = "foobar"
	s.runtimeState.JumpHostSSHKeyPublicKey = "foobar"

	if _, err := os.Stat(s.logDir); os.IsNotExist(err) {
		err := os.MkdirAll(s.logDir, os.ModePerm)
		if err != nil {
			s.NoError(err)
			return
		}
	}
	s.tfUtility = &MockTerraformUtility{}
	s.awsUtility = &MockAWSUtility{}
	s.step = &steps_aws.AWSVPCStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: true,
		TerraformUtility:   s.tfUtility,
		AWSUtility:         s.awsUtility,
	}
}

func (s *VPCStepTest) TestInstallAndUninstallVPC() {
	s.runtimeState.Action = "install"
	s.expectTFUtiliyyCall("install")
	rs, err := steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}

	s.Equal(rs.VPCID, "vpc-12345678")
	s.ElementsMatch([]string{
		"subnet-1",
		"subnet-2",
		"subnet-3",
	}, rs.PrivateSubnetIDs)
	s.ElementsMatch([]string{
		"subnet-4",
		"subnet-5",
		"subnet-6",
	}, rs.PublicSubnetIDs)

	s.runtimeState.Action = "uninstall"
	s.expectTFUtiliyyCall("uninstall")
	_, err = steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
	}
}

func (s *VPCStepTest) expectTFUtiliyyCall(action string) {
	s.awsUtility.On("GetAvailableZones", "us-west-2").Return([]string{"us-west-2a", "us-west-2b", "us-west-2c"}, nil).Once()
	input := steps.TerraformUtilityInput{
		Action:             action,
		ModulePath:         filepath.Join(s.step.RootPath, steps_aws.VPCModulePath),
		LogFile:            filepath.Join(s.logDir, "aws_vpc.log"),
		KeepGeneratedFiles: s.step.KeepGeneratedFiles,
		Variables: steps_aws.AWSVPCVariables{
			Name:               s.config.Global.OrchName,
			Region:             s.config.AWS.Region,
			CidrBlock:          "10.250.0.0/16",
			EnableDnsHostnames: true,
			EnableDnsSupport:   true,
			JumphostIPAllowList: []string{
				"10.250.0.0/16",
			},
			JumphostInstanceSshKey: "foobar",
			Production:             true,
			CustomerTag:            "",
			EndpointSGName:         s.config.Global.OrchName + "-vpc-ep",
			PrivateSubnets: map[string]steps_aws.AWSVPCSubnet{
				"subnet-us-west-2a": {
					Az:        "us-west-2a",
					CidrBlock: "10.250.0.0/22",
				},
				"subnet-us-west-2b": {
					Az:        "us-west-2b",
					CidrBlock: "10.250.4.0/22",
				},
				"subnet-us-west-2c": {
					Az:        "us-west-2c",
					CidrBlock: "10.250.8.0/22",
				},
			},
			PublicSubnets: map[string]steps_aws.AWSVPCSubnet{
				"subnet-us-west-2a-pub": {
					Az:        "us-west-2a",
					CidrBlock: "10.250.12.0/24",
				},
				"subnet-us-west-2b-pub": {
					Az:        "us-west-2b",
					CidrBlock: "10.250.13.0/24",
				},
				"subnet-us-west-2c-pub": {
					Az:        "us-west-2c",
					CidrBlock: "10.250.14.0/24",
				},
			},
			JumphostSubnet: "subnet-us-west-2a-pub",
		},
		BackendConfig: steps_aws.TerraformAWSBucketBackendConfig{
			Region: s.config.AWS.Region,
			Bucket: fmt.Sprintf("%s-%s", s.config.Global.OrchName, s.runtimeState.DeploymentID),
			Key:    "vpc.tfstate",
		},
		TerraformState: "",
	}
	if action == "install" {
		s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
			TerraformState: "",
			Output: map[string]tfexec.OutputMeta{
				"vpc_id": {
					Type:  json.RawMessage(`"string"`),
					Value: json.RawMessage(`"vpc-12345678"`),
				},
				"private_subnets": {
					Type:  json.RawMessage(`"list"`),
					Value: json.RawMessage(`{"subnet-us-west-2a":{"id":"subnet-1"},"subnet-us-west-2b":{"id":"subnet-2"},"subnet-us-west-2c":{"id":"subnet-3"}}`),
				},
				"public_subnets": {
					Type:  json.RawMessage(`"list"`),
					Value: json.RawMessage(`{"subnet-us-west-2a-pub":{"id":"subnet-4"},"subnet-us-west-2b-pub":{"id":"subnet-5"},"subnet-us-west-2c-pub":{"id":"subnet-6"}}`),
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
