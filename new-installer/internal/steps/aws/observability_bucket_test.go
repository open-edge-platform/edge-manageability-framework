// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package steps_aws_test

import (
	"crypto/rand"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	steps_aws "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/aws"

	"github.com/stretchr/testify/suite"
)

type ObservabilityBucketsStepTest struct {
	suite.Suite
	config            config.OrchInstallerConfig
	step              *steps_aws.ObservabilityBucketsStep
	randomText        string
	logDir            string
	terraformExecPath string
}

func TestObservabilityBucketsStep(t *testing.T) {
	suite.Run(t, new(ObservabilityBucketsStepTest))
}

func (s *ObservabilityBucketsStepTest) SetupTest() {
	rootPath, err := filepath.Abs("../../../../")
	if err != nil {
		s.NoError(err)
		return
	}
	s.randomText = strings.ToLower(rand.Text()[0:8])
	s.logDir = filepath.Join(rootPath, ".logs")
	internal.InitLogger("debug", s.logDir)
	s.config.AWS.Region = "us-west-2"
	s.config.Global.OrchName = "piracki-test"

	s.terraformExecPath, err = steps.InstallTerraformAndGetExecPath()
	if err != nil {
		s.NoError(err)
		return
	}

	s.step = &steps_aws.ObservabilityBucketsStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: true,
		TerraformExecPath:  s.terraformExecPath,
	}
}

func (s *ObservabilityBucketsStepTest) TearDownTest() {
	// We will always uninstall VPC module
	s.config.Generated.Action = "uninstall"
	_, err := steps.GoThroughStepFunctions(s.step, &s.config)
	if err != nil {
		s.NoError(err)
	}
}

func (s *ObservabilityBucketsStepTest) TestInstallVPC() {
	s.config.Generated.Action = "install"
	rs, err := steps.GoThroughStepFunctions(s.step, &s.config)
	if err != nil {
		s.NoError(err)
		return
	}
}
