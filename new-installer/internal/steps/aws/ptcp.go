// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package steps_aws

import (
	"context"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
)

const (
	PTCPModulePath          = "new-installer/targets/aws/iac/ptcp"
	PTCPBackendBucketKey    = "ptcp.tfstate"
	DefaultPTCPCPU          = 1024 // 1 vCPU
	DefaultPTCPMemory       = 2048 // 2048 MB
	DefaultPTCPDesiredCount = 1
)

var ptcpStepLabels = []string{
	"aws",
	"ptcp",
	"pull-through-cache-proxy",
}

type PTCPVariables struct {
	ClusterName     string   `json:"cluster_name"`
	Region          string   `json:"region"`
	VPCID           string   `json:"vpc_id"`
	SubnetIDs       []string `json:"subnet_ids"`
	HTTPProxy       string   `json:"http_proxy"`
	HTTPSProxy      string   `json:"https_proxy"`
	NoProxy         string   `json:"no_proxy"`
	CustomerTag     string   `json:"customer_tag"`
	Route53ZoneName string   `json:"route53_zone_name"`
	IPAllowList     []string `json:"ip_allow_list"`
	CPU             int      `json:"cpu"`
	Memory          int      `json:"memory"`
	DesiredCount    int      `json:"desired_count"`
	TLSCertKey      string   `json:"tls_cert_key"`
	TLSCertBody     string   `json:"tls_cert_body"`
}

type PTCPStep struct {
	variables          PTCPVariables
	backendConfig      TerraformAWSBucketBackendConfig
	RootPath           string
	KeepGeneratedFiles bool
	TerraformUtility   steps.TerraformUtility
	AWSUtility         AWSUtility
}

func CreatePTCPStep(rootPath string, keepGeneratedFiles bool, terraformUtility steps.TerraformUtility, awsUtility AWSUtility) *PTCPStep {
	return &PTCPStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: keepGeneratedFiles,
		TerraformUtility:   terraformUtility,
		AWSUtility:         awsUtility,
	}
}

func (s *PTCPStep) Name() string {
	return "PTCPStep"
}

func (s *PTCPStep) Labels() []string {
	return ptcpStepLabels
}

func (s *PTCPStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	s.variables = PTCPVariables{
		ClusterName:     config.Global.OrchName,
		Region:          config.AWS.Region,
		VPCID:           runtimeState.AWS.VPCID,
		SubnetIDs:       runtimeState.AWS.PrivateSubnetIDs,
		HTTPProxy:       config.Proxy.HTTPProxy,
		HTTPSProxy:      config.Proxy.HTTPSProxy,
		NoProxy:         config.Proxy.NoProxy,
		CustomerTag:     config.AWS.CustomerTag,
		Route53ZoneName: config.Global.ParentDomain,
		IPAllowList:     []string{DefaultNetworkCIDR}, // Only allow traffic from the VPC CIDR.
		CPU:             DefaultPTCPCPU,
		Memory:          DefaultPTCPMemory,
		DesiredCount:    DefaultPTCPDesiredCount,
		TLSCertKey:      runtimeState.Cert.TLSKey,
		TLSCertBody:     runtimeState.Cert.TLSCert,
	}
	s.backendConfig = TerraformAWSBucketBackendConfig{
		Region: config.AWS.Region,
		Bucket: config.Global.OrchName + "-" + runtimeState.DeploymentID,
		Key:    PTCPBackendBucketKey,
	}
	return runtimeState, nil
}

func (s *PTCPStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {

	return runtimeState, nil
}

func (s *PTCPStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {

	return runtimeState, nil
}

func (s *PTCPStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {

	return runtimeState, prevStepError
}
