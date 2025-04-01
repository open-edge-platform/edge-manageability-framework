// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package retry_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/open-edge-platform/edge-manageability-framework/internal/retry"
)

var _ = Describe("UntilItSucceeds", func() {
	It("should return nil immediately if an action succeeds", func() {
		Expect(retry.UntilItSucceeds(
			context.Background(),
			func() error {
				return nil
			},
			0,
		)).To(Succeed())
	})

	Context("Context is canceled before the action is executed", func() {
		It("should return the context error", func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			Expect(retry.UntilItSucceeds(
				ctx,
				func() error {
					return nil
				},
				0,
			)).To(MatchError(ctx.Err()))
		})
	})

	Context("Action fails on the first attempt and succeeds on the second attempt", func() {
		It("should return nil", func() {
			var attempt int

			Expect(retry.UntilItSucceeds(
				context.Background(),
				func() error {
					attempt++

					if attempt == 1 {
						return fmt.Errorf("Failing on the first attempt")
					}

					return nil
				},
				0,
			)).To(Succeed())
		})
	})
})
