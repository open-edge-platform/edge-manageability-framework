// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package steps_aws_test

import (
	"crypto/rand"
	"fmt"
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

const (
	DefaultCustomerTag   = "default-customer-tag"
	DefaultNodeGroupRole = "default-node-group-role"
	DefaultS3Prefix      = "default-s3-prefix"
)

type EC2LogStepTest struct {
	suite.Suite
	config       config.OrchInstallerConfig
	runtimeState config.OrchInstallerRuntimeState
	step         *steps_aws.EC2LogStep
	randomText   string
	logDir       string
	tfUtility    *MockTerraformUtility
	awsUtility   *MockAWSUtility
}

func TestEC2LogStep(t *testing.T) {
	suite.Run(t, new(EC2LogStepTest))
}

func (s *EC2LogStepTest) SetupTest() {
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
	s.runtimeState.AWS.NodeGroupRole = DefaultNodeGroupRole
	s.config.AWS.CustomerTag = DefaultCustomerTag
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
	s.step = &steps_aws.EC2LogStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: true,
		TerraformUtility:   s.tfUtility,
		AWSUtility:         s.awsUtility,
	}
}

func (s *EC2LogStepTest) TestInstallAndUninstallEC2Log() {
	s.runtimeState.Action = "install"
	s.expectUtiliyCall("install")
	_, err := steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}

	s.runtimeState.Action = "uninstall"
	s.expectUtiliyCall("uninstall")
	_, err = steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
	}
}

func (s *EC2LogStepTest) TestUpgradeEC2Log() {
	s.runtimeState.Action = "upgrade"
	s.config.AWS.PreviousS3StateBucket = "old-bucket-name"
	// We will mostlly test if the prestep make correct calls to AWS and Terraform utilities.
	s.expectUtiliyCall("upgrade")
	_, err := steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}
}

func (s *EC2LogStepTest) expectUtiliyCall(action string) {
	input := steps.TerraformUtilityInput{
		Action:             action,
		ModulePath:         filepath.Join(s.step.RootPath, steps_aws.EC2LogModulePath),
		LogFile:            filepath.Join(s.logDir, "aws_ec2log.log"),
		KeepGeneratedFiles: s.step.KeepGeneratedFiles,
		Variables: steps_aws.EC2logVariables{
			ClusterName:   s.config.Global.OrchName,
			Region:        s.config.AWS.Region,
			NodeGroupRole: s.runtimeState.AWS.NodeGroupRole,
			S3Prefix:      s.runtimeState.DeploymentID,
			CustomerTag:   s.config.AWS.CustomerTag,
		},
		BackendConfig: steps_aws.TerraformAWSBucketBackendConfig{
			Region: s.config.AWS.Region,
			Bucket: fmt.Sprintf("%s-%s", s.config.Global.OrchName, s.runtimeState.DeploymentID),
			Key:    "ec2log.tfstate",
		},
		TerraformState: "",
	}
	s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
		TerraformState: "",
	}, nil).Once()
	if action == "install" {
		s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
			TerraformState: "",
		}, nil).Once()
	}
	if action == "upgrade" {
		s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
			TerraformState: "",
		}, nil).Once()

		s.awsUtility.On("S3CopyToS3",
			s.config.AWS.Region,
			s.config.AWS.PreviousS3StateBucket,
			fmt.Sprintf("%s/ec2log/%s", s.config.AWS.Region, s.config.Global.OrchName),
			s.config.AWS.Region,
			s.config.Global.OrchName+"-"+s.runtimeState.DeploymentID,
			"ec2log.tfstate",
		).Return(nil).Once()

		s.tfUtility.On("MoveStates", mock.Anything, steps.TerraformUtilityMoveStatesInput{
			ModulePath: filepath.Join(s.step.RootPath, steps_aws.EC2LogModulePath),
			States: map[string]string{
				"module.ec2log.aws_ssm_document.push_log":                                    "aws_ssm_document.push_log",
				"module.ec2log.local_file.ssm_term_src":                                      "local_file.ssm_term_src",
				"module.ec2log.aws_lambda_function.ssm_term":                                 "aws_lambda_function.ssm_term",
				"module.ec2log.aws_cloudwatch_log_group.ssm_term":                            "aws_cloudwatch_log_group.ssm_term",
				"module.ec2log.aws_lambda_permission.allow_cloudwatch":                       "aws_lambda_permission.allow_cloudwatch",
				"module.ec2log.aws_s3_bucket.logs":                                           "aws_s3_bucket.logs",
				"module.ec2log.aws_iam_policy.cloudwatch":                                    "aws_iam_policy.cloudwatch",
				"module.ec2log.aws_iam_policy.s3":                                            "aws_iam_policy.s3",
				"module.ec2log.aws_iam_policy.ssm":                                           "aws_iam_policy.ssm",
				"module.ec2log.aws_iam_role_policy_attachment.ec2_cloudwatch":                "aws_iam_role_policy_attachment.ec2_cloudwatch",
				"module.ec2log.aws_iam_role_policy_attachment.ec2_s3":                        "aws_iam_role_policy_attachment.ec2_s3",
				"module.ec2log.aws_iam_role.lambda":                                          "aws_iam_role.lambda",
				"module.ec2log.aws_iam_role_policy_attachment.lambda_cloudwatch":             "aws_iam_role_policy_attachment.lambda_cloudwatch",
				"module.ec2log.aws_iam_role_policy_attachment.lambda_ssm":                    "aws_iam_role_policy_attachment.lambda_ssm",
				"module.ec2log.aws_autoscaling_lifecycle_hook.nodegroup1-terminating":        "aws_autoscaling_lifecycle_hook.nodegroup1-terminating",
				"module.ec2log.aws_autoscaling_lifecycle_hook.observability-terminating":     "aws_autoscaling_lifecycle_hook.observability-terminating",
				"module.ec2log.aws_cloudwatch_event_rule.instance_terminate":                 "aws_cloudwatch_event_rule.instance_terminate",
				"module.ec2log.aws_cloudwatch_event_target.lambda_runssm_instance_terminate": "aws_cloudwatch_event_target.lambda_runssm_instance_terminate",
			},
		}).Return(nil).Once()
		s.tfUtility.On("RemoveStates", mock.Anything, steps.TerraformUtilityRemoveStatesInput{
			ModulePath: filepath.Join(s.step.RootPath, steps_aws.EC2LogModulePath),
			States: []string{
				"module.s3",
				"module.eks",
				"module.aurora",
				"module.aurora_database",
				"module.aurora_import",
				"module.kms",
				"module.orch_init",
				"module.eks_auth",
				"module.efs",
				"module.aws_lb_controller",
				"module.gitea",
			},
		}).Return(nil).Once()
	}
	if action == "uninstall" {
		s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
			TerraformState: "",
		}, nil).Once()
	}
}
