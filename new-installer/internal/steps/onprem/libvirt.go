// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package onprem

import (
	"context"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
)

type LibvirtStep struct {
	RootPath               string
	KeepGeneratedFiles     bool
	orchConfigReaderWriter config.OrchConfigReaderWriter
	StepLabels             []string
}

func CreateLibvirtStep(rootPath string, keepGeneratedFiles bool, orchConfigReaderWriter config.OrchConfigReaderWriter) *LibvirtStep {
	return &LibvirtStep{
		RootPath:               rootPath,
		KeepGeneratedFiles:     keepGeneratedFiles,
		orchConfigReaderWriter: orchConfigReaderWriter,
	}
}

func (s *LibvirtStep) Name() string {
	return "LibvirtInfraStep"
}

func (s *LibvirtStep) Labels() []string {
	return s.StepLabels
}

func (s *LibvirtStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

func (s *LibvirtStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	// no-op for now
	return runtimeState, nil
}

func (s *LibvirtStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {

	return runtimeState, nil
}

func (s *LibvirtStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, prevStepError
}
