// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Namespace contains prerequisite targets.
type Prereq mg.Namespace

// Ensures all Git LFS artifacts were fetched.
func (Prereq) GitLFSPull() error {
	return sh.RunV("git", "lfs", "pull")
}
