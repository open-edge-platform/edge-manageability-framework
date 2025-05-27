// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package on_prem

import (
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	steps_on_prem "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/on_prem"
)

func CreateOnPremStages(rootPath string, keepGeneratedFiles bool, orchConfigReaderWriter config.OrchConfigReaderWriter) ([]internal.OrchInstallerStage, error) {
	return []internal.OrchInstallerStage{
		NewOnPremStage("Infra", []steps.OrchInstallerStep{
			&steps_on_prem.OnPremNetworkStep{},
			&steps_on_prem.OnPremVMStep{},
			&steps_on_prem.OnPremRKE2Step{},
			&steps_on_prem.OnPremMetalLBStep{},
			&steps_on_prem.OnPremPostgresqlStep{},
		}, []string{"infra"}, orchConfigReaderWriter),
	}, nil
}
