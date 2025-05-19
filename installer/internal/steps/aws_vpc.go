// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package steps

import (
	"context"
	nativeJson "encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/praserx/ipconv"
)

const (
	ModulePath      = "pod-configs/orchestrator/vpc"
	JumpHostAMIName = "ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-amd64-server-*"
)

type AWSVPCVariables struct {
	Region                  string                   `json:"region" yaml:"region"`
	VPCName                 string                   `json:"vpc_name" yaml:"vpc_name"`
	VPCCidrBlock            string                   `json:"vpc_cidr_block" yaml:"vpc_cidr_block"`
	VPCAdditionalCidrBlocks []string                 `json:"vpc_additional_cidr_blocks" yaml:"vpc_additional_cidr_blocks"`
	VPCEnableDnsHostnames   bool                     `json:"vpc_enable_dns_hostnames" yaml:"vpc_enable_dns_hostnames"`
	VPCEnableDnsSupport     bool                     `json:"vpc_enable_dns_support" yaml:"vpc_enable_dns_support"`
	PrivateSubnets          map[string]AWSVPCSubnet  `json:"private_subnets" yaml:"private_subnets"`
	PublicSubnets           map[string]AWSVPCSubnet  `json:"public_subnets" yaml:"public_subnets"`
	EndpointSGName          string                   `json:"endpoint_sg_name" yaml:"endpoint_sg_name"`
	JumphostIPAllowList     []string                 `json:"jumphost_ip_allow_list" yaml:"jumphost_ip_allow_list"`
	JumphostAmiId           string                   `json:"jumphost_ami_id" yaml:"jumphost_ami_id"`
	JumphostInstanceType    string                   `json:"jumphost_instance_type" yaml:"jumphost_instance_type"`
	JumphostInstanceSshKey  string                   `json:"jumphost_instance_ssh_key_pub" yaml:"jumphost_instance_ssh_key_pub"`
	JumphostSubnet          AWSVPCJumphostSubnetType `json:"jumphost_subnet" yaml:"jumphost_subnet"`
	Production              bool                     `json:"production" yaml:"production"`
	CustomerTag             string                   `json:"customer_tag" yaml:"customer_tag"`
}

// NewDefaultAWSVPCVariables creates a new AWSVPCVariables with default values
// based on variable.tf default definitions.
func NewDefaultAWSVPCVariables() AWSVPCVariables {
	return AWSVPCVariables{
		Region:                  "",
		VPCName:                 "",
		VPCCidrBlock:            "",
		VPCAdditionalCidrBlocks: []string{},
		VPCEnableDnsHostnames:   true,
		VPCEnableDnsSupport:     true,
		JumphostIPAllowList:     []string{},
		JumphostInstanceType:    "t3.medium",
		JumphostInstanceSshKey:  "",
		Production:              true,
		CustomerTag:             "",

		// Initialize maps
		PrivateSubnets: make(map[string]AWSVPCSubnet),
		PublicSubnets:  make(map[string]AWSVPCSubnet),
	}
}

type AWSVPCSubnet struct {
	Az        string `json:"az" yaml:"az"`
	CidrBlock string `json:"cidr_block" yaml:"cidr_block"`
}

type AWSVPCJumphostSubnetType struct {
	Name      string `json:"name" yaml:"name"`
	Az        string `json:"az" yaml:"az"`
	CidrBlock string `json:"cidr_block" yaml:"cidr_block"`
}

type AWSVPCStep struct {
	variables          AWSVPCVariables
	backendConfig      TerraformAWSBucketBackendConfig
	RootPath           string
	KeepGeneratedFiles bool
}

func (s *AWSVPCStep) Name() string {
	return "AWSVPCStep"
}

func (s *AWSVPCStep) ConfigStep(ctx context.Context, config internal.OrchInstallerConfig, runtimeState internal.OrchInstallerRuntimeState) (internal.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	s.variables = NewDefaultAWSVPCVariables()
	s.variables.Region = config.Region
	s.variables.VPCName = config.DeploymentName
	s.variables.VPCCidrBlock = config.NetworkCIDR

	//Based on the region, we need to get the availability zones.

	// Extract availability zones
	availabilityZones, err := GetAvailableZones(config.Region)
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorStep: s.Name(),
			ErrorMsg:  fmt.Sprintf("failed to get availability zones: %v", err),
		}
	}

	// Based on the VPC CIDR block, we need to calculate the private and public subnets
	// and the availability zones.
	vpcCIDR, vpcNet, err := net.ParseCIDR(s.variables.VPCCidrBlock)
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorStep: s.Name(),
			ErrorMsg:  fmt.Sprintf("failed to parse VPC CIDR block: %v", err),
		}
	}
	vpcMaskSize, _ := vpcNet.Mask.Size()
	// This logic is correct, since the number of IPs is 2^(32-maskSize).
	if vpcMaskSize > MinimumVPCCIDRMaskSize {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorStep: s.Name(),
			ErrorMsg:  fmt.Sprintf("VPC CIDR block is too small: %s, minimum is %d", s.variables.VPCCidrBlock, MinimumVPCCIDRMaskSize),
		}
	}
	netAddr := vpcCIDR
	netAddrInt, err := ipconv.IPv4ToInt(netAddr)
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorStep: s.Name(),
			ErrorMsg:  fmt.Sprintf("failed to convert IP to int: %v", err),
		}
	}
	for i := range RequiredAvailabilityZones {
		name := fmt.Sprintf("subnet-%s-pub", availabilityZones[i])
		ipInt := netAddrInt + (uint32)(i*(1<<uint(32-PublicSubnetMaskSize)))
		ip := ipconv.IntToIPv4(ipInt)
		s.variables.PublicSubnets[name] = AWSVPCSubnet{
			Az:        availabilityZones[i],
			CidrBlock: fmt.Sprintf("%s/%d", ip.String(), PublicSubnetMaskSize),
		}
	}
	netAddrInt += RequiredAvailabilityZones * (1 << uint(32-PublicSubnetMaskSize))
	for i := range RequiredAvailabilityZones {
		name := fmt.Sprintf("subnet-%s", availabilityZones[i])
		ipInt := netAddrInt + (uint32)(i*(1<<uint(32-PrivateSubnetMaskSize)))
		ip := ipconv.IntToIPv4(ipInt)
		s.variables.PrivateSubnets[name] = AWSVPCSubnet{
			Az:        availabilityZones[i],
			CidrBlock: fmt.Sprintf("%s/%d", ip.String(), PrivateSubnetMaskSize),
		}
	}

	s.variables.JumphostSubnet = AWSVPCJumphostSubnetType{
		Name:      fmt.Sprintf("subnet-%s-pub", availabilityZones[0]),
		Az:        availabilityZones[0],
		CidrBlock: s.variables.PublicSubnets[fmt.Sprintf("subnet-%s-pub", availabilityZones[0])].CidrBlock,
	}
	s.variables.JumphostAmiId, err = FindAMIIDByName(config.Region, JumpHostAMIName)
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorStep: s.Name(),
			ErrorMsg:  fmt.Sprintf("failed to find AMI ID: %v", err),
		}
	}
	s.variables.CustomerTag = config.CustomerTag
	s.variables.JumphostIPAllowList = config.JumpHostIPAllowList

	s.backendConfig = TerraformAWSBucketBackendConfig{
		Region: config.Region,
		Bucket: config.DeploymentName + "-" + config.StateStoreBucketPostfix,
		Key:    config.Region + "/vpc/" + config.DeploymentName,
	}
	return runtimeState, nil
}

func (s *AWSVPCStep) PreSetp(ctx context.Context, config internal.OrchInstallerConfig, runtimeState internal.OrchInstallerRuntimeState) (internal.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

func (s *AWSVPCStep) RunStep(ctx context.Context, config internal.OrchInstallerConfig, runtimeState internal.OrchInstallerRuntimeState) (internal.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	terraformStep := OrchInstallerTerraformStep{
		Action:             runtimeState.Action,
		ExecPath:           runtimeState.TerraformExecPath,
		ModulePath:         filepath.Join(s.RootPath, ModulePath),
		Variables:          s.variables,
		BackendConfig:      s.backendConfig,
		LogFile:            filepath.Join(runtimeState.LogDir, "aws_vpc.log"),
		KeepGeneratedFiles: s.KeepGeneratedFiles,
	}
	terraformStepOutput, err := terraformStep.Run(ctx)
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeTerraform,
			ErrorStep: s.Name(),
			ErrorMsg:  fmt.Sprintf("failed to run terraform: %v", err),
		}
	}
	if terraformStepOutput != nil && terraformStepOutput.Output != nil {
		if vpcIDMeta, ok := terraformStepOutput.Output["vpc_id"]; !ok {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorStep: s.Name(),
				ErrorMsg:  "vpc_id does not exist in terraform output",
			}
		} else {
			runtimeState.VPCID = strings.Trim(string(vpcIDMeta.Value), "\"")
		}
		// TODO: Reuse same code for public and private subnets
		if publicSubnets, ok := terraformStepOutput.Output["public_subnets"]; !ok {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorStep: s.Name(),
				ErrorMsg:  "public_subnets does not exist in terraform output",
			}
		} else {
			var subnets []struct {
				Id string `json:"id" yaml:"id"`
				// Skip other fields
			}
			unmarshalErr := nativeJson.Unmarshal([]byte(publicSubnets.Value), &subnets)
			if unmarshalErr != nil {
				return runtimeState, &internal.OrchInstallerError{
					ErrorCode: internal.OrchInstallerErrorCodeTerraform,
					ErrorStep: s.Name(),
					ErrorMsg:  "not able to unmarshal public subnets output",
				}
			}
			runtimeState.PublicSubnetIds = nil
			for _, s := range subnets {
				runtimeState.PublicSubnetIds = append(runtimeState.PublicSubnetIds, s.Id)
			}
		}
		if privateSubnets, ok := terraformStepOutput.Output["private_subnets"]; !ok {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorStep: s.Name(),
				ErrorMsg:  "private_subnets does not exist in terraform output",
			}
		} else {
			var subnets []struct {
				Id string `json:"id" yaml:"id"`
				// Skip other fields
			}
			unmarshalErr := nativeJson.Unmarshal([]byte(privateSubnets.Value), &subnets)
			if unmarshalErr != nil {
				return runtimeState, &internal.OrchInstallerError{
					ErrorCode: internal.OrchInstallerErrorCodeTerraform,
					ErrorStep: s.Name(),
					ErrorMsg:  "not able to unmarshal public subnets output",
				}
			}
			runtimeState.PrivateSubnetIds = nil
			for _, s := range subnets {
				runtimeState.PrivateSubnetIds = append(runtimeState.PrivateSubnetIds, s.Id)
			}
		}
	} else {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeTerraform,
			ErrorStep: s.Name(),
			ErrorMsg:  "cannot find any output from VPC module",
		}
	}
	return runtimeState, nil
}

func (s *AWSVPCStep) PostStep(ctx context.Context, config internal.OrchInstallerConfig, runtimeState internal.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (internal.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}
