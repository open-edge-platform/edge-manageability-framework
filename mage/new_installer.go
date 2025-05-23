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
