// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps_aws

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
)

const (
	RDSModulePath       = "new-installer/targets/aws/iac/rds"
	RDSBackendBucketKey = "rds.tfstate"
)

var rdsStepLabels = []string{"aws", "rds"}

type RDSVariables struct {
	ClusterName               string   `json:"cluster_name" yaml:"cluster_name"`
	Region                    string   `json:"region" yaml:"region"`
	CustomerTag               string   `json:"customer_tag" yaml:"customer_tag"`
	SubnetIDs                 []string `json:"subnet_ids" yaml:"subnet_ids"`
	VPCID                     string   `json:"vpc_id" yaml:"vpc_id"`
	IPAllowList               []string `json:"ip_allow_list" yaml:"ip_allow_list"`
	AvailabilityZones         []string `json:"availability_zones" yaml:"availability_zones"`
	InstanceAvailabilityZones []string `json:"instance_availability_zones" yaml:"instance_availability_zones"`
	PostgresVerMajor          string   `json:"postgres_ver_major,omitempty" yaml:"postgres_ver_major,omitempty"`
	PostgresVerMinor          string   `json:"postgres_ver_minor,omitempty" yaml:"postgres_ver_minor,omitempty"`
	MinACUs                   float32  `json:"min_acus" yaml:"min_acus"`
	MaxACUs                   float32  `json:"max_acus" yaml:"max_acus"`
	DevMode                   bool     `json:"dev_mode" yaml:"dev_mode"`
	Username                  string   `json:"username,omitempty" yaml:"username,omitempty"`
	CACertIdentifier          string   `json:"ca_cert_identifier,omitempty" yaml:"ca_cert_identifier,omitempty"`
}

// NewDefaultRDSVariables creates a new RDSVariables with default values
// based on variable.tf default definitions.
func NewDefaultRDSVariables() RDSVariables {
	return RDSVariables{
		ClusterName:               "",
		Region:                    "",
		CustomerTag:               "",
		SubnetIDs:                 []string{},
		VPCID:                     "",
		IPAllowList:               []string{},
		AvailabilityZones:         []string{},
		InstanceAvailabilityZones: []string{},
		PostgresVerMajor:          "",
		PostgresVerMinor:          "",
		MinACUs:                   0,
		MaxACUs:                   0,
		DevMode:                   false,
		Username:                  "",
		CACertIdentifier:          "",
	}
}

type RDSStep struct {
	variables          RDSVariables
	backendConfig      TerraformAWSBucketBackendConfig
	RootPath           string
	KeepGeneratedFiles bool
	TerraformUtility   steps.TerraformUtility
	AWSUtility         AWSUtility
}

func CreateRDSStep(rootPath string, keepGeneratedFiles bool, terraformUtility steps.TerraformUtility, awsUtility AWSUtility) *RDSStep {
	return &RDSStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: keepGeneratedFiles,
		TerraformUtility:   terraformUtility,
		AWSUtility:         awsUtility,
	}
}

func (s *RDSStep) Name() string {
	return "RDSStep"
}
func (s *RDSStep) Labels() []string {
	return rdsStepLabels
}

func (s *RDSStep) ConfigStep(ctx context.Context, cfg config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	s.variables = NewDefaultRDSVariables()
	s.variables.ClusterName = cfg.Global.OrchName
	s.variables.Region = cfg.AWS.Region
	s.variables.CustomerTag = cfg.AWS.CustomerTag

	if len(runtimeState.AWS.PrivateSubnetIDs) == 0 {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidRuntimeState,
			ErrorMsg:  fmt.Sprintf("PrivateSubnetIDs should not be empty in runtime state for step %s", s.Name()),
		}
	}
	s.variables.SubnetIDs = runtimeState.AWS.PrivateSubnetIDs
	s.variables.IPAllowList = []string{DefaultNetworkCIDR}

	if runtimeState.AWS.VPCID == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidRuntimeState,
			ErrorMsg:  fmt.Sprintf("VPCID should not be empty in runtime state for step %s", s.Name()),
		}
	}
	s.variables.VPCID = runtimeState.AWS.VPCID

	switch cfg.Global.Scale {
	case config.Scale50:
		s.variables.MinACUs = 0.5
		s.variables.MaxACUs = 2
	case config.Scale100:
		s.variables.MinACUs = 0.5
		s.variables.MaxACUs = 2
	case config.Scale500:
		s.variables.MinACUs = 0.5
		s.variables.MaxACUs = 4
	case config.Scale1000:
		s.variables.MinACUs = 0.5
		s.variables.MaxACUs = 8
	}

	zones, err := s.AWSUtility.GetAvailableZones(cfg.AWS.Region)
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidRuntimeState,
			ErrorMsg:  fmt.Sprintf("failed to get available zones: %v", err),
		}
	}
	s.variables.AvailabilityZones = zones
	s.variables.InstanceAvailabilityZones = zones

	// TODO:
	s.variables.DevMode = false

	s.backendConfig = TerraformAWSBucketBackendConfig{
		Region: cfg.AWS.Region,
		Bucket: cfg.Global.OrchName + "-" + runtimeState.DeploymentID,
		Key:    RDSBackendBucketKey,
	}

	return runtimeState, nil
}

func (s *RDSStep) PreStep(ctx context.Context, cfg config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if cfg.AWS.PreviousS3StateBucket == "" {
		// No need to migrate state, since there is no previous state bucket
		return runtimeState, nil
	}

	// Need to move Terraform state from old bucket to new bucket:
	oldRDSBucketKey := fmt.Sprintf("%s/cluster/%s", cfg.AWS.Region, cfg.Global.OrchName)
	err := s.AWSUtility.S3CopyToS3(cfg.AWS.Region,
		cfg.AWS.PreviousS3StateBucket,
		oldRDSBucketKey,
		cfg.AWS.Region,
		s.backendConfig.Bucket,
		s.backendConfig.Key)
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to move Terraform state from old bucket to new bucket: %v", err),
		}
	}

	// Need to delete unrelevant states.
	// Anything that is not related to EKS should be deleted.
	modulePath := filepath.Join(s.RootPath, RDSModulePath)
	mvErr := s.TerraformUtility.MoveStates(ctx, steps.TerraformUtilityMoveStatesInput{
		ModulePath: modulePath,
		States: map[string]string{
			"module.aurora.aws_db_subnet_group.main":                "aws_db_subnet_group.main",
			"module.aurora.aws_rds_cluster.main":                    "aws_rds_cluster.main",
			"module.aurora.aws_rds_cluster_instance.main":           "aws_rds_cluster_instance.main",
			"module.aurora.aws_rds_cluster_parameter_group.default": "aws_rds_cluster_parameter_group.default",
			"module.aurora.aws_security_group.rds":                  "aws_security_group.rds",
		},
	})
	if mvErr != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to move Terraform states: %v", mvErr),
		}
	}

	rmErr := s.TerraformUtility.RemoveStates(ctx, steps.TerraformUtilityRemoveStatesInput{
		ModulePath: modulePath,
		States: []string{
			"module.s3",
			"module.eks",
			"module.efs",
			"module.aurora_database",
			"module.aurora_import",
			"module.kms",
			"module.orch_init",
			"module.eks_auth",
			"module.ec2log",
			"module.aws_lb_controller",
			"module.gitea",
		},
	})
	if rmErr != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to remove Terraform states: %v", rmErr),
		}
	}
	return runtimeState, nil
}

func (s *RDSStep) RunStep(ctx context.Context, cfg config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	terraformStepInput := steps.TerraformUtilityInput{
		Action:             runtimeState.Action,
		ModulePath:         filepath.Join(s.RootPath, RDSModulePath),
		Variables:          s.variables,
		BackendConfig:      s.backendConfig,
		LogFile:            filepath.Join(runtimeState.LogDir, "aws_rds.log"),
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
		if host, ok := terraformStepOutput.Output["host"]; ok {
			runtimeState.Database.Host = strings.Trim(string(host.Value), "\"")
		} else {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  fmt.Sprintf("cannot find host in %s module output", s.Name()),
			}
		}

		if readerHost, ok := terraformStepOutput.Output["host_reader"]; ok {
			runtimeState.Database.ReaderHost = strings.Trim(string(readerHost.Value), "\"")
		} else {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  fmt.Sprintf("cannot find host_reader in %s module output", s.Name()),
			}
		}

		if port, ok := terraformStepOutput.Output["port"]; ok {
			portNum, err := strconv.Atoi(strings.Trim(string(port.Value), "\""))
			if err != nil {
				return runtimeState, &internal.OrchInstallerError{
					ErrorCode: internal.OrchInstallerErrorCodeTerraform,
					ErrorMsg:  fmt.Sprintf("failed to convert port to integer: %v", err),
				}
			}
			runtimeState.Database.Port = portNum
		} else {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  fmt.Sprintf("cannot find port in %s module output", s.Name()),
			}
		}

		if username, ok := terraformStepOutput.Output["username"]; ok {
			runtimeState.Database.Username = strings.Trim(string(username.Value), "\"")
		} else {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  fmt.Sprintf("cannot find username in %s module output", s.Name()),
			}
		}
		if password, ok := terraformStepOutput.Output["password"]; ok {
			runtimeState.Database.Password = strings.Trim(string(password.Value), "\"")
		} else {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  fmt.Sprintf("cannot find password in %s module output", s.Name()),
			}
		}
	} else {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeTerraform,
			ErrorMsg:  fmt.Sprintf("cannot find any output from %s module", s.Name()),
		}
	}

	return runtimeState, nil
}

func (s *RDSStep) PostStep(ctx context.Context, cfg config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, prevStepError
}
