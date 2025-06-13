// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps_common_test

import (
	"context"
	"errors"
	"os"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	"github.com/stretchr/testify/mock"
)

type ShellUtilityMock struct {
	mock.Mock
}

func (m *ShellUtilityMock) Run(ctx context.Context, input steps.ShellUtilityInput) (*steps.ShellUtilityOutput, *internal.OrchInstallerError) {
	args := m.Called(ctx, input)
	if err := args.Error(1); err != nil {
		var orchErr *internal.OrchInstallerError
		if errors.As(err, &orchErr) {
			return args.Get(0).(*steps.ShellUtilityOutput), orchErr
		}
	}
	return args.Get(0).(*steps.ShellUtilityOutput), nil
}

func (m *ShellUtilityMock) Process() *os.Process {
	args := m.Called()
	return args.Get(0).(*os.Process)
}

func (m *ShellUtilityMock) Kill() error {
	args := m.Called()
	return args.Error(0)
}

func (m *ShellUtilityMock) Wait() error {
	args := m.Called()
	return args.Error(0)
}
