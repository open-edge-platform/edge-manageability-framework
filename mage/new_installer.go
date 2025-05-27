// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"fmt"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

type NewInstaller mg.Namespace

func (NewInstaller) Build() error {
	if (NewInstaller{}).BuildInstaller() != nil {
		return fmt.Errorf("failed to build installer")
	}
	if (NewInstaller{}).BuildConfigBuilder() != nil {
		return fmt.Errorf("failed to build config builder")
	}
	return nil
}

func (NewInstaller) BuildInstaller() error {
	if err := sh.RunV("mkdir", "-p", "new-installer/_build"); err != nil {
		return err
	}
	// Build the new installer binary
	if err := sh.RunV("go", "build", "-C", "new-installer", "-o", "_build/orch-installer", "./cmd"); err != nil {
		return err
	}
	fmt.Println("Installer built successfully. Run ./new-installer/_build/orch-installer to start the installer.")
	return nil
}

func (NewInstaller) BuildConfigBuilder() error {
	if err := sh.RunV("mkdir", "-p", "new-installer/_build"); err != nil {
		return err
	}
	// Build the new installer binary
	if err := sh.RunV("go", "build", "-C", "new-installer", "-o", "_build/config-builder", "./cmd/config"); err != nil {
		return err
	}
	fmt.Println("Installer built successfully. Run ./new-installer/_build/config-builder to start the config builder.")
	return nil
}

func (NewInstaller) Test() error {
	if err := (NewInstaller{}).TestInstaller(); err != nil {
		return fmt.Errorf("failed to run installer tests: %w", err)
	}
	return nil
}

func (NewInstaller) TestInstaller() error {
	// Run tests for the new installer, except for the AWS IaC tests
	// Ginkgo flags:
	// -v: verbose output
	// -r: recursive test
	// -p: parallel test
	// --skip-package: skip tests in specific packages
	if err := sh.RunV("ginkgo", "-v", "-r", "-p", "--skip-package=new-installer/targets/aws/iac", "new-installer"); err != nil {
		return err
	}
	fmt.Println("Installer tests passed successfully.")
	return nil
}

// Test Terraform modules
func (NewInstaller) TestIaC() error {
	return nil
}
