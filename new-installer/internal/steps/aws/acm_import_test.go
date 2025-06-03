// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package steps_aws_test

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	steps_aws "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/aws"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ACMImportTest struct {
	suite.Suite
	config       config.OrchInstallerConfig
	runtimeState config.OrchInstallerRuntimeState

	step              *steps_aws.ImportCertificateToACM
	randomText        string
	randomTLSCert     string
	randomTLSCA       string
	randomTLSKey      string
	randomCustomerTag string
	randomClusterName string
	logDir            string
	tfUtility         *MockTerraformUtility
	awsUtility        *MockAWSUtility
}

func TestACMImport(t *testing.T) {
	suite.Run(t, new(ACMImportTest))
}

func (s *ACMImportTest) SetupTest() {
	rootPath, err := filepath.Abs("../../../../")
	if err != nil {
		s.NoError(err)
		return
	}

	s.randomText = strings.ToLower(rand.Text()[0:8])
	s.logDir = filepath.Join(rootPath, ".logs")
	internal.InitLogger("debug", s.logDir)
	s.randomTLSCert = strings.ToLower(rand.Text()[0:8])
	s.randomTLSCA = strings.ToLower(rand.Text()[0:8])
	s.randomTLSKey = strings.ToLower(rand.Text()[0:8])
	s.randomCustomerTag = strings.ToLower(rand.Text()[0:8])
	s.randomClusterName = strings.ToLower(rand.Text()[0:8])
	s.runtimeState.LogDir = filepath.Join(rootPath, ".logs")

	// Initialize the config and runtime state
	s.config = config.OrchInstallerConfig{}
	s.config.AWS.Region = "us-west-2"
	s.config.Global.OrchName = "test"
	s.runtimeState.DeploymentID = s.randomText

	s.config.Cert.TLSCert = s.randomTLSCert
	s.config.Cert.TLSCA = s.randomTLSCA
	s.config.Cert.TLSKey = s.randomTLSKey
	s.config.AWS.CustomerTag = s.randomCustomerTag
	s.config.Global.OrchName = s.randomClusterName

	if _, err := os.Stat(s.logDir); os.IsNotExist(err) {
		err := os.MkdirAll(s.logDir, os.ModePerm)
		if err != nil {
			s.NoError(err)
			return
		}
	}

	s.tfUtility = &MockTerraformUtility{}
	s.awsUtility = &MockAWSUtility{}

	s.step = &steps_aws.ImportCertificateToACM{
		RootPath:           rootPath,
		KeepGeneratedFiles: true,
		TerraformUtility:   s.tfUtility,
		AWSUtility:         s.awsUtility,
	}
}

func (s *ACMImportTest) TestInstallAndUninstallACM() {
	s.runtimeState.Action = "install"
	s.expectTFUtiliyyCall("install")
	rs, err := steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
		return
	}

	s.Equal(rs.CertID, "acm-12345678")

	s.runtimeState.Action = "uninstall"
	s.expectTFUtiliyyCall("uninstall")
	_, err = steps.GoThroughStepFunctions(s.step, &s.config, s.runtimeState)
	if err != nil {
		s.NoError(err)
	}
}

func (s *ACMImportTest) expectTFUtiliyyCall(action string) {
	input := steps.TerraformUtilityInput{
		Action:             action,
		ModulePath:         filepath.Join(s.step.RootPath, steps_aws.ACMModulePath),
		LogFile:            filepath.Join(s.step.RootPath, ".logs", "aws_acm_import.log"),
		KeepGeneratedFiles: s.step.KeepGeneratedFiles,
		Variables: steps_aws.ACMVariables{
			Region:           s.config.AWS.Region,
			CertificateBody:  s.config.Cert.TLSCert,
			CertificateChain: s.config.Cert.TLSCA,
			PrivateKey:       s.config.Cert.TLSKey,
			ClusterName:      s.config.Global.OrchName,
			CustomerTag:      s.config.AWS.CustomerTag,
		},

		BackendConfig: steps_aws.TerraformAWSBucketBackendConfig{
			Region: s.config.AWS.Region,
			Bucket: fmt.Sprintf("%s-%s", s.config.Global.OrchName, s.runtimeState.DeploymentID),
			Key:    "acm.tfstate",
		},

		TerraformState: "",
	}
	if action == "install" {
		s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
			TerraformState: "",
			Output: map[string]tfexec.OutputMeta{
				"cert": {
					Type:  json.RawMessage(`"string"`),
					Value: json.RawMessage(`acm-12345678"`),
				},
			},
		}, nil).Once()
	} else {
		s.tfUtility.On("Run", mock.Anything, input).Return(steps.TerraformUtilityOutput{
			TerraformState: "",
		}, nil).Once()
	}
}
