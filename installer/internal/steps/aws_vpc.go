// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package steps

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/praserx/ipconv"
	"golang.org/x/crypto/ssh"
)

const (
	ModulePath       = "pod-configs/orchestrator/vpc"
	JumpHostAMIName  = "ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-amd64-server-20250516.1"
	JumpHostAMIOwner = "099720109477"
	JumpHostAMIID    = "ami-0026a04369a3093cc"
	SSKKeySize       = 4096
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
	s.variables.EndpointSGName = config.DeploymentName + "-vpc-ep"

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
		name := fmt.Sprintf("subnet-%s", availabilityZones[i])
		ipInt := netAddrInt + (uint32)(i*(1<<uint(32-PrivateSubnetMaskSize)))
		ip := ipconv.IntToIPv4(ipInt)
		s.variables.PrivateSubnets[name] = AWSVPCSubnet{
			Az:        availabilityZones[i],
			CidrBlock: fmt.Sprintf("%s/%d", ip.String(), PrivateSubnetMaskSize),
		}
	}
	netAddrInt += RequiredAvailabilityZones * (1 << uint(32-PrivateSubnetMaskSize))
	for i := range RequiredAvailabilityZones {
		name := fmt.Sprintf("subnet-%s-pub", availabilityZones[i])
		ipInt := netAddrInt + (uint32)(i*(1<<uint(32-PublicSubnetMaskSize)))
		ip := ipconv.IntToIPv4(ipInt)
		s.variables.PublicSubnets[name] = AWSVPCSubnet{
			Az:        availabilityZones[i],
			CidrBlock: fmt.Sprintf("%s/%d", ip.String(), PublicSubnetMaskSize),
		}
	}

	s.variables.JumphostSubnet = AWSVPCJumphostSubnetType{
		Name:      fmt.Sprintf("%s-subnet-%s-pub", config.DeploymentName, availabilityZones[0]),
		Az:        availabilityZones[0],
		CidrBlock: s.variables.PublicSubnets[fmt.Sprintf("subnet-%s-pub", availabilityZones[0])].CidrBlock,
	}
	s.variables.JumphostAmiId = JumpHostAMIID
	s.variables.JumphostIPAllowList = config.JumpHostIPAllowList

	// Generate SSH key pair for the jumphost
	if runtimeState.JumpHostSSHKeyPrivateKey == "" || runtimeState.JumpHostSSHKeyPublicKey == "" {
		privateKey, publicKey, err := GenerateSSHKeyPair()
		if err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorStep: s.Name(),
				ErrorMsg:  fmt.Sprintf("failed to generate SSH key pair: %v", err),
			}
		}
		s.variables.JumphostInstanceSshKey = publicKey
		runtimeState.JumpHostSSHKeyPrivateKey = privateKey
		runtimeState.JumpHostSSHKeyPublicKey = publicKey
	} else {
		s.variables.JumphostInstanceSshKey = runtimeState.JumpHostSSHKeyPublicKey
	}

	s.variables.CustomerTag = config.CustomerTag

	s.backendConfig = TerraformAWSBucketBackendConfig{
		Region: config.Region,
		Bucket: config.DeploymentName + "-" + config.StateStoreBucketPostfix,
		Key:    config.Region + "/vpc/" + config.DeploymentName,
	}
	return runtimeState, nil
}

func (s *AWSVPCStep) PreSetp(ctx context.Context, config internal.OrchInstallerConfig, runtimeState internal.OrchInstallerRuntimeState) (internal.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	terraformExecPath, err := InstallTerraformAndGetExecPath()
	runtimeState.TerraformExecPath = terraformExecPath
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorStep: s.Name(),
			ErrorMsg:  fmt.Sprintf("failed to get terraform exec path: %v", err),
		}
	}
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
			jsonBytes, marshalErr := publicSubnets.Value.MarshalJSON()
			if marshalErr != nil {
				return runtimeState, &internal.OrchInstallerError{
					ErrorCode: internal.OrchInstallerErrorCodeTerraform,
					ErrorStep: s.Name(),
					ErrorMsg:  fmt.Sprintf("not able to marshal value of public subnets: %v", marshalErr),
				}
			}

			k := koanf.New(".")
			unmarshalErr := k.Load(rawbytes.Provider(jsonBytes), json.Parser())
			if unmarshalErr != nil {
				return runtimeState, &internal.OrchInstallerError{
					ErrorCode: internal.OrchInstallerErrorCodeTerraform,
					ErrorStep: s.Name(),
					ErrorMsg:  fmt.Sprintf("not able to unmarshal public subnets output: %v", unmarshalErr),
				}
			}
			runtimeState.PublicSubnetIds = nil
			for subnetName := range s.variables.PublicSubnets {
				subnetId := k.Get(fmt.Sprintf("%s.id", subnetName))
				if subnetId == nil {
					return runtimeState, &internal.OrchInstallerError{
						ErrorCode: internal.OrchInstallerErrorCodeTerraform,
						ErrorStep: s.Name(),
						ErrorMsg:  fmt.Sprintf("subnet id for %s does not exist in terraform output", subnetName),
					}
				}
				runtimeState.PublicSubnetIds = append(runtimeState.PublicSubnetIds, subnetId.(string))
			}
		}
		if privateSubnets, ok := terraformStepOutput.Output["private_subnets"]; !ok {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorStep: s.Name(),
				ErrorMsg:  "private_subnets does not exist in terraform output",
			}
		} else {
			jsonBytes, marshalErr := privateSubnets.Value.MarshalJSON()
			if marshalErr != nil {
				return runtimeState, &internal.OrchInstallerError{
					ErrorCode: internal.OrchInstallerErrorCodeTerraform,
					ErrorStep: s.Name(),
					ErrorMsg:  fmt.Sprintf("not able to marshal value of private subnets: %v", marshalErr),
				}
			}

			k := koanf.New(".")
			unmarshalErr := k.Load(rawbytes.Provider(jsonBytes), json.Parser())
			if unmarshalErr != nil {
				return runtimeState, &internal.OrchInstallerError{
					ErrorCode: internal.OrchInstallerErrorCodeTerraform,
					ErrorStep: s.Name(),
					ErrorMsg:  fmt.Sprintf("not able to unmarshal private subnets output: %v", unmarshalErr),
				}
			}
			runtimeState.PrivateSubnetIds = nil
			for subnetName := range s.variables.PrivateSubnets {
				subnetId := k.Get(fmt.Sprintf("%s.id", subnetName))
				if subnetId == nil {
					return runtimeState, &internal.OrchInstallerError{
						ErrorCode: internal.OrchInstallerErrorCodeTerraform,
						ErrorStep: s.Name(),
						ErrorMsg:  fmt.Sprintf("subnet id for %s does not exist in terraform output", subnetName),
					}
				}
				runtimeState.PrivateSubnetIds = append(runtimeState.PrivateSubnetIds, subnetId.(string))
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
	return runtimeState, prevStepError
}

func GenerateSSHKeyPair() (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, SSKKeySize)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %v", err)
	}

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}
	privateKeyString := string(pem.EncodeToMemory(privateKeyPEM))
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", err
	}
	publicKeyString := string(ssh.MarshalAuthorizedKey(pub))
	return privateKeyString, publicKeyString, nil
}
