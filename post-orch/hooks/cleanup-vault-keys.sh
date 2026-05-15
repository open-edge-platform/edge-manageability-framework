#!/bin/bash
# SPDX-FileCopyrightText: 2026 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0
# Pre-sync hook for secrets-config: cleans up stale vault-keys secret
# and stuck jobs so vault can be re-initialized cleanly.
#
# UPGRADE SAFE: Only preserves vault-keys when vault is BOTH running
# AND already unsealed (meaning previous secrets-config completed
# successfully). If vault is running but sealed/uninitialized, the
# old vault-keys are stale and must be deleted.

set -o pipefail

NS="orch-platform"

# Check if vault pod is running
vault_running=$(kubectl get pods -n "$NS" -l app.kubernetes.io/name=vault \
  --field-selector=status.phase=Running --no-headers 2>/dev/null | head -1)

# Check if vault is actually initialized and unsealed
vault_initialized=false
if [[ -n "$vault_running" ]]; then
  vault_pod=$(echo "$vault_running" | awk '{print $1}')
  # Query vault status — if initialized+unsealed, this is a real upgrade
  # (same approach as vault_unseal.sh uses to verify vault state)
  vault_status=$(kubectl exec -n "$NS" "$vault_pod" -c vault -- \
    vault status -format=json 2>/dev/null || true)
  if echo "$vault_status" | grep -q '"initialized": true' \
    && echo "$vault_status" | grep -q '"sealed": false'; then
    vault_initialized=true
  fi
  # Also verify vault-keys secret has valid unseal keys
  if [[ "$vault_initialized" == "true" ]]; then
    secret_json=$(kubectl -n "$NS" get secret vault-keys \
      -o jsonpath='{.data.vault-keys}' 2>/dev/null | base64 -d 2>/dev/null || true)
    keys=$(echo "$secret_json" | jq -r '.keys_base64[]?' 2>/dev/null)
    if [[ -z "$keys" ]]; then
      echo "⚠️  vault-keys secret exists but has no valid unseal keys — treating as stale"
      vault_initialized=false
    fi
  fi
fi

if [[ "$vault_initialized" == "true" ]]; then
  echo "✅ Vault is running and unsealed — preserving vault-keys (upgrade mode)"
else
  if kubectl get secret vault-keys -n "$NS" --no-headers 2>/dev/null | grep -q .; then
    echo "🧹 Deleting stale vault-keys secret (vault not initialized — fresh install)"
    kubectl delete secret vault-keys -n "$NS" --ignore-not-found 2>/dev/null || true
  fi
fi

# Delete any stuck secrets-config jobs
kubectl get jobs -n "$NS" --no-headers 2>/dev/null \
  | awk '$2!="1/1" && $1 ~ /^secrets-config/ {print $1}' \
  | while read -r job; do
    echo "🧹 Deleting stuck job $job in $NS"
    kubectl delete job "$job" -n "$NS" --ignore-not-found 2>/dev/null || true
  done

exit 0
