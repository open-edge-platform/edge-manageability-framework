// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	goErr "errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/magefile/mage/sh"
)

const (
	Rke2KillAllScript   = "/usr/local/bin/rke2-killall.sh"
	Rke2UninstallScript = "/usr/local/bin/rke2-uninstall.sh"
)

func (Undeploy) rke2server() error {
	if _, err := os.Stat(Rke2KillAllScript); goErr.Is(err, fs.ErrNotExist) {
		fmt.Printf("RKE2 already uninstalled, nothing to be done...\n")
		return nil
	}

	// TODO: Return nil if no cluster exists.
	if err := sh.RunV("sudo", Rke2KillAllScript); err != nil {
		return err
	}
	return sh.RunV("sudo", Rke2UninstallScript)
}
