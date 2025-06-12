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

var availabilityZones = []string{"us-west-2a", "us-west-2b", "us-west-2c"}

type RDSStepTest struct {
	suite.Suite
	config       config.OrchInstallerConfig
	runtimeState config.OrchInstallerRuntimeState
	step         *steps_aws.RDSStep
	logDir       string
	tfUtility    *MockTerraformUtility
	awsUtility   *MockAWSUtility
}

func TestRDSStep(t *testing.T) {
	suite.Run(t, new(RDSStepTest))
}

func (s *RDSStepTest) SetupTest() {
	rootPath, err := filepath.Abs("../../../../")
	if err != nil {
		s.NoError(err)
		return
	}
	s.logDir = filepath.Join(rootPath, ".logs")
	if err := internal.InitLogger("debug", s.logDir); err != nil {
		s.NoError(err)
		return
	}

	s.config.AWS.Region = "us-west-2"
	s.config.Global.OrchName = "test"
	s.config.Global.Scale = config.Scale100
	s.config.AWS.CustomerTag = utils.DefaultTestCustomerTag
	s.config.Advanced.DevMode = true
	s.runtimeState.AWS.PrivateSubnetIDs = []string{"subnet-12345678", "subnet-87654321"}
	s.runtimeState.AWS.VPCID = "vpc-12345678"

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
	s.step = &steps_aws.RDSStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: true,
		TerraformUtility:   s.tfUtility,
		AWSUtility:         s.awsUtility,
	}
}

func (s *RDSStepTest) TestInstallAndUninstallRDS() {
	s.runtimeState.Action = "install"
	s.expectTFUtiliyCall("install")
	s.expectAWSUtiliyCall("install")
	rs, err := steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}
	s.NotEmptyf(rs.Database.Host, "database host should not be empty after installation")
	s.NotEmptyf(rs.Database.ReaderHost, "database reader host should not be empty after installation")
	s.NotEmptyf(rs.Database.Port, "database port should not be empty after installation")
	s.NotEmptyf(rs.Database.Username, "database username should not be empty after installation")
	s.NotEmptyf(rs.Database.Password, "database password should not be empty after installation")

	s.runtimeState.Action = "uninstall"
	s.expectTFUtiliyCall("uninstall")
	s.expectAWSUtiliyCall("uninstall")
	_, err = steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
	}
}

func (s *RDSStepTest) TestUpgradeRDS() {
	s.runtimeState.Action = "upgrade"
	s.config.AWS.PreviousS3StateBucket = "old-bucket-name"
	// We will mostlly test if the prestep make correct calls to AWS and Terraform utilities.
	s.expectTFUtiliyCall("upgrade")
	s.expectAWSUtiliyCall("upgrade")
	rs, err := steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}
	s.NotEmptyf(rs.Database.Host, "database host should not be empty after installation")
	s.NotEmptyf(rs.Database.ReaderHost, "database reader host should not be empty after installation")
	s.NotEmptyf(rs.Database.Port, "database port should not be empty after installation")
	s.NotEmptyf(rs.Database.Username, "database username should not be empty after installation")
	s.NotEmptyf(rs.Database.Password, "database password should not be empty after installation")
}

func (s *RDSStepTest) expectTFUtiliyCall(action string) {
	input := steps.TerraformUtilityInput{
		Action:             action,
		ModulePath:         filepath.Join(s.step.RootPath, steps_aws.RDSModulePath),
		LogFile:            filepath.Join(s.logDir, "aws_rds.log"),
		KeepGeneratedFiles: s.step.KeepGeneratedFiles,
		Variables: steps_aws.RDSVariables{
			ClusterName:               s.config.Global.OrchName,
			Region:                    s.config.AWS.Region,
			CustomerTag:               s.config.AWS.CustomerTag,
			SubnetIDs:                 s.runtimeState.AWS.PrivateSubnetIDs,
			VPCID:                     s.runtimeState.AWS.VPCID,
			IPAllowList:               []string{steps_aws.DefaultNetworkCIDR},
			AvailabilityZones:         availabilityZones,
			InstanceAvailabilityZones: availabilityZones,
			PostgresVerMajor:          "",
			PostgresVerMinor:          "",
			MinACUs:                   0.5,
			MaxACUs:                   2,
			DevMode:                   true,
			Username:                  "",
			CACertIdentifier:          "",
		},
		BackendConfig: steps_aws.TerraformAWSBucketBackendConfig{
			Region: s.config.AWS.Region,
			Bucket: fmt.Sprintf("%s-%s", s.config.Global.OrchName, s.runtimeState.DeploymentID),
			Key:    "rds.tfstate",
		},
		TerraformState: "",
	}
	if action == "install" {
		s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
			TerraformState: "",
			Output: map[string]tfexec.OutputMeta{
				"host": {
					Type:  json.RawMessage(`"string"`),
					Value: json.RawMessage(`"rds-instance-12345678.us-west-2.rds.amazonaws.com"`),
				},
				"host_reader": {
					Type:  json.RawMessage(`"string"`),
					Value: json.RawMessage(`"rds-instance-reader-12345678.us-west-2.rds.amazonaws.com"`),
				},
				"port": {
					Type:  json.RawMessage(`"string"`),
					Value: json.RawMessage(`"5432"`),
				},
				"username": {
					Type:  json.RawMessage(`"string"`),
					Value: json.RawMessage(`"postgres"`),
				},
				"password": {
					Type:  json.RawMessage(`"string"`),
					Value: json.RawMessage(`"fakepassword"`),
				},
			},
		}, nil).Once()
	}
	if action == "upgrade" {
		s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
			TerraformState: "",
			Output: map[string]tfexec.OutputMeta{
				"host": {
					Type:  json.RawMessage(`"string"`),
					Value: json.RawMessage(`"rds-instance-12345678.us-west-2.rds.amazonaws.com"`),
				},
				"host_reader": {
					Type:  json.RawMessage(`"string"`),
					Value: json.RawMessage(`"rds-instance-reader-12345678.us-west-2.rds.amazonaws.com"`),
				},
				"port": {
					Type:  json.RawMessage(`"string"`),
					Value: json.RawMessage(`"5432"`),
				},
				"username": {
					Type:  json.RawMessage(`"string"`),
					Value: json.RawMessage(`"postgres"`),
				},
				"password": {
					Type:  json.RawMessage(`"string"`),
					Value: json.RawMessage(`"fakepassword"`),
				},
			},
		}, nil).Once()

		s.awsUtility.On("S3CopyToS3",
			s.config.AWS.Region,
			s.config.AWS.PreviousS3StateBucket,
			fmt.Sprintf("%s/cluster/%s", s.config.AWS.Region, s.config.Global.OrchName),
			s.config.AWS.Region,
			s.config.Global.OrchName+"-"+s.runtimeState.DeploymentID,
			"rds.tfstate",
		).Return(nil).Once()

		s.tfUtility.On("MoveStates", mock.Anything, steps.TerraformUtilityMoveStatesInput{
			ModulePath: filepath.Join(s.step.RootPath, steps_aws.RDSModulePath),
			States: map[string]string{
				"module.aurora.aws_db_subnet_group.main":                "aws_db_subnet_group.main",
				"module.aurora.aws_rds_cluster.main":                    "aws_rds_cluster.main",
				"module.aurora.aws_rds_cluster_instance.main":           "aws_rds_cluster_instance.main",
				"module.aurora.aws_rds_cluster_parameter_group.default": "aws_rds_cluster_parameter_group.default",
				"module.aurora.aws_security_group.rds":                  "aws_security_group.rds",
			},
		}).Return(nil).Once()

		s.tfUtility.On("RemoveStates", mock.Anything, steps.TerraformUtilityRemoveStatesInput{
			ModulePath: filepath.Join(s.step.RootPath, steps_aws.RDSModulePath),
			States: []string{
				"module.s3",
				"module.eks",
				"module.efs",
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

func (s *RDSStepTest) expectAWSUtiliyCall(action string) {
	if action == "install" {
		s.awsUtility.On("GetAvailableZones", s.config.AWS.Region).
			Return(availabilityZones, nil).
			Times(2) // One for install, and one for uninstall
	} else if action == "upgrade" {
		s.awsUtility.On("GetAvailableZones", s.config.AWS.Region).
			Return(availabilityZones, nil).
			Once()
	} else if action == "uninstall" {
		s.awsUtility.On("DisableRDSDeletionProtection",
			s.config.AWS.Region, s.config.Global.OrchName,
		).Return(nil).Once()
	}
}
