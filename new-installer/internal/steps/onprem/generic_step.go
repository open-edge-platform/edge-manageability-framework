// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package onprem

import (
	"context"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
)

type GenerickStep struct {
	RootPath               string
	KeepGeneratedFiles     bool
	orchConfigReaderWriter config.OrchConfigReaderWriter
	StepLabels             []string
}

func CreateGenericStep(rootPath string, keepGeneratedFiles bool, orchConfigReaderWriter config.OrchConfigReaderWriter) *GenerickStep {
	return &GenerickStep{
		RootPath:               rootPath,
		KeepGeneratedFiles:     keepGeneratedFiles,
		orchConfigReaderWriter: orchConfigReaderWriter,
	}
}

func (s *GenerickStep) Name() string {
	return "GenericStep"
}

func (s *GenerickStep) Labels() []string {
	return s.StepLabels
}

func (s *GenerickStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

func (s *GenerickStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	// no-op for now
	return runtimeState, nil
}

func (s *GenerickStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

func (s *GenerickStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, prevStepError
}
