#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
#
# Post-sync hook for postgresql-secrets: ensures PostgreSQL role passwords
# match the Kubernetes secrets after helm upgrade regenerates them.
#
# Problem: The postgresql-secrets chart uses randAlphaNum to generate new
# passwords on every helm upgrade. Even with cnpg.io/reload annotation,
# there's a race condition where application pods (Keycloak, etc.) restart
# with the new password before CNPG has synced it to PostgreSQL.
#
# Fix: After postgresql-secrets deploys, this hook reads each secret from
# orch-database namespace and ALTERs the PostgreSQL role password to match.

set -euo pipefail

PGCLUSTER_NS="orch-database"

# Check if any PostgreSQL pods exist (postgresql-cluster may not be deployed yet)
PGCLUSTER_POD=$(kubectl get pods -n "$PGCLUSTER_NS" -l cnpg.io/instanceRole=primary -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)

if [[ -z "$PGCLUSTER_POD" ]]; then
  # Fallback: look for role=primary label
  PGCLUSTER_POD=$(kubectl get pods -n "$PGCLUSTER_NS" -l role=primary --field-selector=status.phase=Running -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
fi

if [[ -z "$PGCLUSTER_POD" ]]; then
  echo "⏭️  No PostgreSQL primary pod found in $PGCLUSTER_NS — skipping password sync (postgresql-cluster not yet deployed)"
  exit 0
fi

# Wait for the primary pod to be ready
echo "⏳ Waiting for PostgreSQL primary pod to be ready..."
kubectl wait --for=condition=Ready pod "$PGCLUSTER_POD" -n "$PGCLUSTER_NS" --timeout=120s 2>/dev/null || {
  echo "⚠️  PostgreSQL primary pod $PGCLUSTER_POD not ready — skipping password sync"
  exit 0
}

echo "🔄 Syncing DB passwords to PostgreSQL (pod: $PGCLUSTER_POD)..."

# Find all managed basic-auth secrets in orch-database namespace
SECRETS=$(kubectl get secrets -n "$PGCLUSTER_NS" -l managed-by=edge-manageability-framework \
  -o jsonpath='{range .items[?(@.type=="kubernetes.io/basic-auth")]}{.metadata.name}{"\n"}{end}' 2>/dev/null)

if [[ -z "$SECRETS" ]]; then
  echo "ℹ️  No managed basic-auth secrets found — skipping"
  exit 0
fi

SYNCED=0
FAILED=0

for secret in $SECRETS; do
  USERNAME=$(kubectl get secret "$secret" -n "$PGCLUSTER_NS" -o jsonpath='{.data.username}' | base64 -d)
  PASSWORD_B64=$(kubectl get secret "$secret" -n "$PGCLUSTER_NS" -o jsonpath='{.data.password}')

  if [[ -z "$USERNAME" || -z "$PASSWORD_B64" ]]; then
    echo "  ⚠️  Skipping $secret — missing username or password"
    continue
  fi

  # Use a SQL command that decodes base64 inside psql to avoid shell escaping issues
  # with special characters in passwords
  SQL="DO \$\$ BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname='${USERNAME}') THEN
      EXECUTE format('ALTER ROLE %I WITH PASSWORD %L', '${USERNAME}', convert_from(decode('${PASSWORD_B64}', 'base64'), 'UTF8'));
    END IF;
  END \$\$;"

  if kubectl exec -n "$PGCLUSTER_NS" "$PGCLUSTER_POD" -c postgres -- \
    psql -U postgres -c "$SQL" 2>/dev/null; then
    # Check if the role actually existed
    ROLE_EXISTS=$(kubectl exec -n "$PGCLUSTER_NS" "$PGCLUSTER_POD" -c postgres -- \
      psql -U postgres -tAc "SELECT 1 FROM pg_roles WHERE rolname='$USERNAME'" 2>/dev/null)
    if [[ "$ROLE_EXISTS" == "1" ]]; then
      echo "  ✅ $USERNAME — password synced"
      SYNCED=$((SYNCED + 1))
    else
      echo "  ⏭️  $USERNAME — role does not exist yet (will be created by CNPG)"
    fi
  else
    echo "  ❌ $USERNAME — ALTER ROLE failed"
    FAILED=$((FAILED + 1))
  fi
done

echo "🔄 Password sync complete: $SYNCED synced, $FAILED failed"

# Update derived connection-string secrets (mps, rps) that are built from the
# *-local-postgresql secrets by the amt-dbpassword-secret-job. On rerun, the Job
# may not have re-run yet, leaving these secrets with stale passwords.
for app in mps rps; do
  SRC_SECRET="${app}-local-postgresql"
  if kubectl get secret "$SRC_SECRET" -n orch-infra &>/dev/null && \
     kubectl get secret "$app" -n orch-infra &>/dev/null; then
    PW=$(kubectl get secret "$SRC_SECRET" -n orch-infra -o jsonpath='{.data.PGPASSWORD}' | base64 -d)
    USER=$(kubectl get secret "$SRC_SECRET" -n orch-infra -o jsonpath='{.data.PGUSER}' | base64 -d)
    DB=$(kubectl get secret "$SRC_SECRET" -n orch-infra -o jsonpath='{.data.PGDATABASE}' | base64 -d)
    HOST=$(kubectl get secret "$SRC_SECRET" -n orch-infra -o jsonpath='{.data.PGHOST}' | base64 -d)
    PORT=$(kubectl get secret "$SRC_SECRET" -n orch-infra -o jsonpath='{.data.PGPORT}' | base64 -d)
    CONNSTR="postgresql://${USER}:${PW}@${HOST}:${PORT}/${DB}?search_path=public&sslmode=disable"

    # Check if connection string changed
    CURRENT=$(kubectl get secret "$app" -n orch-infra -o jsonpath='{.data.connectionString}' 2>/dev/null | base64 -d)
    if [[ "$CONNSTR" != "$CURRENT" ]]; then
      kubectl create secret generic "$app" \
        --from-literal=connectionString="$CONNSTR" \
        -n orch-infra --dry-run=client -o yaml | kubectl apply -f - &>/dev/null
      echo "  🔄 Updated $app connection-string secret"
      # Restart deployment to pick up new secret
      if kubectl get deploy "$app" -n orch-infra &>/dev/null; then
        kubectl rollout restart deploy "$app" -n orch-infra &>/dev/null
        echo "  🔄 Restarted $app deployment"
      fi
    fi
  fi
done

if (( FAILED > 0 )); then
  echo "⚠️  Some password syncs failed — roles may not exist yet (first install). Will be synced on next run."
fi
