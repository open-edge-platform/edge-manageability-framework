// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package steps_aws

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
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	"github.com/praserx/ipconv"
	"golang.org/x/crypto/ssh"
)

const (
	VPCModulePath       = "new-installer/targets/aws/iac/vpc"
	SSKKeySize          = 4096
	DefaultNetworkCIDR  = "10.250.0.0/16"
	VPCBackendBucketKey = "vpc.tfstate"
)

type AWSVPCVariables struct {
	Region                 string                  `json:"region" yaml:"region"`
	Name                   string                  `json:"name" yaml:"name"`
	CidrBlock              string                  `json:"cidr_block" yaml:"cidr_block"`
	EnableDnsHostnames     bool                    `json:"enable_dns_hostnames" yaml:"enable_dns_hostnames"`
	EnableDnsSupport       bool                    `json:"enable_dns_support" yaml:"enable_dns_support"`
	PrivateSubnets         map[string]AWSVPCSubnet `json:"private_subnets" yaml:"private_subnets"`
	PublicSubnets          map[string]AWSVPCSubnet `json:"public_subnets" yaml:"public_subnets"`
	EndpointSGName         string                  `json:"endpoint_sg_name" yaml:"endpoint_sg_name"`
	JumphostIPAllowList    []string                `json:"jumphost_ip_allow_list" yaml:"jumphost_ip_allow_list"`
	JumphostInstanceSshKey string                  `json:"jumphost_instance_ssh_key_pub" yaml:"jumphost_instance_ssh_key_pub"`
	JumphostSubnet         string                  `json:"jumphost_subnet" yaml:"jumphost_subnet"`
	Production             bool                    `json:"production" yaml:"production"`
	CustomerTag            string                  `json:"customer_tag" yaml:"customer_tag"`
}

// NewDefaultAWSVPCVariables creates a new AWSVPCVariables with default values
// based on variable.tf default definitions.
func NewDefaultAWSVPCVariables() AWSVPCVariables {
	return AWSVPCVariables{
		Region:                 "",
		Name:                   "",
		CidrBlock:              "",
		EnableDnsHostnames:     true,
		EnableDnsSupport:       true,
		JumphostIPAllowList:    []string{},
		JumphostInstanceSshKey: "",
		Production:             true,
		CustomerTag:            "",

		// Initialize maps
		PrivateSubnets: make(map[string]AWSVPCSubnet),
		PublicSubnets:  make(map[string]AWSVPCSubnet),
	}
}

type AWSVPCSubnet struct {
	Az        string `json:"az" yaml:"az"`
	CidrBlock string `json:"cidr_block" yaml:"cidr_block"`
}

type AWSVPCStep struct {
	variables          AWSVPCVariables
	backendConfig      TerraformAWSBucketBackendConfig
	RootPath           string
	KeepGeneratedFiles bool
	TerraformExecPath  string
}

func (s *AWSVPCStep) Name() string {
	return "AWSVPCStep"
}

func (s *AWSVPCStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	s.variables = NewDefaultAWSVPCVariables()
	s.variables.Region = config.AWS.Region
	s.variables.Name = config.Global.OrchName
	s.variables.CidrBlock = DefaultNetworkCIDR
	s.variables.EndpointSGName = config.Global.OrchName + "-vpc-ep"

	//Based on the region, we need to get the availability zones.

	// Extract availability zones
	availabilityZones, err := GetAvailableZones(config.AWS.Region)
	if err != nil {
		return config.Generated, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to get availability zones: %v", err),
		}
	}

	// Based on the VPC CIDR block, we need to calculate the private and public subnets
	// and the availability zones.
	vpcCIDR, vpcNet, err := net.ParseCIDR(s.variables.CidrBlock)
	if err != nil {
		return config.Generated, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  fmt.Sprintf("failed to parse VPC CIDR block: %v", err),
		}
	}
	vpcMaskSize, _ := vpcNet.Mask.Size()
	// This logic is correct, since the number of IPs is 2^(32-maskSize).
	if vpcMaskSize > MinimumVPCCIDRMaskSize {
		return config.Generated, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  fmt.Sprintf("VPC CIDR block is too small: %s, minimum is %d", s.variables.CidrBlock, MinimumVPCCIDRMaskSize),
		}
	}
	netAddr := vpcCIDR
	netAddrInt, err := ipconv.IPv4ToInt(netAddr)
	if err != nil {
		return config.Generated, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
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

	s.variables.JumphostSubnet = fmt.Sprintf("subnet-%s-pub", availabilityZones[0])
	s.variables.JumphostIPAllowList = config.AWS.JumpHostWhitelist

	// Generate SSH key pair for the jumphost
	if config.Generated.JumpHostSSHKeyPrivateKey == "" || config.Generated.JumpHostSSHKeyPublicKey == "" {
		privateKey, publicKey, err := GenerateSSHKeyPair()
		if err != nil {
			return config.Generated, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorMsg:  fmt.Sprintf("failed to generate SSH key pair: %v", err),
			}
		}
		s.variables.JumphostInstanceSshKey = publicKey
		config.Generated.JumpHostSSHKeyPrivateKey = privateKey
		config.Generated.JumpHostSSHKeyPublicKey = publicKey
	} else {
		s.variables.JumphostInstanceSshKey = config.Generated.JumpHostSSHKeyPublicKey
	}

	s.variables.CustomerTag = config.AWS.CustomerTag
	s.backendConfig = TerraformAWSBucketBackendConfig{
		Region: config.AWS.Region,
		Bucket: config.Global.OrchName + "-" + config.Generated.DeploymentID,
		Key:    VPCBackendBucketKey,
	}
	return config.Generated, nil
}

func (s *AWSVPCStep) PreStep(ctx context.Context, config config.OrchInstallerConfig) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return config.Generated, nil
}

func (s *AWSVPCStep) RunStep(ctx context.Context, config config.OrchInstallerConfig) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	terraformStepInput := steps.TerraformUtilityInput{
		Action:             config.Generated.Action,
		ExecPath:           s.TerraformExecPath,
		ModulePath:         filepath.Join(s.RootPath, VPCModulePath),
		Variables:          s.variables,
		BackendConfig:      s.backendConfig,
		LogFile:            filepath.Join(config.Generated.LogDir, "aws_vpc.log"),
		KeepGeneratedFiles: s.KeepGeneratedFiles,
	}
	terraformStepOutput, err := steps.RunTerraformModule(ctx, terraformStepInput)
	if err != nil {
		return config.Generated, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeTerraform,
			ErrorMsg:  fmt.Sprintf("failed to run terraform: %v", err),
		}
	}
	if config.Generated.Action == "uninstall" {
		return config.Generated, nil
	}
	if terraformStepOutput.Output != nil {
		if vpcIDMeta, ok := terraformStepOutput.Output["vpc_id"]; !ok {
			return config.Generated, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  "vpc_id does not exist in terraform output",
			}
		} else {
			config.Generated.VPCID = strings.Trim(string(vpcIDMeta.Value), "\"")
		}
		// TODO: Reuse same code for public and private subnets
		if publicSubnets, ok := terraformStepOutput.Output["public_subnets"]; !ok {
			return config.Generated, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  "public_subnets does not exist in terraform output",
			}
		} else {
			jsonBytes, marshalErr := publicSubnets.Value.MarshalJSON()
			if marshalErr != nil {
				return config.Generated, &internal.OrchInstallerError{
					ErrorCode: internal.OrchInstallerErrorCodeTerraform,
					ErrorMsg:  fmt.Sprintf("not able to marshal value of public subnets: %v", marshalErr),
				}
			}

			k := koanf.New(".")
			unmarshalErr := k.Load(rawbytes.Provider(jsonBytes), json.Parser())
			if unmarshalErr != nil {
				return config.Generated, &internal.OrchInstallerError{
					ErrorCode: internal.OrchInstallerErrorCodeTerraform,
					ErrorMsg:  fmt.Sprintf("not able to unmarshal public subnets output: %v", unmarshalErr),
				}
			}
			config.Generated.PublicSubnetIDs = nil
			for subnetName := range s.variables.PublicSubnets {
				subnetId := k.Get(fmt.Sprintf("%s.id", subnetName))
				if subnetId == nil {
					return config.Generated, &internal.OrchInstallerError{
						ErrorCode: internal.OrchInstallerErrorCodeTerraform,
						ErrorMsg:  fmt.Sprintf("subnet id for %s does not exist in terraform output", subnetName),
					}
				}
				config.Generated.PublicSubnetIDs = append(config.Generated.PublicSubnetIDs, subnetId.(string))
			}
		}
		if privateSubnets, ok := terraformStepOutput.Output["private_subnets"]; !ok {
			return config.Generated, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  "private_subnets does not exist in terraform output",
			}
		} else {
			jsonBytes, marshalErr := privateSubnets.Value.MarshalJSON()
			if marshalErr != nil {
				return config.Generated, &internal.OrchInstallerError{
					ErrorCode: internal.OrchInstallerErrorCodeTerraform,
					ErrorMsg:  fmt.Sprintf("not able to marshal value of private subnets: %v", marshalErr),
				}
			}

			k := koanf.New(".")
			unmarshalErr := k.Load(rawbytes.Provider(jsonBytes), json.Parser())
			if unmarshalErr != nil {
				return config.Generated, &internal.OrchInstallerError{
					ErrorCode: internal.OrchInstallerErrorCodeTerraform,
					ErrorMsg:  fmt.Sprintf("not able to unmarshal private subnets output: %v", unmarshalErr),
				}
			}
			config.Generated.PrivateSubnetIDs = nil
			for subnetName := range s.variables.PrivateSubnets {
				subnetId := k.Get(fmt.Sprintf("%s.id", subnetName))
				if subnetId == nil {
					return config.Generated, &internal.OrchInstallerError{
						ErrorCode: internal.OrchInstallerErrorCodeTerraform,
						ErrorMsg:  fmt.Sprintf("subnet id for %s does not exist in terraform output", subnetName),
					}
				}
				config.Generated.PrivateSubnetIDs = append(config.Generated.PrivateSubnetIDs, subnetId.(string))
			}
		}
	} else {
		return config.Generated, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeTerraform,
			ErrorMsg:  "cannot find any output from VPC module",
		}
	}
	return config.Generated, nil
}

func (s *AWSVPCStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return config.Generated, prevStepError
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
