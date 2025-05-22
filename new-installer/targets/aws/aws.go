// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
)

func CreateAWSStages(rootPath string, keepGeneratedFiles bool) ([]internal.OrchInstallerStage, error) {
	return []internal.OrchInstallerStage{
		NewAWSStage("PreInfra", []steps.OrchInstallerStep{}),
		NewAWSStage("Infra", []steps.OrchInstallerStep{}),
		NewAWSStage("PreOrch", []steps.OrchInstallerStep{}),
		NewAWSStage("Orch", []steps.OrchInstallerStep{}),
		NewAWSStage("OrchInit", []steps.OrchInstallerStep{}),
	}, nil
}
