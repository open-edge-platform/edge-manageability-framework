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
	TestTraefikDNSName            = "traefik.example.com"
	TestInfraDNSName              = "infra.example.com"
	TestTraefikTargetGroupARN     = "arn:aws:elasticloadbalancing:us-west-2:123456789012:targetgroup/traefik/12345678"
	TestTraefikGRPCTargetGroupARN = "arn:aws:elasticloadbalancing:us-west-2:123456789012:targetgroup/traefik-grpc/87654321"
	TestInfraArgoCDTargetGroupARN = "arn:aws:elasticloadbalancing:us-west-2:123456789012:targetgroup/argocd/23456789"
	TestInfraGiteaTargetGroupARN  = "arn:aws:elasticloadbalancing:us-west-2:123456789012:targetgroup/gitea/34567890"
	TestTraefikLBARN              = "arn:aws:elasticloadbalancing:us-west-2:123456789012:loadbalancer/app/traefik/12345678"
	TestInfraLBARN                = "arn:aws:elasticloadbalancing:us-west-2:123456789012:loadbalancer/app/infra/87654321"
)

type ALBStepTestSuite struct {
	suite.Suite
	config       config.OrchInstallerConfig
	runtimeState config.OrchInstallerRuntimeState
	step         *steps_aws.ALBStep
	logDir       string
	tfUtility    *MockTerraformUtility
	awsUtility   *MockAWSUtility
}

func TestALBStepSuite(t *testing.T) {
	suite.Run(t, new(ALBStepTestSuite))
}

func (s *ALBStepTestSuite) SetupTest() {
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
	s.step = &steps_aws.ALBStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: false,
		TerraformUtility:   s.tfUtility,
		AWSUtility:         s.awsUtility,
	}
}

func (s *ALBStepTestSuite) TestInstallAndUninstallALB() {
	s.runtimeState.Action = "install"
	s.expectUtilityCalls("install")
	rs, err := steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}

	s.Equal(TestTraefikDNSName, rs.AWS.TraefikDNSName)
	s.Equal(TestInfraDNSName, rs.AWS.InfraDNSName)
	s.Equal(TestTraefikTargetGroupARN, rs.AWS.TraefikTargetGroupARN)
	s.Equal(TestTraefikGRPCTargetGroupARN, rs.AWS.TraefikGRPCTargetGroupARN)
	s.Equal(TestInfraArgoCDTargetGroupARN, rs.AWS.InfraArgoCDTargetGroupARN)
	s.Equal(TestInfraGiteaTargetGroupARN, rs.AWS.InfraGiteaTargetGroupARN)
	s.Equal(TestTraefikLBARN, rs.AWS.TraefikLBARN)
	s.Equal(TestInfraLBARN, rs.AWS.InfraLBARN)
	s.runtimeState.Action = "uninstall"
	s.runtimeState.AWS.TraefikLBARN = TestTraefikLBARN
	s.runtimeState.AWS.InfraLBARN = TestInfraLBARN
	s.expectUtilityCalls("uninstall")
	_, err = steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
	}
}

func (s *ALBStepTestSuite) TestUpgradeALB() {
	s.runtimeState.Action = "upgrade"
	s.config.AWS.PreviousS3StateBucket = "old-bucket-name"
	// We will mostlly test if the prestep make correct calls to AWS and Terraform utilities.
	s.expectUtilityCalls("upgrade")
	rs, err := steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}
	s.Equal(TestTraefikDNSName, rs.AWS.TraefikDNSName)
	s.Equal(TestInfraDNSName, rs.AWS.InfraDNSName)
	s.Equal(TestTraefikTargetGroupARN, rs.AWS.TraefikTargetGroupARN)
	s.Equal(TestTraefikGRPCTargetGroupARN, rs.AWS.TraefikGRPCTargetGroupARN)
	s.Equal(TestInfraArgoCDTargetGroupARN, rs.AWS.InfraArgoCDTargetGroupARN)
	s.Equal(TestInfraGiteaTargetGroupARN, rs.AWS.InfraGiteaTargetGroupARN)
	s.Equal(TestTraefikLBARN, rs.AWS.TraefikLBARN)
	s.Equal(TestInfraLBARN, rs.AWS.InfraLBARN)
}

func (s *ALBStepTestSuite) expectUtilityCalls(action string) {
	input := steps.TerraformUtilityInput{
		Action:             action,
		ModulePath:         filepath.Join(s.step.RootPath, steps_aws.ALBModulePath),
		LogFile:            filepath.Join(s.logDir, "aws_alb.log"),
		KeepGeneratedFiles: s.step.KeepGeneratedFiles,
		Variables: steps_aws.ALBVariables{
			Internal:                 s.config.AWS.VPCID != "",
			VPCID:                    s.runtimeState.AWS.VPCID,
			ClusterName:              s.config.Global.OrchName,
			PublicSubnetIDs:          s.runtimeState.AWS.PublicSubnetIDs,
			IPAllowList:              s.config.AWS.LoadBalancerAllowList,
			EnableDeletionProtection: s.config.AWS.EnableLBDeletionProtection,
			TLSCertARN:               s.runtimeState.AWS.ACMCertARN,
			Region:                   s.config.AWS.Region,
			CustomerTag:              s.config.AWS.CustomerTag,
		},
		BackendConfig: steps_aws.TerraformAWSBucketBackendConfig{
			Region: s.config.AWS.Region,
			Bucket: fmt.Sprintf("%s-%s", s.config.Global.OrchName, s.runtimeState.DeploymentID),
			Key:    "alb.tfstate",
		},
		TerraformState: "",
	}
	output := map[string]tfexec.OutputMeta{
		"traefik_lb_arn": {
			Type:  json.RawMessage(`"string"`),
			Value: json.RawMessage(fmt.Sprintf(`"%s"`, TestTraefikLBARN)),
		},
		"infra_lb_arn": {
			Type:  json.RawMessage(`"string"`),
			Value: json.RawMessage(fmt.Sprintf(`"%s"`, TestInfraLBARN)),
		},
		"traefik_target_group_arn": {
			Type:  json.RawMessage(`"string"`),
			Value: json.RawMessage(fmt.Sprintf(`"%s"`, TestTraefikTargetGroupARN)),
		},
		"traefik_grpc_target_group_arn": {
			Type:  json.RawMessage(`"string"`),
			Value: json.RawMessage(fmt.Sprintf(`"%s"`, TestTraefikGRPCTargetGroupARN)),
		},
		"infra_argocd_target_group_arn": {
			Type:  json.RawMessage(`"string"`),
			Value: json.RawMessage(fmt.Sprintf(`"%s"`, TestInfraArgoCDTargetGroupARN)),
		},
		"infra_gitea_target_group_arn": {
			Type:  json.RawMessage(`"string"`),
			Value: json.RawMessage(fmt.Sprintf(`"%s"`, TestInfraGiteaTargetGroupARN)),
		},
		"traefik_dns_name": {
			Type:  json.RawMessage(`"string"`),
			Value: json.RawMessage(fmt.Sprintf(`"%s"`, TestTraefikDNSName)),
		},
		"infra_dns_name": {
			Type:  json.RawMessage(`"string"`),
			Value: json.RawMessage(fmt.Sprintf(`"%s"`, TestInfraDNSName)),
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
			"alb.tfstate",
		).Return(nil).Once()

		s.tfUtility.On("MoveStates", mock.Anything, steps.TerraformUtilityMoveStatesInput{
			ModulePath: filepath.Join(s.step.RootPath, steps_aws.ALBModulePath),
			States: map[string]string{
				// Traefik
				"module.traefik_load_balancer.aws_security_group.common":                    "aws_security_group.traefik",
				"module.traefik_load_balancer.aws_lb.main":                                  "aws_lb.traefik",
				"module.traefik_load_balancer.aws_lb_target_group.main[\"default\"]":        "aws_lb_target_group.traefik",
				"module.traefik_load_balancer.aws_lb_target_group.main[\"grpc\"]":           "aws_lb_target_group.traefik_grpc",
				"module.traefik_load_balancer.aws_lb_listener.main":                         "aws_lb_listener.traefik",
				"module.traefik_load_balancer.aws_lb_listener_rule.match_headers[\"grpc\"]": "aws_lb_listener_rule.traefik_grpc",
				// ArgoCD and Gitea
				"module.argocd_load_balancer.aws_security_group.common":                    "aws_security_group.infra",
				"module.argocd_load_balancer.aws_lb.main":                                  "aws_lb.infra",
				"module.argocd_load_balancer.aws_lb_target_group.main[\"argocd\"]":         "aws_lb_target_group.infra_argocd",
				"module.argocd_load_balancer.aws_lb_target_group.main[\"gitea\"]":          "aws_lb_target_group.infra_gitea",
				"module.argocd_load_balancer.aws_lb_listener.main":                         "aws_lb_listener.infra",
				"module.argocd_load_balancer.aws_lb_listener_rule.match_hosts[\"argocd\"]": "aws_lb_listener_rule.infra_argocd",
				"module.argocd_load_balancer.aws_lb_listener_rule.match_hosts[\"gitea\"]":  "aws_lb_listener_rule.infra_gitea",
			},
		}).Return(nil).Once()

		s.tfUtility.On("RemoveStates", mock.Anything, steps.TerraformUtilityRemoveStatesInput{
			ModulePath: filepath.Join(s.step.RootPath, steps_aws.ALBModulePath),
			States: []string{
				"module.traefik2_load_balancer",
				"module.traefik_lb_target_group_binding",
				"module.aws_lb_security_group_roles",
				"module.wait_until_alb_ready",
				"module.waf_web_acl_traefik",
				"module.waf_web_acl_argocd",
			},
		}).Return(nil).Once()
	}
	if action == "uninstall" {
		s.awsUtility.On("DisableLBDeletionProtection", utils.DefaultTestRegion, TestTraefikLBARN).Return(nil).Once()
		s.awsUtility.On("DisableLBDeletionProtection", utils.DefaultTestRegion, TestInfraLBARN).Return(nil).Once()
		s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
			TerraformState: "",
			Output:         map[string]tfexec.OutputMeta{},
		}, nil).Once()
	}
}
