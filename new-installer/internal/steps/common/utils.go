// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps_common

import (
	"context"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
)

func commandExists(ctx context.Context, shellUtility steps.ShellUtility, command string) bool {
	_, err := shellUtility.Run(ctx, steps.ShellUtilityInput{
		Command:         []string{"which", command},
		Timeout:         60,
		SkipError:       false,
		RunInBackground: false,
	})
	return err == nil
}
