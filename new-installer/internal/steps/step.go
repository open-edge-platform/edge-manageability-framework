// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps

import (
	"context"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
)

type OrchInstallerStep interface {
	Name() string
	ConfigStep(ctx context.Context, config internal.OrchInstallerConfig, runtimeState internal.OrchInstallerRuntimeState) (internal.OrchInstallerRuntimeState, *internal.OrchInstallerError)
	PreSetp(ctx context.Context, config internal.OrchInstallerConfig, runtimeState internal.OrchInstallerRuntimeState) (internal.OrchInstallerRuntimeState, *internal.OrchInstallerError)
	RunStep(ctx context.Context, config internal.OrchInstallerConfig, runtimeState internal.OrchInstallerRuntimeState) (internal.OrchInstallerRuntimeState, *internal.OrchInstallerError)
	PostStep(ctx context.Context, config internal.OrchInstallerConfig, runtimeState internal.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (internal.OrchInstallerRuntimeState, *internal.OrchInstallerError)
}
