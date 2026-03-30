#!/bin/bash
set -euo pipefail
# ═══════════════════════════════════════════════════════════════
# pre-install.sh — Helmfile presync hook for app-specific cleanup
#
# Called by helmfile hooks before installing certain releases.
# Usage: ./hooks/pre-install.sh <app-name> <namespace>
# ═══════════════════════════════════════════════════════════════

APP="${1:-}"
NS="${2:-}"

if [[ -z "$APP" || -z "$NS" ]]; then
  echo "Usage: $0 <app-name> <namespace>" >&2
  exit 1
fi

remove_stale_vault_keys() {
  local ns="$1"
  if kubectl get secret vault-keys -n "$ns" >/dev/null 2>&1; then
    echo "   🔑 Removing stale vault-keys secret (will be recreated by secrets-config)"
    kubectl delete secret vault-keys -n "$ns" 2>/dev/null || true
  fi
}

case "$APP" in
  vault)
    remove_stale_vault_keys "$NS"

    # Truncate PG tables to force a clean reinit
    pg_ns="orch-database"
    pg_pod=$(kubectl get pods -n "$pg_ns" -l app.kubernetes.io/name=postgres \
      -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    if [[ -n "$pg_pod" ]]; then
      echo "   🗄️  Truncating vault tables in PostgreSQL for clean reinit"
      kubectl exec "$pg_pod" -n "$pg_ns" -c postgres -- \
        psql -U postgres -d vault -c "TRUNCATE vault_kv_store, vault_ha_locks;" 2>/dev/null || true
    fi
    ;;
  secrets-config|orch-utils)
    remove_stale_vault_keys "$NS"
    ;;
  *)
    # No pre-install action needed
    ;;
esac
