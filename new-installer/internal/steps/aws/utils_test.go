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
	} else {
		err = nil
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

func (m *MockTerraformUtility) MoveStates(ctx context.Context, input steps.TerraformUtilityMoveStatesInput) *internal.OrchInstallerError {
	args := m.Called(ctx, input)
	if err, ok := args.Get(0).(*internal.OrchInstallerError); ok {
		return err
	} else {
		// If the error is nil, return nil
		if args.Get(0) == nil {
			return nil
		}
	}
	if len(args) != 1 {
		return &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "Unexpected number of arguments",
		}
	}
	return nil
}

func (m *MockTerraformUtility) RemoveStates(ctx context.Context, input steps.TerraformUtilityRemoveStatesInput) *internal.OrchInstallerError {
	args := m.Called(ctx, input)
	if err, ok := args.Get(0).(*internal.OrchInstallerError); ok {
		return err
	}
	return nil
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

func (m *MockAWSUtility) S3CopyToS3(srcRegion, srcBucket, srcKey, destRegion, destBucket, destKey string) error {
	args := m.Called(srcRegion, srcBucket, srcKey, destRegion, destBucket, destKey)
	return args.Error(0)
}

func (m *MockAWSUtility) GetSubnetIDsFromVPC(region, vpcID string) ([]string, []string, error) {
	args := m.Called(region, vpcID)
	if len(args) != 3 {
		return nil, nil, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "Unexpected number of arguments",
		}
	}
	privateSubnets, ok1 := args.Get(0).([]string)
	publicSubnets, ok2 := args.Get(1).([]string)
	err, ok3 := args.Get(2).(error)
	if !ok1 || !ok2 || !ok3 {
		return nil, nil, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "Invalid argument types",
		}
	}
	return privateSubnets, publicSubnets, err
}

func (m *MockAWSUtility) DisableRDSDeletionProtection(region, dbIdentifier string) error {
	args := m.Called(region, dbIdentifier)
	return args.Error(0)
}
