// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps_aws

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"path/filepath"
	"strings"
	"time"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
)

const (
	ACMModulePath       = "new-installer/targets/aws/iac/acm"
	ACMBackendBucketKey = "acm.tfstate"
)

var AWSACMStepLabels = []string{
	"aws",
	"acm_import",
}

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

type ImportCertificateToACMStep struct {
	variables          ACMVariables
	backendConfig      TerraformAWSBucketBackendConfig
	RootPath           string
	KeepGeneratedFiles bool
	TerraformUtility   steps.TerraformUtility
	StepLabels         []string
	AWSUtility         AWSUtility
}

func CreateImportCertificateToACMStep(rootPath string, keepGeneratedFiles bool, terraformUtility steps.TerraformUtility, awsUtility AWSUtility) *ImportCertificateToACMStep {
	return &ImportCertificateToACMStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: keepGeneratedFiles,
		TerraformUtility:   terraformUtility,
		AWSUtility:         awsUtility,
		StepLabels:         AWSACMStepLabels,
	}
}

func (s *ImportCertificateToACMStep) Name() string {
	return "ImportCertificateToACMStep"
}

func (s *ImportCertificateToACMStep) Labels() []string {
	return s.StepLabels
}

func (s *ImportCertificateToACMStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
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

	// Generate TLS Certificate and Private Key if not provided
	if config.Cert.TLSCert == "" || config.Cert.TLSCA == "" || config.Cert.TLSKey == "" {
		tlsCert, tlsCA, tlsKey, err := GenerateSelfSignedTLSCert(config.Global.OrchName)
		if err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorMsg:  fmt.Sprintf("failed to generate self-signed TLS certificate: %v", err),
			}
		}
		config.Cert.TLSCert = tlsCert
		config.Cert.TLSCA = tlsCA
		config.Cert.TLSKey = tlsKey
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

func (s *ImportCertificateToACMStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if config.AWS.PreviousS3StateBucket == "" {
		// No need to migrate state, since there is no previous state bucket
		return runtimeState, nil
	}

	// Need to move Terraform state from old bucket to new bucket:
	oldACMBucketKey := fmt.Sprintf("%s/orch-load-balancer/%s", config.AWS.Region, config.Global.OrchName)
	err := s.AWSUtility.S3CopyToS3(config.AWS.Region,
		config.AWS.PreviousS3StateBucket,
		oldACMBucketKey,
		config.AWS.Region,
		s.backendConfig.Bucket,
		s.backendConfig.Key)
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to move Terraform state from old ACM bucket to new ACM bucket: %v", err),
		}
	}
	modulePath := filepath.Join(s.RootPath, ACMModulePath)
	states := map[string]string{
		"module.acm_import.aws_acm_certificate.main": "aws_acm_certificate.main",
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

	rmErr := s.TerraformUtility.RemoveStates(ctx, steps.TerraformUtilityRemoveStatesInput{
		ModulePath: modulePath,
		States: []string{
			"module.traefik_load_balancer",
			"module.traefik2_load_balancer",
			"module.argocd_load_balancer",
			"module.traefik_lb_target_group_binding",
			"module.aws_lb_security_group_roles",
			"module.wait_until_alb_ready",
			"module.waf_web_acl_traefik",
			"module.waf_web_acl_argocd",
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

func (s *ImportCertificateToACMStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
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
			runtimeState.AWS.CertID = strings.Trim(string(acmCertMeta.Value), "\"")
		}
	} else {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeTerraform,
			ErrorMsg:  "cannot find any output from ACM module",
		}
	}
	return runtimeState, nil
}

func (s *ImportCertificateToACMStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, prevStepError
}

// GenerateSelfSignedTLSCert generates a self-signed TLS certificate, CA certificate, and private key.
// Returns the leaf certificate, CA certificate, and private key as PEM-encoded strings.
// This CA is not an external or trusted third-party CA, but a local, self-signed CA created on the fly. The leaf (end-entity) certificate
// is then signed by this self-generated CA, establishing a trust chain between the two. Both the CA certificate
// This approach is typical for development, testing, or internal use where a trusted CA is not required.
func GenerateSelfSignedTLSCert(commonName string) (tlsCertPEM string, tlsCAPEM string, keyPEM string, err error) {
	// Generate CA private key
	caPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", "", err
	}

	// Create CA certificate template
	caSerialNumber, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return "", "", "", err
	}
	caTemplate := x509.Certificate{
		SerialNumber: caSerialNumber,
		Subject: pkix.Name{
			CommonName: commonName + "-CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour), // CA valid for 10 years
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	// Self-sign CA certificate
	caDER, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caPriv.PublicKey, caPriv)
	if err != nil {
		return "", "", "", err
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})

	// Generate leaf private key (reuse CA key for simplicity, or generate a new one if preferred)
	leafPriv := caPriv

	// Create leaf certificate template
	leafSerialNumber, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return "", "", "", err
	}
	leafTemplate := x509.Certificate{
		SerialNumber: leafSerialNumber,
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // valid for 1 year
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	// Sign leaf certificate with CA
	leafDER, err := x509.CreateCertificate(rand.Reader, &leafTemplate, &caTemplate, &leafPriv.PublicKey, caPriv)
	if err != nil {
		return "", "", "", err
	}
	leafPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})

	// PEM encode the private key
	keyBytes, err := x509.MarshalECPrivateKey(leafPriv)
	if err != nil {
		return "", "", "", err
	}
	keyPEMBytes := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	return string(leafPEM), string(caPEM), string(keyPEMBytes), nil
}
