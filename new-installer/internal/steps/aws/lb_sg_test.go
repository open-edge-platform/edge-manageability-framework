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

type LBSGStepTest struct {
	suite.Suite
	config       config.OrchInstallerConfig
	runtimeState config.OrchInstallerRuntimeState
	step         *steps_aws.LBSGStep
	randomText   string
	logDir       string
	tfUtility    *MockTerraformUtility
	awsUtility   *MockAWSUtility
}

func TestLBSGStep(t *testing.T) {
	suite.Run(t, new(LBSGStepTest))
}

func (s *LBSGStepTest) SetupTest() {
	rootPath, err := filepath.Abs("../../../../")
	s.Require().NoError(err, "Failed to get absolute path")
	s.randomText = strings.ToLower(rand.Text()[0:8])
	s.logDir = filepath.Join(rootPath, ".logs")
	err = internal.InitLogger("debug", s.logDir)
	s.Require().NoError(err, "Failed to initialize logger")
	s.config.AWS.Region = "us-west-2"
	s.config.Global.OrchName = "lbsg-test"
	s.config.AWS.CustomerTag = "test"
	s.runtimeState.LogDir = filepath.Join(rootPath, ".logs")
	s.runtimeState.AWS.EKSNodeSecurityGroupID = "sg-1234567890abcdef0"
	s.runtimeState.AWS.TraefikSecurityGroupID = "sg-abcdef01234567890"
	s.runtimeState.AWS.Traefik2SecurityGroupID = "sg-abcdef01234567891"
	s.runtimeState.AWS.ArgoCDSecurityGroupID = "sg-abcdef01234567892"
	s.tfUtility = &MockTerraformUtility{}
	s.awsUtility = &MockAWSUtility{}
	s.step = &steps_aws.LBSGStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: true,
		TerraformUtility:   s.tfUtility,
		AWSUtility:         s.awsUtility,
	}
}

func (s *LBSGStepTest) TestInstallAndUninstallLBSG() {
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

func (s *LBSGStepTest) TestUpgradeLBSG() {
	s.runtimeState.Action = "upgrade"
	s.config.AWS.PreviousS3StateBucket = "old-bucket-name"
	// We will mostlly test if the prestep make correct calls to AWS and Terraform utilities.
	s.expectTFUtiliyCall("upgrade")
	_, err := steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}
}

func (s *LBSGStepTest) expectTFUtiliyCall(action string) {
	input := steps.TerraformUtilityInput{
		Action:             action,
		ModulePath:         filepath.Join(s.step.RootPath, steps_aws.LBSGModulePath),
		LogFile:            filepath.Join(s.logDir, "aws_lbsg.log"),
		KeepGeneratedFiles: s.step.KeepGeneratedFiles,
		Variables: steps_aws.LBSGVariables{
			Region:       s.config.AWS.Region,
			CustomerTag:  s.config.AWS.CustomerTag,
			ClusterName:  s.config.Global.OrchName,
			EKSNodeSGID:  "sg-1234567890abcdef0",
			TraefikSGID:  "sg-abcdef01234567890",
			Traefik2SGID: "sg-abcdef01234567891",
			ArgoCDSGID:   "sg-abcdef01234567892",
		},
		BackendConfig: steps.TerraformAWSBucketBackendConfig{
			Region: s.config.AWS.Region,
			Bucket: fmt.Sprintf("%s-%s", s.config.Global.OrchName, s.runtimeState.DeploymentID),
			Key:    "lbsg.tfstate",
		},
		TerraformState: "",
	}
	if action == "install" {
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
			fmt.Sprintf("%s/orch-load-balancer/%s", s.config.AWS.Region, s.config.Global.OrchName),
			s.config.AWS.Region,
			s.config.Global.OrchName+"-"+s.runtimeState.DeploymentID,
			"lbsg.tfstate",
		).Return(nil).Once()

		input.Action = "uninstall"
		input.DestroyTarget = "module.aws_lb_security_group_roles.aws_security_group_rule.node_sg_rule"
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
}
