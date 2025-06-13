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

const (
	TestNLBDNSName        = "test-nlb.example.com"
	TestNLBTargetGroupARN = "arn:aws:elasticloadbalancing:us-west-2:123456789012:targetgroup/test-nlb-target-group/1234567890abcdef"
	TestNLBARN            = "arn:aws:elasticloadbalancing:us-west-2:123456789012:loadbalancer/net/test-nlb/1234567890abcdef"
)

type NLBStepTestSuite struct {
	suite.Suite
	config       config.OrchInstallerConfig
	runtimeState config.OrchInstallerRuntimeState
	step         *steps_aws.NLBStep
	logDir       string
	tfUtility    *MockTerraformUtility
	awsUtility   *MockAWSUtility
}

func TestNLBStepSuite(t *testing.T) {
	suite.Run(t, new(NLBStepTestSuite))
}

func (s *NLBStepTestSuite) SetupTest() {
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
	s.runtimeState.AWS.PublicSubnetIDs = []string{"subnet-12345678", "subnet-87654321"}
	s.runtimeState.AWS.VPCID = "vpc-12345678"
	s.config.Global.OrchName = "test"
	s.config.AWS.LoadBalancerAllowList = []string{"10.0.0.0/8"}
	s.runtimeState.AWS.ACMCertARN = "arn:aws:acm:us-west-2:123456789012:certificate/12345678-1234-1234-1234-123456789012"
	s.config.AWS.Region = utils.DefaultTestRegion
	s.config.AWS.CustomerTag = utils.DefaultTestCustomerTag
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
	s.step = &steps_aws.NLBStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: false,
		TerraformUtility:   s.tfUtility,
		AWSUtility:         s.awsUtility,
	}
}

func (s *NLBStepTestSuite) TestInstallAndUninstallNLB() {
	s.runtimeState.Action = "install"
	s.expectUtilityCalls("install")
	_, err := steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}

	s.runtimeState.Action = "uninstall"
	s.runtimeState.AWS.NLBARN = TestNLBARN
	s.expectUtilityCalls("uninstall")
	_, err = steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
	}
}

func (s *NLBStepTestSuite) TestUpgradeNLB() {
	s.runtimeState.Action = "upgrade"
	s.config.AWS.PreviousS3StateBucket = "old-bucket-name"
	// We will mostlly test if the prestep make correct calls to AWS and Terraform utilities.
	s.expectUtilityCalls("upgrade")
	_, err := steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}
}

func (s *NLBStepTestSuite) expectUtilityCalls(action string) {
	input := steps.TerraformUtilityInput{
		Action:             action,
		ModulePath:         filepath.Join(s.step.RootPath, steps_aws.NLBModulePath),
		LogFile:            filepath.Join(s.logDir, "aws_nlb.log"),
		KeepGeneratedFiles: s.step.KeepGeneratedFiles,
		Variables: steps_aws.NLBVariables{
			Internal:                 false,
			VPCID:                    s.runtimeState.AWS.VPCID,
			ClusterName:              s.config.Global.OrchName,
			SubnetIDs:                s.runtimeState.AWS.PublicSubnetIDs,
			IPAllowList:              s.config.AWS.LoadBalancerAllowList,
			EnableDeletionProtection: s.config.AWS.EnableLBDeletionProtection,
			Region:                   s.config.AWS.Region,
			CustomerTag:              s.config.AWS.CustomerTag,
		},
		BackendConfig: steps_aws.TerraformAWSBucketBackendConfig{
			Region: s.config.AWS.Region,
			Bucket: fmt.Sprintf("%s-%s", s.config.Global.OrchName, s.runtimeState.DeploymentID),
			Key:    "nlb.tfstate",
		},
		TerraformState: "",
	}
	output := map[string]tfexec.OutputMeta{
		"nlb_dns_name": {
			Type:  json.RawMessage(`"string"`),
			Value: json.RawMessage(fmt.Sprintf(`"%s"`, TestNLBDNSName)),
		},
		"nlb_target_group_arn": {
			Type:  json.RawMessage(`"string"`),
			Value: json.RawMessage(fmt.Sprintf(`"%s"`, TestNLBTargetGroupARN)),
		},
		"nlb_arn": {
			Type:  json.RawMessage(`"string"`),
			Value: json.RawMessage(fmt.Sprintf(`"%s"`, TestNLBARN)),
		},
	}
	if action == "install" {
		s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
			TerraformState: "",
			Output:         output,
		}, nil).Once()
	}
	if action == "upgrade" {
		s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
			TerraformState: "",
			Output:         output,
		}, nil).Once()

		s.awsUtility.On("S3CopyToS3",
			s.config.AWS.Region,
			s.config.AWS.PreviousS3StateBucket,
			fmt.Sprintf("%s/orch-load-balancer/%s", s.config.AWS.Region, s.config.Global.OrchName),
			s.config.AWS.Region,
			s.config.Global.OrchName+"-"+s.runtimeState.DeploymentID,
			"nlb.tfstate",
		).Return(nil).Once()

		s.tfUtility.On("MoveStates", mock.Anything, steps.TerraformUtilityMoveStatesInput{
			ModulePath: filepath.Join(s.step.RootPath, steps_aws.NLBModulePath),
			States: map[string]string{
				"module.traefik2_load_balancer.aws_eip.main[\"subnet-12345678\"]":   "aws_eip.main[\"subnet-12345678\"]",
				"module.traefik2_load_balancer.aws_eip.main[\"subnet-87654321\"]":   "aws_eip.main[\"subnet-87654321\"]",
				"module.traefik2_load_balancer.aws_lb.main":                         "aws_lb.main",
				"module.traefik2_load_balancer.aws_lb_listener.main[\"https\"]":     "aws_lb_listener.main",
				"module.traefik2_load_balancer.aws_lb_target_group.main[\"https\"]": "aws_lb_target_group.main",
				"module.traefik2_load_balancer.aws_security_group.common":           "aws_security_group.common",
			},
		}).Return(nil).Once()

		s.tfUtility.On("RemoveStates", mock.Anything, steps.TerraformUtilityRemoveStatesInput{
			ModulePath: filepath.Join(s.step.RootPath, steps_aws.NLBModulePath),
			States: []string{
				"module.traefik_load_balancer",
				"module.argocd_load_balancer",
				"module.traefik_lb_target_group_binding",
				"module.aws_lb_security_group_roles",
				"module.wait_until_alb_ready",
				"module.waf_web_acl_traefik",
				"module.waf_web_acl_argocd",
			},
		}).Return(nil).Once()
	}
	if action == "uninstall" {
		s.awsUtility.On("DisableLBDeletionProtection", utils.DefaultTestRegion, TestNLBARN).Return(nil).Once()
		s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
			TerraformState: "",
			Output:         map[string]tfexec.OutputMeta{},
		}, nil).Once()
	}
}
