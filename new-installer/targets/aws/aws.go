// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws

import "github.com/open-edge-platform/edge-manageability-framework/installer/internal"

func CreateAWSStages(rootPath string, keepGeneratedFiles bool) []internal.OrchInstallerStage {
	return []internal.OrchInstallerStage{
		NewPreInfraStage(rootPath, keepGeneratedFiles),
		NewInfraStage(rootPath, keepGeneratedFiles),
		NewPreOrchStage(rootPath, keepGeneratedFiles),
		NewOrchStage(rootPath, keepGeneratedFiles),
		NewPostOrchStage(rootPath, keepGeneratedFiles),
	}
}
