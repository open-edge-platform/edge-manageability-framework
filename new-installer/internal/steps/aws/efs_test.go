// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package steps_aws_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	steps_aws "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/aws"
	"github.com/open-edge-platform/edge-manageability-framework/installer/targets/aws/iac/utils"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type EFSStepTest struct {
	suite.Suite
	config       config.OrchInstallerConfig
	runtimeState config.OrchInstallerRuntimeState
	step         *steps_aws.EFSStep
	logDir       string
	tfUtility    *MockTerraformUtility
	awsUtility   *MockAWSUtility
}

func TestEFSStep(t *testing.T) {
	suite.Run(t, new(EFSStepTest))
}

func (s *EFSStepTest) SetupTest() {
	rootPath, err := filepath.Abs("../../../../")
	if err != nil {
		s.NoError(err)
		return
	}
	s.logDir = filepath.Join(rootPath, ".logs")
	internal.InitLogger("debug", s.logDir)
	s.config.AWS.Region = "us-west-2"
	s.config.Global.OrchName = "test"
	s.config.AWS.CustomerTag = utils.DefaultTestCustomerTag
	s.runtimeState.AWS.PrivateSubnetIDs = []string{"subnet-12345678", "subnet-87654321"}
	s.runtimeState.AWS.VPCID = "vpc-12345678"
	s.runtimeState.AWS.EKSOIDCIssuer = "https://oidc.eks.us-west-2.amazonaws.com/id/mocked-issuer"
	s.runtimeState.LogDir = filepath.Join(rootPath, ".logs")

	if _, err := os.Stat(s.logDir); os.IsNotExist(err) {
		err := os.MkdirAll(s.logDir, os.ModePerm)
		if err != nil {
			s.NoError(err)
			return
		}
	}
	s.tfUtility = &MockTerraformUtility{}
	s.awsUtility = &MockAWSUtility{}
	s.step = &steps_aws.EFSStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: true,
		TerraformUtility:   s.tfUtility,
		AWSUtility:         s.awsUtility,
	}
}

func (s *EFSStepTest) TestInstallAndUninstallEFS() {
	s.runtimeState.Action = "install"
	s.expectTFUtiliyCall("install")
	rs, err := steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}
	s.Equal(rs.AWS.EFSFileSystemID, "fs-12345678")

	s.runtimeState.Action = "uninstall"
	s.expectTFUtiliyCall("uninstall")
	_, err = steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
	}
}

func (s *EFSStepTest) TestUpgradeEFS() {
	s.runtimeState.Action = "upgrade"
	s.config.AWS.PreviousS3StateBucket = "old-bucket-name"
	// We will mostlly test if the prestep make correct calls to AWS and Terraform utilities.
	s.expectTFUtiliyCall("upgrade")
	rs, err := steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}
	s.Equal(rs.AWS.EFSFileSystemID, "fs-12345678")
}

func (s *EFSStepTest) expectTFUtiliyCall(action string) {
	input := steps.TerraformUtilityInput{
		Action:             action,
		ModulePath:         filepath.Join(s.step.RootPath, steps_aws.EFSModulePath),
		LogFile:            filepath.Join(s.logDir, "aws_efs.log"),
		KeepGeneratedFiles: s.step.KeepGeneratedFiles,
		Variables: steps_aws.EFSVariables{
			ClusterName:      s.config.Global.OrchName,
			Region:           s.config.AWS.Region,
			CustomerTag:      s.config.AWS.CustomerTag,
			PrivateSubnetIDs: s.runtimeState.AWS.PrivateSubnetIDs,
			VPCID:            s.runtimeState.AWS.VPCID,
			EKSOIDCIssuer:    s.runtimeState.AWS.EKSOIDCIssuer,
		},
		BackendConfig: steps_aws.TerraformAWSBucketBackendConfig{
			Region: s.config.AWS.Region,
			Bucket: fmt.Sprintf("%s-%s", s.config.Global.OrchName, s.runtimeState.DeploymentID),
			Key:    "efs.tfstate",
		},
		TerraformState: "",
	}
	if action == "install" {
		s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
			TerraformState: "",
			Output: map[string]tfexec.OutputMeta{
				"efs_id": {
					Type:  json.RawMessage(`"string"`),
					Value: json.RawMessage(`"fs-12345678"`),
				},
			},
		}, nil).Once()
	}
	if action == "upgrade" {
		s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
			TerraformState: "",
			Output: map[string]tfexec.OutputMeta{
				"efs_id": {
					Type:  json.RawMessage(`"string"`),
					Value: json.RawMessage(`"fs-12345678"`),
				},
			},
		}, nil).Once()

		s.awsUtility.On("S3CopyToS3",
			s.config.AWS.Region,
			s.config.AWS.PreviousS3StateBucket,
			fmt.Sprintf("%s/cluster/%s", s.config.AWS.Region, s.config.Global.OrchName),
			s.config.AWS.Region,
			s.config.Global.OrchName+"-"+s.runtimeState.DeploymentID,
			"efs.tfstate",
		).Return(nil).Once()

		s.tfUtility.On("MoveStates", mock.Anything, steps.TerraformUtilityMoveStatesInput{
			ModulePath: filepath.Join(s.step.RootPath, steps_aws.EFSModulePath),
			States: map[string]string{
				"module.efs.aws_efs_file_system.efs": "aws_efs_file_system.efs",
			},
		}).Return(nil).Once()

		s.tfUtility.On("RemoveStates", mock.Anything, steps.TerraformUtilityRemoveStatesInput{
			ModulePath: filepath.Join(s.step.RootPath, steps_aws.EFSModulePath),
			States: []string{
				"module.s3",
				"module.eks",
				"module.aurora",
				"module.aurora_database",
				"module.aurora_import",
				"module.kms",
				"module.orch_init",
				"module.eks_auth",
				"module.ec2log",
				"module.aws_lb_controller",
				"module.gitea",
			},
		}).Return(nil).Once()
	}
	if action == "uninstall" {
		s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
			TerraformState: "",
			Output:         map[string]tfexec.OutputMeta{},
		}, nil).Once()
	}
}
