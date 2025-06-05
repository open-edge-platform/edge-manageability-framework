package steps_aws

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
)

const (
	ACMModulePath       = "new-installer/targets/aws/iac/acm"
	ACMBackendBucketKey = "acm.tfstate"
)

type ACMVariables struct {
	CertificateBody  string `json:"certificate_body" yaml:"certificate_body"`
	CertificateChain string `json:"certificate_chain" yaml:"certificate_chain"`
	PrivateKey       string `json:"private_key" yaml:"private_key"`
	ClusterName      string `json:"cluster_name" yaml:"cluster_name"`
	CustomerTag      string `json:"customer_tag" yaml:"customer_tag"`
	Region           string `json:"region" yaml:"region"`
}

// NewDefaultACMVariables creates a new ACMVariables instance with default values
// based on variable.tf default definitions.
func NewDefaultACMVariables() ACMVariables {
	return ACMVariables{
		CertificateBody:  "",
		CertificateChain: "",
		PrivateKey:       "",
		ClusterName:      "",
		CustomerTag:      "",
		Region:           "",
	}
}

type ImportCertificateToACM struct {
	variables          ACMVariables
	backendConfig      TerraformAWSBucketBackendConfig
	RootPath           string
	KeepGeneratedFiles bool
	TerraformUtility   steps.TerraformUtility
	StepLabels         []string
	AWSUtility         AWSUtility
}

func (s *ImportCertificateToACM) Name() string {
	return "ImportCertificateToACM"
}

func (s *ImportCertificateToACM) Labels() []string {
	return s.StepLabels
}

// to do: add code to create a self signed TLS certificate and CA if not provided
func (s *ImportCertificateToACM) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if config.Cert.TLSCert == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "TLSCert is not set",
		}
	}
	if config.Cert.TLSCA == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "TLSCA is not set",
		}
	}
	if config.Cert.TLSKey == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "TLSKey is not set",
		}
	}
	if config.AWS.CustomerTag == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "CustomerTag is not set",
		}
	}
	if config.AWS.Region == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "Region is not set",
		}
	}
	s.variables = NewDefaultACMVariables()
	s.variables = ACMVariables{
		CertificateBody:  config.Cert.TLSCert,
		CertificateChain: config.Cert.TLSCA,
		PrivateKey:       config.Cert.TLSKey,
		ClusterName:      config.Global.OrchName,
		CustomerTag:      config.AWS.CustomerTag,
		Region:           config.AWS.Region,
	}
	s.backendConfig = TerraformAWSBucketBackendConfig{
		Region: config.AWS.Region,
		Bucket: config.Global.OrchName + "-" + runtimeState.DeploymentID,
		Key:    ACMBackendBucketKey,
	}
	return runtimeState, nil
}

func (s *ImportCertificateToACM) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	// no-op for now
	return runtimeState, nil
}

func (s *ImportCertificateToACM) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if runtimeState.Action == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "Action is not set",
		}
	}
	terraformStepOutput, err := s.TerraformUtility.Run(ctx, steps.TerraformUtilityInput{
		Action:             runtimeState.Action,
		ModulePath:         filepath.Join(s.RootPath, ACMModulePath),
		Variables:          s.variables,
		BackendConfig:      s.backendConfig,
		LogFile:            filepath.Join(s.RootPath, ".logs", "aws_acm_import.log"),
		KeepGeneratedFiles: s.KeepGeneratedFiles,
	})
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
		if acmCertMeta, ok := terraformStepOutput.Output["cert"]; !ok {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeTerraform,
				ErrorMsg:  "The ACM certificate does not exist in terraform output",
			}
		} else {
			runtimeState.CertID = strings.Trim(string(acmCertMeta.Value), "\"")
		}
	} else {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeTerraform,
			ErrorMsg:  "cannot find any output from ACM module",
		}
	}
	return runtimeState, nil
}

func (s *ImportCertificateToACM) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, prevStepError
}
