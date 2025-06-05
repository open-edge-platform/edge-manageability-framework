// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package steps_aws_test

import (
	"context"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	"github.com/stretchr/testify/mock"
)

type MockTerraformUtility struct {
	mock.Mock
}

func (m *MockTerraformUtility) Run(ctx context.Context, input steps.TerraformUtilityInput) (steps.TerraformUtilityOutput, *internal.OrchInstallerError) {
	args := m.Called(ctx, input)
	var err *internal.OrchInstallerError
	if e, ok := args.Get(1).(*internal.OrchInstallerError); ok {
		err = e
	}

	if output, ok := args.Get(0).(steps.TerraformUtilityOutput); ok {
		return output, err
	}
	if len(args) != 2 {
		return steps.TerraformUtilityOutput{}, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "Unexpected number of arguments",
		}
	}
	return steps.TerraformUtilityOutput{}, args.Get(1).(*internal.OrchInstallerError)
}

type MockAWSUtility struct {
	mock.Mock
}

func (m *MockAWSUtility) GetAvailableZones(region string) ([]string, error) {
	args := m.Called(region)
	if zones, ok := args.Get(0).([]string); ok {
		return zones, args.Error(1)
	}
	if len(args) != 2 {
		return nil, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "Unexpected number of arguments",
		}
	}
	return nil, args.Error(1)
}
