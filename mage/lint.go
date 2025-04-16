// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"github.com/magefile/mage/sh"
)

func (Lint) golang() error {
	return sh.RunV("golangci-lint", "run", "--config", ".golangci.yml", "-v", "--timeout", "5m0s")
}

func (Lint) helm() error {
	if err := sh.RunV("helm", "lint", "argocd/root-app", "-f", "argocd/root-app/values.yaml"); err != nil {
		return err
	}

	if err := sh.RunV("helm", "lint", "argocd/applications", "-f", "argocd/root-app/values.yaml"); err != nil {
		return err
	}

	if err := sh.RunV("helm", "lint", "argocd-internal/root-app", "-f", "argocd-internal/root-app/values.yaml"); err != nil {
		return err
	}

	if err := sh.RunV("helm", "lint", "argocd-internal/applications", "-f", "argocd-internal/root-app/values.yaml"); err != nil {
		return err
	}

	return nil
}

func (Lint) yaml() error {
	return sh.RunV("yamllint", "-c", "tools/yamllint-conf.yaml", "argocd/applications/configs")
}

// Lint Terraform source files.
func (Lint) Terraform() error {
	return sh.RunV("tflint", "--init", "--chdir=terraform")
}

// Lint markdown files using markdownlint-cli2 tool.
func (Lint) Markdown() error {
	return sh.RunV("markdownlint-cli2", "**/*.md", "--config", "tools/.markdownlint-cli2.yaml")
}
