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
	vpcID, _, _, _, _, err := utils.CreateVPC(s.T(), s.name) //nolint:dogsled
	s.Require().NoError(err, "Failed to create VPC for LB SG test")
	s.vpcID = vpcID
	// Create parent security group for EKS
	s.eksSGID, err = utils.CreateSecurityGroup(s.T(), s.name+"-eks", s.vpcID)
	if err != nil {
		s.T().Fatalf("Failed to create EKS Node security group: %v", err)
	}
	// Create security groups for Traefik, Traefik2, and ArgoCD
	s.traefikSGID, err = utils.CreateSecurityGroup(s.T(), s.name+"-traefik", s.vpcID)
	if err != nil {
		s.T().Fatalf("Failed to create Traefik security group: %v", err)
	}
	s.traefik2SGID, err = utils.CreateSecurityGroup(s.T(), s.name+"-traefik2", s.vpcID)
	if err != nil {
		s.T().Fatalf("Failed to create Traefik2 security group: %v", err)
	}
	s.argocdSGID, err = utils.CreateSecurityGroup(s.T(), s.name+"-argocd", s.vpcID)
	if err != nil {
		s.T().Fatalf("Failed to create ArgoCD security group: %v", err)
	}
}

func (s *LBSGTestSuite) TearDownTest() {
	// Delete the EKS Node security group
	err := utils.DeleteSecurityGroup(s.T(), s.eksSGID)
	if err != nil {
		s.T().Fatalf("Failed to delete EKS Node security group: %v", err)
	}
	// Delete the Traefik security group
	err = utils.DeleteSecurityGroup(s.T(), s.traefikSGID)
	if err != nil {
		s.T().Fatalf("Failed to delete Traefik security group: %v", err)
	}
	// Delete the security groups created for the test
	err = utils.DeleteSecurityGroup(s.T(), s.traefik2SGID)
	if err != nil {
		s.T().Fatalf("Failed to delete Traefik2 security group: %v", err)
	}
	// Delete the ArgoCD security group
	err = utils.DeleteSecurityGroup(s.T(), s.argocdSGID)
	if err != nil {
		s.T().Fatalf("Failed to delete ArgoCD security group: %v", err)
	}
	// Delete the VPC created for the test
	err = utils.DeleteVPC(s.T(), s.name)
	if err != nil {
		s.T().Fatalf("Failed to delete VPC: %v", err)
	}
	// Empty and delete the S3 bucket created for the test
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
				Name:   aws_sdk.String("tag:name"),
				Values: []string{"traefik_ingress_rule"},
			},
		},
	})
	s.Require().NoError(err, "Failed to describe security group rules")
	s.Len(traefikRule.SecurityGroupRules, 1, "Expected 1 rule for Traefik access to EKS Nodes")
	s.Equal(int32(8443), *traefikRule.SecurityGroupRules[0].ToPort, "Expected Traefik rule to allow traffic to port 8443")
	s.Equal(int32(8443), *traefikRule.SecurityGroupRules[0].FromPort, "Expected Traefik rule to allow traffic from port 8443")
	s.Equal("tcp", *traefikRule.SecurityGroupRules[0].IpProtocol, "Expected Traefik rule to use TCP protocol")
	s.Equal(s.traefikSGID, *traefikRule.SecurityGroupRules[0].ReferencedGroupInfo.GroupId, "Traefik rule should reference the Traefik security group")
	// Check if traefik2 rule is valid
	traefik2Rule, err := ec2Client.DescribeSecurityGroupRules(s.T().Context(), &ec2.DescribeSecurityGroupRulesInput{
		Filters: []types.Filter{
			{
				Name:   aws_sdk.String("group-id"),
				Values: []string{s.eksSGID},
			},
			{
				Name:   aws_sdk.String("tag:name"),
				Values: []string{"traefik2_ingress_rule"},
			},
		},
	})
	s.Require().NoError(err, "Failed to describe security group rules")
	s.Len(traefik2Rule.SecurityGroupRules, 1, "Expected 1 rule for Traefik2 access to EKS Nodes")
	s.Equal(int32(443), *traefik2Rule.SecurityGroupRules[0].ToPort, "Expected Traefik2 rule to allow traffic to port 443")
	s.Equal(int32(443), *traefik2Rule.SecurityGroupRules[0].FromPort, "Expected Traefik2 rule to allow traffic from port 443")
	s.Equal("tcp", *traefik2Rule.SecurityGroupRules[0].IpProtocol, "Expected Traefik2 rule to use TCP protocol")
	s.Equal(s.traefik2SGID, *traefik2Rule.SecurityGroupRules[0].ReferencedGroupInfo.GroupId, "Traefik2 rule should reference the Traefik2 security group")
	// Check if ArgoCD rule is valid
	argocdRule, err := ec2Client.DescribeSecurityGroupRules(s.T().Context(), &ec2.DescribeSecurityGroupRulesInput{
		Filters: []types.Filter{
			{
				Name:   aws_sdk.String("group-id"),
				Values: []string{s.eksSGID},
			},
			{
				Name:   aws_sdk.String("tag:name"),
				Values: []string{"argocd_ingress_rule"},
			},
		},
	})
	s.Require().NoError(err, "Failed to describe security group rules")
	s.Len(argocdRule.SecurityGroupRules, 1, "Expected 1 rule for ArgoCD access to EKS Nodes")
	s.Equal(int32(8080), *argocdRule.SecurityGroupRules[0].ToPort, "Expected ArgoCD rule to allow traffic to port 8080")
	s.Equal(int32(8080), *argocdRule.SecurityGroupRules[0].FromPort, "Expected ArgoCD rule to allow traffic from port 8080")
	s.Equal("tcp", *argocdRule.SecurityGroupRules[0].IpProtocol, "Expected ArgoCD rule to use TCP protocol")
	s.Equal(s.argocdSGID, *argocdRule.SecurityGroupRules[0].ReferencedGroupInfo.GroupId, "Argocd rule should reference the ArgoCD security group")
	// Check if Gitea rule is valid
	giteaRule, err := ec2Client.DescribeSecurityGroupRules(s.T().Context(), &ec2.DescribeSecurityGroupRulesInput{
		Filters: []types.Filter{
			{
				Name:   aws_sdk.String("group-id"),
				Values: []string{s.eksSGID},
			},
			{
				Name:   aws_sdk.String("tag:name"),
				Values: []string{"gitea_ingress_rule"},
			},
		},
	})
	s.Require().NoError(err, "Failed to describe security group rules")
	s.Len(giteaRule.SecurityGroupRules, 1, "Expected 1 rule for Gitea access to EKS Nodes")
	s.Equal(int32(3000), *giteaRule.SecurityGroupRules[0].ToPort, "Expected Gitea rule to allow traffic to port 3000")
	s.Equal(int32(3000), *giteaRule.SecurityGroupRules[0].FromPort, "Expected Gitea rule to allow traffic from port 3000")
	s.Equal("tcp", *giteaRule.SecurityGroupRules[0].IpProtocol, "Expected Gitea rule to use TCP protocol")
	s.Equal(s.argocdSGID, *giteaRule.SecurityGroupRules[0].ReferencedGroupInfo.GroupId, "Gitea rule should reference the ArgoCD security group")
}
