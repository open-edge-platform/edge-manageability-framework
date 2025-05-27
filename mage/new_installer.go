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

func (NewInstaller) Test() error {
	// Run tests for the new installer, except for the AWS IaC tests.
	if err := sh.RunV("ginkgo", "-v", "-r", "-p", "--skip-package=new-installer/targets/aws/iac/tests", "new-installer"); err != nil {
		return err
	}
	fmt.Println("Installer tests passed successfully.")
	return nil
}

func (NewInstaller) TestIaC() error {
	// Test Terraform modules only.
	if err := sh.RunV("ginkgo", "-v", "-r", "new-installer/targets/aws/iac/tests"); err != nil {
		return fmt.Errorf("failed to run IaC tests: %w", err)
	}
	fmt.Println("IaC tests passed successfully.")
	return nil
}
