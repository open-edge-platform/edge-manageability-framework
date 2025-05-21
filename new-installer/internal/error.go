// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package internal

type OrchInstallerErrorCode int

const (
	OrchInstallerErrorCodeUnknown OrchInstallerErrorCode = iota
	OrchInstallerErrorCodeInternal
	OrchInstallerErrorCodeInvalidArgument
	OrchInstallerErrorCodeTerraform
	OrchInstallerErrorCodeStateUploadFailed
)

type OrchInstallerError struct {
	ErrorCode OrchInstallerErrorCode
	ErrorMsg  string
}

func (e *OrchInstallerError) Error() string {
	return e.ErrorMsg
}

type OrchInstallerStageError struct {
	ErrorCode  OrchInstallerErrorCode
	ErrorMsg   string
	StepErrors map[string]*OrchInstallerError
}
