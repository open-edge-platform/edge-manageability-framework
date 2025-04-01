// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package retry

import (
	"context"
	"time"
)

// UntilItSucceeds will retry the action at interval until it returns nil or the context is canceled. Any logging should
// be done in the action func itself.
func UntilItSucceeds(ctx context.Context, action func() error, retryInterval time.Duration) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		default:
			// no-op
		}

		// Return if action succeeds 🙏
		if err := action(); err == nil {
			return nil
		}

		// Action errored, attempt action again after duration elapses ⏲️
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-time.After(retryInterval):
			// Wheeee, here we go 🎢
		}
	}
}
