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

var vpcStepLabels = []string{
	"aws",
	"vpc",
}

var VPCEndpoints = []string{
	"elasticfilesystem",
	"s3",
	"eks",
	"sts",
	"ec2",
	"ec2messages",
	"ecr.dkr",
	"ecr.api",
	"elasticloadbalancing",
}

type VPCVariables struct {
	Region                 string               `json:"region"`
	Name                   string               `json:"name"`
	CidrBlock              string               `json:"cidr_block"`
	EnableDnsHostnames     bool                 `json:"enable_dns_hostnames"`
	EnableDnsSupport       bool                 `json:"enable_dns_support"`
	PrivateSubnets         map[string]VPCSubnet `json:"private_subnets"`
	PublicSubnets          map[string]VPCSubnet `json:"public_subnets"`
	EndpointSGName         string               `json:"endpoint_sg_name"`
	JumphostIPAllowList    []string             `json:"jumphost_ip_allow_list"`
	JumphostInstanceSSHKey string               `json:"jumphost_instance_ssh_key_pub"`
	JumphostSubnet         string               `json:"jumphost_subnet"`
	Production             bool                 `json:"production"`
	CustomerTag            string               `json:"customer_tag,omitempty"`
	Endpoints              []string             `json:"endpoints"`
}

// NewDefaultVPCVariables creates a new VPCVariables with default values
// based on variable.tf default definitions.
func NewDefaultVPCVariables() VPCVariables {
	return VPCVariables{
		Region:                 "",
		Name:                   "",
		CidrBlock:              "",
		EnableDnsHostnames:     true,
		EnableDnsSupport:       true,
		Endpoints:              VPCEndpoints,
		JumphostIPAllowList:    make([]string, 0),
		JumphostInstanceSSHKey: "",
		Production:             true,
		CustomerTag:            "",

		// Initialize maps
		PrivateSubnets: make(map[string]VPCSubnet),
		PublicSubnets:  make(map[string]VPCSubnet),
	}
}

type VPCSubnet struct {
	Az        string `json:"az" yaml:"az"`
	CidrBlock string `json:"cidr_block" yaml:"cidr_block"`
}

type VPCStep struct {
	variables          VPCVariables
	backendConfig      TerraformAWSBucketBackendConfig
	RootPath           string
	KeepGeneratedFiles bool
	StepLabels         []string
	TerraformUtility   steps.TerraformUtility
	AWSUtility         AWSUtility
}

func CreateVPCStep(rootPath string, keepGeneratedFiles bool, terraformUtility steps.TerraformUtility, awsUtility AWSUtility) *VPCStep {
	return &VPCStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: keepGeneratedFiles,
		TerraformUtility:   terraformUtility,
		AWSUtility:         awsUtility,
		StepLabels:         vpcStepLabels,
	}
}

func (s *VPCStep) Name() string {
	return "VPCStep"
}

func (s *VPCStep) Labels() []string {
	return s.StepLabels
}

func (s *VPCStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if s.skipVPCStep(config) {
		// If VPC ID is already set, we skip this step.
		runtimeState.AWS.VPCID = config.AWS.VPCID
		var err error
		runtimeState.AWS.PublicSubnetIDs, runtimeState.AWS.PrivateSubnetIDs, err = s.AWSUtility.GetSubnetIDsFromVPC(config.AWS.Region, runtimeState.AWS.VPCID)
		if err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorMsg:  fmt.Sprintf("failed to get subnet IDs from VPC: %v", err),
			}
		}
		return runtimeState, nil
	}
	s.variables = NewDefaultVPCVariables()
	s.variables.Region = config.AWS.Region
	s.variables.Name = config.Global.OrchName
	s.variables.CidrBlock = DefaultNetworkCIDR
	s.variables.EndpointSGName = config.Global.OrchName + "-vpc-ep"
	s.variables.Endpoints = VPCEndpoints

	// Based on the region, we need to get the availability zones.
	availabilityZones, err := s.AWSUtility.GetAvailableZones(config.AWS.Region)
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to get availability zones: %v", err),
		}
	}

	// Based on the VPC CIDR block, we need to calculate the private and public subnets
	// and the availability zones.
	vpcCIDR, vpcNet, err := net.ParseCIDR(s.variables.CidrBlock)
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  fmt.Sprintf("failed to parse VPC CIDR block: %v", err),
		}
	}
	vpcMaskSize, _ := vpcNet.Mask.Size()
	// This logic is correct, since the number of IPs is 2^(32-maskSize).
	if vpcMaskSize > MinimumVPCCIDRMaskSize {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  fmt.Sprintf("VPC CIDR block is too small: %s, minimum is %d", s.variables.CidrBlock, MinimumVPCCIDRMaskSize),
		}
	}
	netAddr := vpcCIDR
	netAddrInt, err := ipconv.IPv4ToInt(netAddr)
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to convert IP to int: %v", err),
		}
	}
	for i, zone := range availabilityZones {
		name := fmt.Sprintf("subnet-%s", zone)
		ipInt := netAddrInt + (uint32)(i*(1<<uint(32-PrivateSubnetMaskSize)))
		ip := ipconv.IntToIPv4(ipInt)
		s.variables.PrivateSubnets[name] = VPCSubnet{
			Az:        zone,
			CidrBlock: fmt.Sprintf("%s/%d", ip.String(), PrivateSubnetMaskSize),
		}
	}
	netAddrInt += uint32(len(availabilityZones)) * (1 << uint(32-PrivateSubnetMaskSize))

	for i, zone := range availabilityZones {
		name := fmt.Sprintf("subnet-%s-pub", zone)
		ipInt := netAddrInt + (uint32)(i*(1<<uint(32-PublicSubnetMaskSize)))
		ip := ipconv.IntToIPv4(ipInt)
		s.variables.PublicSubnets[name] = VPCSubnet{
			Az:        zone,
			CidrBlock: fmt.Sprintf("%s/%d", ip.String(), PublicSubnetMaskSize),
		}
	}

	s.variables.JumphostSubnet = fmt.Sprintf("subnet-%s-pub", availabilityZones[0])
	s.variables.JumphostIPAllowList = config.AWS.JumpHostWhitelist

	// Generate SSH key pair for the jumphost
	if runtimeState.AWS.JumpHostSSHKeyPrivateKey == "" || runtimeState.AWS.JumpHostSSHKeyPublicKey == "" {
		privateKey, publicKey, err := generateSSHKeyPair()
		if err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorMsg:  fmt.Sprintf("failed to generate SSH key pair: %v", err),
			}
		}
		s.variables.JumphostInstanceSSHKey = publicKey
		runtimeState.AWS.JumpHostSSHKeyPrivateKey = privateKey
		runtimeState.AWS.JumpHostSSHKeyPublicKey = publicKey
	} else {
		s.variables.JumphostInstanceSSHKey = runtimeState.AWS.JumpHostSSHKeyPublicKey
	}

	s.variables.CustomerTag = config.AWS.CustomerTag
	s.backendConfig = TerraformAWSBucketBackendConfig{
		Region: config.AWS.Region,
		Bucket: config.Global.OrchName + "-" + runtimeState.DeploymentID,
		Key:    VPCBackendBucketKey,
	}
	return runtimeState, nil
}

func (s *VPCStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if s.skipVPCStep(config) {
		return runtimeState, nil
	}
	if config.AWS.PreviousS3StateBucket == "" {
		// No need to migrate state, since there is no previous state bucket
		return runtimeState, nil
	}

	// Need to move Terraform state from old bucket to new bucket:
	oldVPCBucketKey := fmt.Sprintf("%s/vpc/%s", config.AWS.Region, config.Global.OrchName)
	err := s.AWSUtility.S3CopyToS3(config.AWS.Region,
		config.AWS.PreviousS3StateBucket,
		oldVPCBucketKey,
		config.AWS.Region,
		s.backendConfig.Bucket,
		s.backendConfig.Key)
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to move Terraform state from old bucket to new bucket: %v", err),
		}
	}

	modulePath := filepath.Join(s.RootPath, VPCModulePath)
	states := map[string]string{
		"module.vpc.main": "aws_vpc.main",
		"module.internet_gateway.aws_internet_gateway.igw":                            "aws_internet_gateway.igw",
		"module.endpoint.aws_security_group.vpc_endpoints":                            "aws_security_group.vpc_endpoints",
		"module.jumphost.aws_key_pair.jumphost_instance_launch_key":                   "aws_key_pair.jumphost_instance_launch_key",
		"module.jumphost.aws_iam_role.ec2":                                            "aws_iam_role.ec2",
		"module.jumphost.aws_iam_policy.eks_cluster_access_policy":                    "aws_iam_policy.eks_cluster_access_policy",
		"module.jumphost.aws_iam_role_policy_attachment.eks_cluster_access":           "aws_iam_role_policy_attachment.eks_cluster_access",
		"module.jumphost.aws_iam_role_policy_attachment.AmazonSSMManagedInstanceCore": "aws_iam_role_policy_attachment.AmazonSSMManagedInstanceCore",
		"module.jumphost.aws_iam_instance_profile.ec2":                                "aws_iam_instance_profile.ec2",
		"module.jumphost.aws_instance.jumphost":                                       "aws_instance.jumphost",
		"module.jumphost.aws_security_group.jumphost":                                 "aws_security_group.jumphost",
		"module.jumphost.aws_security_group_rule.jumphost_egress_https":               "aws_security_group_rule.jumphost_egress_https",
		"module.jumphost.aws_eip.jumphost":                                            "aws_eip.jumphost",
		"module.jumphost.aws_eip_association.jumphost":                                "aws_eip_association.jumphost",
	}
	for name := range s.variables.PublicSubnets {
		states[fmt.Sprintf("module.vpc.aws_subnet.public_subnet[\"%s\"]", name)] = fmt.Sprintf("aws_subnet.public_subnet[\"%s\"]", name)
		states[fmt.Sprintf("module.nat_gateway.aws_eip.ngw[\"%s\"]", name)] = fmt.Sprintf("aws_eip.ngw[\"%s\"]", name)
		states[fmt.Sprintf("module.nat_gateway.aws_nat_gateway.ngw_with_eip[\"%s\"]", name)] = fmt.Sprintf("aws_nat_gateway.main[\"%s\"]", name)
		states[fmt.Sprintf("module.route_table.aws_route_table.public_subnet[\"%s\"]", name)] = fmt.Sprintf("aws_route_table.public_subnet[\"%s\"]", name)
		states[fmt.Sprintf("module.route_table.aws_route_table_association.public_subnet[\"%s\"]", name)] = fmt.Sprintf("aws_route_table_association.public_subnet[\"%s\"]", name)
	}
	for name := range s.variables.PrivateSubnets {
		states[fmt.Sprintf("module.vpc.aws_subnet.private_subnet[\"%s\"]", name)] = fmt.Sprintf("aws_subnet.private_subnet[\"%s\"]", name)
		states[fmt.Sprintf("module.route_table.aws_route_table.private_subnet[\"%s\"]", name)] = fmt.Sprintf("aws_route_table.private_subnet[\"%s\"]", name)
		states[fmt.Sprintf("module.route_table.aws_route_table_association.private_subnet[\"%s\"]", name)] = fmt.Sprintf("aws_route_table_association.private_subnet[\"%s\"]", name)
	}
	for _, ep := range VPCEndpoints {
		states[fmt.Sprintf("module.endpoint.aws_vpc_endpoint.endpoint[\"%s\"]", ep)] = fmt.Sprintf("aws_vpc_endpoint.endpoint[\"%s\"]", ep)
	}
	mvErr := s.TerraformUtility.MoveStates(ctx, steps.TerraformUtilityMoveStatesInput{
		ModulePath: modulePath,
		States:     states,
	})
	if mvErr != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to move Terraform state: %v", mvErr),
		}
	}

	return runtimeState, nil
}

func (s *VPCStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if s.skipVPCStep(config) {
		return runtimeState, nil
	}
	terraformStepInput := steps.TerraformUtilityInput{
		Action:             runtimeState.Action,
		ModulePath:         filepath.Join(s.RootPath, VPCModulePath),
		Variables:          s.variables,
		BackendConfig:      s.backendConfig,
		LogFile:            filepath.Join(runtimeState.LogDir, "aws_vpc.log"),
		KeepGeneratedFiles: s.KeepGeneratedFiles,
	}
	terraformStepOutput, err := s.TerraformUtility.Run(ctx, terraformStepInput)
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeTerraform,
			ErrorMsg:  fmt.Sprintf("failed to run terraform: %v", err),
		}
	}
	if runtimeState.Action == "uninstall" {
		return runtimeState, nil
	}
	if terraformStepOutput.Output != nil {
		if vpcIDMeta, ok := terraformStepOutput.Output["vpc_id"]; !ok {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  "vpc_id does not exist in terraform output",
			}
		} else {
			runtimeState.AWS.VPCID = strings.Trim(string(vpcIDMeta.Value), "\"")
		}
		// TODO: Reuse same code for public and private subnets
		if publicSubnets, ok := terraformStepOutput.Output["public_subnets"]; !ok {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  "public_subnets does not exist in terraform output",
			}
		} else {
			jsonBytes, marshalErr := publicSubnets.Value.MarshalJSON()
			if marshalErr != nil {
				return runtimeState, &internal.OrchInstallerError{
					ErrorCode: internal.OrchInstallerErrorCodeTerraform,
					ErrorMsg:  fmt.Sprintf("not able to marshal value of public subnets: %v", marshalErr),
				}
			}

			k := koanf.New(".")
			unmarshalErr := k.Load(rawbytes.Provider(jsonBytes), json.Parser())
			if unmarshalErr != nil {
				return runtimeState, &internal.OrchInstallerError{
					ErrorCode: internal.OrchInstallerErrorCodeTerraform,
					ErrorMsg:  fmt.Sprintf("not able to unmarshal public subnets output: %v", unmarshalErr),
				}
			}
			runtimeState.AWS.PublicSubnetIDs = nil
			for subnetName := range s.variables.PublicSubnets {
				subnetId := k.Get(fmt.Sprintf("%s.id", subnetName))
				if subnetId == nil {
					return runtimeState, &internal.OrchInstallerError{
						ErrorCode: internal.OrchInstallerErrorCodeTerraform,
						ErrorMsg:  fmt.Sprintf("subnet id for %s does not exist in terraform output", subnetName),
					}
				}
				runtimeState.AWS.PublicSubnetIDs = append(runtimeState.AWS.PublicSubnetIDs, subnetId.(string))
			}
		}
		if privateSubnets, ok := terraformStepOutput.Output["private_subnets"]; !ok {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  "private_subnets does not exist in terraform output",
			}
		} else {
			jsonBytes, marshalErr := privateSubnets.Value.MarshalJSON()
			if marshalErr != nil {
				return runtimeState, &internal.OrchInstallerError{
					ErrorCode: internal.OrchInstallerErrorCodeTerraform,
					ErrorMsg:  fmt.Sprintf("not able to marshal value of private subnets: %v", marshalErr),
				}
			}

			k := koanf.New(".")
			unmarshalErr := k.Load(rawbytes.Provider(jsonBytes), json.Parser())
			if unmarshalErr != nil {
				return runtimeState, &internal.OrchInstallerError{
					ErrorCode: internal.OrchInstallerErrorCodeTerraform,
					ErrorMsg:  fmt.Sprintf("not able to unmarshal private subnets output: %v", unmarshalErr),
				}
			}
			runtimeState.AWS.PrivateSubnetIDs = nil
			for subnetName := range s.variables.PrivateSubnets {
				subnetId := k.Get(fmt.Sprintf("%s.id", subnetName))
				if subnetId == nil {
					return runtimeState, &internal.OrchInstallerError{
						ErrorCode: internal.OrchInstallerErrorCodeTerraform,
						ErrorMsg:  fmt.Sprintf("subnet id for %s does not exist in terraform output", subnetName),
					}
				}
				runtimeState.AWS.PrivateSubnetIDs = append(runtimeState.AWS.PrivateSubnetIDs, subnetId.(string))
			}
		}
		if jumphostIP, ok := terraformStepOutput.Output["jumphost_ip"]; !ok {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  "jumphost_ip does not exist in terraform output",
			}
		} else {
			runtimeState.AWS.JumpHostIP = strings.Trim(string(jumphostIP.Value), "\"")
		}
	} else {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeTerraform,
			ErrorMsg:  "cannot find any output from VPC module",
		}
	}
	return runtimeState, nil
}

func (s *VPCStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if s.skipVPCStep(config) {
		return runtimeState, nil
	}
	return runtimeState, prevStepError
}

func generateSSHKeyPair() (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, SSKKeySize)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
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

func (s *VPCStep) skipVPCStep(config config.OrchInstallerConfig) bool {
	return config.AWS.VPCID != ""
}
