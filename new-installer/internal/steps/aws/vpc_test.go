// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package steps_aws_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	steps_aws "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/aws"

	"github.com/stretchr/testify/suite"
)

type VPCStepTest struct {
	suite.Suite
	config       internal.OrchInstallerConfig
	runtimeState internal.OrchInstallerRuntimeState
	step         *steps_aws.AWSVPCStep
}

func TestVPCStep(t *testing.T) {
	suite.Run(t, new(VPCStepTest))
}

func (s *VPCStepTest) SetupTest() {
	randomText := rand.Text()[0:8]
	s.config = internal.OrchInstallerConfig{
		DeploymentName:          fmt.Sprintf("test-%s", randomText),
		Region:                  "us-west-2",
		NetworkCIDR:             "10.250.0.0/8",
		StateStoreBucketPostfix: randomText,
		JumpHostIPAllowList:     []string{"10.250.0.0/8"},
		CustomerTag:             "unit-test",
	}
	s.runtimeState = internal.OrchInstallerRuntimeState{
		Mutex:  &sync.Mutex{},
		Action: "install",
		LogDir: ".log",
	}
	rootPath, err := filepath.Abs("../../../../")
	s.NoError(err)

	s.step = &steps_aws.AWSVPCStep{
		RootPath:           rootPath,
		KeepGeneratedFiles: true,
	}
}

func (s *VPCStepTest) TearDownTest() {
	// We will always uninstall VPC module
	s.runtimeState.Action = "uninstall"
	s.goThroughStepFunctions()
}

func (s *VPCStepTest) TestInstallVPC() {
	s.goThroughStepFunctions()
}

func (s *VPCStepTest) goThroughStepFunctions() {
	ctx := context.Background()
	newRS, err := s.step.ConfigStep(ctx, s.config, s.runtimeState)
	s.NoError(err)
	err = s.runtimeState.UpdateRuntimeState(newRS)
	s.NoError(err)

	newRS, err = s.step.PreStep(ctx, s.config, s.runtimeState)
	s.NoError(err)
	err = s.runtimeState.UpdateRuntimeState(newRS)
	s.NoError(err)

	newRS, err = s.step.RunStep(ctx, s.config, s.runtimeState)
	s.NoError(err)
	err = s.runtimeState.UpdateRuntimeState(newRS)
	s.NoError(err)

	newRS, err = s.step.PostStep(ctx, s.config, s.runtimeState, err)
	s.NoError(err)
	err = s.runtimeState.UpdateRuntimeState(newRS)
	s.NoError(err)
}
