// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_lb_sg_test

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	aws_sdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	terratest_aws "github.com/gruntwork-io/terratest/modules/aws"
	"github.com/open-edge-platform/edge-manageability-framework/installer/targets/aws/iac/utils"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/suite"
)

type LBSGTestSuite struct {
	suite.Suite
	name         string
	eksSGID      string
	traefikSGID  string
	traefik2SGID string
	argocdSGID   string
	vpcID        string
}

type LBSGVariables struct {
	Region       string `json:"region" yaml:"region"`
	CustomerTag  string `json:"customer_tag" yaml:"customer_tag"`
	ClusterName  string `json:"cluster_name" yaml:"cluster_name"`
	EKSNodeSGID  string `json:"eks_node_sg_id" yaml:"eks_node_sg_id"`
	TraefikSGID  string `json:"traefik_sg_id" yaml:"traefik_sg_id"`
	Traefik2SGID string `json:"traefik2_sg_id" yaml:"traefik2_sg_id"`
	ArgoCDSGID   string `json:"argocd_sg_id" yaml:"argocd_sg_id"`
}

func TestLBSGTestSuite(t *testing.T) {
	suite.Run(t, new(LBSGTestSuite))
}

func (s *LBSGTestSuite) SetupTest() {
	randomPostfix := strings.ToLower(rand.Text()[:8])
	s.name = "lbsg-test-" + randomPostfix
	terratest_aws.CreateS3Bucket(s.T(), utils.DefaultTestRegion, s.name)
	vpcID, _, _, _, _, err := utils.CreateVPC(s.T(), s.name)
	s.Require().NoError(err, "Failed to create VPC for LB SG test")
	s.vpcID = vpcID
	s.eksSGID, err = utils.CreateSecurityGroup(s.T(), s.name+"-eks", s.vpcID)
	s.traefikSGID, err = utils.CreateSecurityGroup(s.T(), s.name+"-traefik", s.vpcID)
	s.traefik2SGID, err = utils.CreateSecurityGroup(s.T(), s.name+"-traefik2", s.vpcID)
	s.argocdSGID, err = utils.CreateSecurityGroup(s.T(), s.name+"-argocd", s.vpcID)
	s.Require().NoError(err, "Failed to create test security group")

}

func (s *LBSGTestSuite) TearDownTest() {
	utils.DeleteSecurityGroup(s.T(), s.eksSGID)
	utils.DeleteSecurityGroup(s.T(), s.traefikSGID)
	utils.DeleteSecurityGroup(s.T(), s.traefik2SGID)
	utils.DeleteSecurityGroup(s.T(), s.argocdSGID)
	utils.DeleteVPC(s.T(), s.name)
	terratest_aws.EmptyS3Bucket(s.T(), utils.DefaultTestRegion, s.name)
	terratest_aws.DeleteS3Bucket(s.T(), utils.DefaultTestRegion, s.name)
}

func (s *LBSGTestSuite) TestApplyingModule() {
	variables := LBSGVariables{
		Region:       utils.DefaultTestRegion,
		CustomerTag:  utils.DefaultTestCustomerTag,
		ClusterName:  s.name,
		EKSNodeSGID:  s.eksSGID,
		TraefikSGID:  s.traefikSGID,
		Traefik2SGID: s.traefik2SGID,
		ArgoCDSGID:   s.argocdSGID,
	}
	jsonData, err := json.Marshal(variables)
	if err != nil {
		s.T().Fatalf("Failed to marshal variables: %v", err)
	}
	tempFile, err := os.CreateTemp("", "variables-*.tfvar.json")
	if err != nil {
		s.T().Fatalf("Failed to create temporary file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	if _, err := tempFile.Write(jsonData); err != nil {
		s.T().Fatalf("Failed to write to temporary file: %v", err)
	}

	terraformOptions := terraform.WithDefaultRetryableErrors(s.T(), &terraform.Options{
		TerraformDir: ".",
		VarFiles:     []string{tempFile.Name()},
		BackendConfig: map[string]interface{}{
			"region": utils.DefaultTestRegion,
			"bucket": s.name,
			"key":    "lbsg.tfstate",
		},
		Reconfigure: true,
		Upgrade:     true,
	})
	defer terraform.Destroy(s.T(), terraformOptions)
	terraform.InitAndApply(s.T(), terraformOptions)

	fmt.Printf("SGs created")
	// time.Sleep(time.Minute * 5)

	ec2Client, err := terratest_aws.NewEc2ClientE(s.T(), utils.DefaultTestRegion)
	s.Require().NoError(err, "Failed to create EC2 client")
	// Check if traefik rule is valid
	traefikRule, err := ec2Client.DescribeSecurityGroupRules(s.T().Context(), &ec2.DescribeSecurityGroupRulesInput{
		Filters: []types.Filter{
			{
				Name:   aws_sdk.String("group-id"),
				Values: []string{s.eksSGID},
			},
			{
				Name:   aws_sdk.String("referenced-security-group-id"),
				Values: []string{s.traefikSGID},
			},
		},
	})
	s.Require().NoError(err, "Failed to describe security group rules")
	s.Assert().Len(traefikRule.SecurityGroupRules, 1, "Expected 1 rule for Traefik access to EKS Nodes")
	s.Assert().Equal(int32(8443), traefikRule.SecurityGroupRules[0].ToPort, "Expected Traefik rule to allow traffic to port 8843")
	s.Assert().Equal(int32(8443), traefikRule.SecurityGroupRules[0].FromPort, "Expected Traefik rule to allow traffic from port 8843")
	s.Assert().Equal("tcp", traefikRule.SecurityGroupRules[0].IpProtocol, "Expected Traefik rule to use TCP protocol")
	// Check if traefik2 rule is valid
	traefik2Rule, err := ec2Client.DescribeSecurityGroupRules(s.T().Context(), &ec2.DescribeSecurityGroupRulesInput{
		Filters: []types.Filter{
			{
				Name:   aws_sdk.String("group-id"),
				Values: []string{s.eksSGID},
			},
			{
				Name:   aws_sdk.String("referenced-security-group-id"),
				Values: []string{s.traefik2SGID},
			},
		},
	})
	s.Require().NoError(err, "Failed to describe security group rules")
	s.Assert().Len(traefik2Rule.SecurityGroupRules, 1, "Expected 1 rule for Traefik2 access to EKS Nodes")
	s.Assert().Equal(int32(8843), traefik2Rule.SecurityGroupRules[0].ToPort, "Expected Traefik2 rule to allow traffic to port 8843")
	s.Assert().Equal(int32(8843), traefik2Rule.SecurityGroupRules[0].FromPort, "Expected Traefik2 rule to allow traffic from port 8843")
	s.Assert().Equal("tcp", traefik2Rule.SecurityGroupRules[0].IpProtocol, "Expected Traefik2 rule to use TCP protocol")
	// Check if ArgoCD rule is valid
	argocdRule, err := ec2Client.DescribeSecurityGroupRules(s.T().Context(), &ec2.DescribeSecurityGroupRulesInput{
		Filters: []types.Filter{
			{
				Name:   aws_sdk.String("group-id"),
				Values: []string{s.eksSGID},
			},
			{
				Name:   aws_sdk.String("referenced-security-group-id"),
				Values: []string{s.argocdSGID},
			},
			{
				Name:   aws_sdk.String("tag:name"),
				Values: []string{"argocd_ingress_rule"},
			},
		},
	})
	s.Require().NoError(err, "Failed to describe security group rules")
	s.Assert().Len(argocdRule.SecurityGroupRules, 1, "Expected 1 rule for ArgoCD access to EKS Nodes")
	s.Assert().Equal(int32(8080), argocdRule.SecurityGroupRules[0].ToPort, "Expected ArgoCD rule to allow traffic to port 8080")
	s.Assert().Equal(int32(8080), argocdRule.SecurityGroupRules[0].FromPort, "Expected ArgoCD rule to allow traffic from port 8080")
	s.Assert().Equal("tcp", argocdRule.SecurityGroupRules[0].IpProtocol, "Expected ArgoCD rule to use TCP protocol")
	// Check if Gitea rule is valid
	giteaRule, err := ec2Client.DescribeSecurityGroupRules(s.T().Context(), &ec2.DescribeSecurityGroupRulesInput{
		Filters: []types.Filter{
			{
				Name:   aws_sdk.String("group-id"),
				Values: []string{s.eksSGID},
			},
			{
				Name:   aws_sdk.String("referenced-security-group-id"),
				Values: []string{s.argocdSGID},
			},
			{
				Name:   aws_sdk.String("tag:name"),
				Values: []string{"gitea_ingress_rule"},
			},
		},
	})
	s.Require().NoError(err, "Failed to describe security group rules")
	s.Assert().Len(giteaRule.SecurityGroupRules, 1, "Expected 1 rule for Gitea access to EKS Nodes")
	s.Assert().Equal(int32(3000), giteaRule.SecurityGroupRules[0].ToPort, "Expected Gitea rule to allow traffic to port 3000")
	s.Assert().Equal(int32(3000), giteaRule.SecurityGroupRules[0].FromPort, "Expected Gitea rule to allow traffic from port 3000")
	s.Assert().Equal("tcp", giteaRule.SecurityGroupRules[0].IpProtocol, "Expected Gitea rule to use TCP protocol")

}
