// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bitfield/script"
	"github.com/magefile/mage/sh"

	"github.com/open-edge-platform/edge-manageability-framework/internal/retry"
)

const vaultKeysFile = "vault-keys.json"

func (Vault) keys() error {
	if err := waitForVaultKeysSecret(); err != nil {
		return err
	}
	cmd := fmt.Sprintf(
		"kubectl --v=%d -n orch-platform get secret vault-keys -o jsonpath='{.data.vault-keys}'",
		verboseLevel,
	)
	if _, err := script.
		NewPipe().
		Exec(cmd).
		Exec("base64 -d").
		Exec("jq ."). // Use actual jq to pretty print
		WriteFile(vaultKeysFile); err != nil {
		return fmt.Errorf("vault init: %w ", err)
	}
	fmt.Printf("Wrote Vault keys to file 🔒: %s\n", vaultKeysFile)

	return nil
}

func waitForVaultKeysSecret() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fn := func() error {
		return sh.RunV("kubectl", "get", "secret", "-n",
			"orch-platform", "vault-keys")
	}
	if err := retry.UntilItSucceeds(ctx, fn, 3*time.Second); err != nil {
		return fmt.Errorf("vault keys secret error: %w 😲", err)
	}
	return nil
}

func (Vault) unseal() error {
	if err := waitForVaultKeysSecret(); err != nil {
		return err
	}
	cmd := "kubectl -n orch-platform get secret vault-keys -o jsonpath='{.data.vault-keys}'"

	keys, err := script.Exec(cmd).Exec("base64 -d").JQ(".keys_base64 | .[]").Slice()
	if err != nil {
		return fmt.Errorf("executing kubectl pipeline: %w", err)
	}

	for _, key := range keys {
		if err := sh.RunV(
			"kubectl",
			"-n", "orch-platform",
			"exec",
			"-it",
			"vault-0",
			"--",
			"vault", "operator", "unseal",
			strings.Trim(key, "\""), // Strip double-quotes from the string
		); err != nil {
			return fmt.Errorf("apply unseal key share: %w", err)
		}
	}
	fmt.Printf("\nVault unsealed 🔓\n")

	return nil
}
