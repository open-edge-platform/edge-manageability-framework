#!/bin/bash
# SPDX-FileCopyrightText: 2026 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0
# Pre-sync hook for secrets-config: cleans up stale vault-keys secret
# and stuck jobs so vault can be re-initialized cleanly.
# UPGRADE SAFE: Only deletes vault-keys when vault is NOT running
# (fresh install or failed install). On upgrades, vault-keys contains
# real unseal keys and must be preserved.
set -o pipefail
NS="orch-platform"
# Only delete vault-keys on fresh install (vault pod not running)
vault_running=$(kubectl get pods -n "$NS" -l app.kubernetes.io/name=vault \
  --field-selector=status.phase=Running --no-headers 2>/dev/null | head -1)
if [[ -z "$vault_running" ]]; then
  if kubectl get secret vault-keys -n "$NS" --no-headers 2>/dev/null | grep -q .; then
    echo "🧹 Deleting stale vault-keys secret (vault not running — fresh install)"
    kubectl delete secret vault-keys -n "$NS" --ignore-not-found 2>/dev/null || true
  fi
else
  echo "✅ Vault is running — preserving vault-keys secret (upgrade mode)"
fi
# Delete any stuck secrets-config jobs
kubectl get jobs -n "$NS" --no-headers 2>/dev/null \
  | awk '$2!="1/1" && $1 ~ /^secrets-config/ {print $1}' \
  | while read -r job; do
    echo "🧹 Deleting stuck job $job in $NS"
    kubectl delete job "$job" -n "$NS" --ignore-not-found 2>/dev/null || true
  done
exit 0
