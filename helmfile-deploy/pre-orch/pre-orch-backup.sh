#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2026 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Pre-upgrade backup for Edge Orchestrator (helmfile-based deployment).
# Creates backups of all critical data before running the upgrade.
#
# Backup artifacts are written to helmfile-deploy/upgrade-backup/ so they
# can be consumed by post-orch-deploy.sh upgrade.
#
# Usage:
#   ./pre-orch-backup.sh           # Run all backup phases
#   ./pre-orch-backup.sh -h        # Show help

set -euo pipefail

export PATH="/usr/local/bin:${PATH}"
export KUBECONFIG="${KUBECONFIG:-/home/${SUDO_USER:-$USER}/.kube/config}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HELMFILE_DEPLOY_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
POST_ORCH_DIR="$HELMFILE_DEPLOY_DIR/post-orch"

# Shared backup directory — consumed by post-orch-deploy.sh upgrade
UPGRADE_BACKUP_DIR="$HELMFILE_DEPLOY_DIR/upgrade-backup"

# Source post-orch.env for EMF_ variables
if [[ -f "$POST_ORCH_DIR/post-orch.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$POST_ORCH_DIR/post-orch.env"
  set +a
else
  echo "❌ post-orch.env not found at $POST_ORCH_DIR/post-orch.env"
  exit 1
fi

################################
# PostgreSQL settings
################################

POSTGRES_NAMESPACE="orch-database"
POSTGRES_USERNAME="postgres"
POSTGRES_POD="postgresql-cluster-1"
POSTGRES_BACKUP_FILE="${POSTGRES_NAMESPACE}_backup.sql"
POSTGRES_BACKUP_PATH="${UPGRADE_BACKUP_DIR}/${POSTGRES_BACKUP_FILE}"

################################
# Logging
################################

LOG_DIR="$SCRIPT_DIR/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/pre-orch-backup_$(date +'%Y%m%d_%H%M%S').log"

exec > >(tee -a "$LOG_FILE") 2>&1

log_info()  { echo "[$(date +'%Y-%m-%d %H:%M:%S')] INFO:  $*"; }
log_warn()  { echo "[$(date +'%Y-%m-%d %H:%M:%S')] WARN:  $*"; }
log_error() { echo "[$(date +'%Y-%m-%d %H:%M:%S')] ERROR: $*"; }

log_info "Starting pre-orch-backup"
log_info "Log file: $LOG_FILE"
log_info "Backup directory: $UPGRADE_BACKUP_DIR"

################################
# Prerequisites
################################

check_prerequisites() {
  log_info "Checking prerequisites..."

  if ! command -v kubectl &>/dev/null; then
    log_error "kubectl not found"
    exit 1
  fi

  if ! kubectl cluster-info &>/dev/null; then
    log_error "Cannot reach Kubernetes cluster. Check KUBECONFIG."
    exit 1
  fi

  mkdir -p "$UPGRADE_BACKUP_DIR"
  log_info "Prerequisites met."
}

################################
# Phase 1: PostgreSQL Health Check
################################

phase1_postgres_health() {
  log_info "=== Phase 1: PostgreSQL health check ==="

  if [[ -f "$POSTGRES_BACKUP_PATH" ]]; then
    log_warn "Existing backup found: $POSTGRES_BACKUP_PATH"
    read -rp "Backup already exists. Type 'Continue' to overwrite or Ctrl-C to abort: " confirm
    if [[ ! "$confirm" =~ ^[Cc][Oo][Nn][Tt][Ii][Nn][Uu][Ee]$ ]]; then
      log_error "User aborted."
      exit 1
    fi
    log_info "Continuing with existing backup (recovery mode)."
    return 0
  fi

  local pod_status
  pod_status=$(kubectl get pods -n "$POSTGRES_NAMESPACE" "$POSTGRES_POD" \
    -o jsonpath='{.status.phase}' 2>/dev/null || true)

  if [[ "$pod_status" != "Running" ]]; then
    log_error "PostgreSQL pod $POSTGRES_POD is not running (status: ${pod_status:-not found})."
    exit 1
  fi

  log_info "PostgreSQL pod $POSTGRES_POD is healthy (Running)."
}

################################
# Phase 2: PostgreSQL Superuser Secret
################################

phase2_postgres_secret() {
  log_info "=== Phase 2: Backing up PostgreSQL superuser secret ==="

  local secret_file="${UPGRADE_BACKUP_DIR}/postgres_secret.yaml"
  if [[ -f "$secret_file" ]]; then
    log_info "postgres_secret.yaml already exists, skipping."
    return 0
  fi

  if kubectl get secret -n "$POSTGRES_NAMESPACE" postgresql-cluster-superuser >/dev/null 2>&1; then
    kubectl get secret -n "$POSTGRES_NAMESPACE" postgresql-cluster-superuser \
      -o yaml > "$secret_file"
    log_info "PostgreSQL superuser secret saved."
  else
    log_warn "postgresql-cluster-superuser secret not found, skipping."
  fi
}

################################
# Phase 3: PostgreSQL Database Dump
################################

phase3_postgres_dump() {
  log_info "=== Phase 3: Backing up PostgreSQL databases ==="

  if [[ -f "$POSTGRES_BACKUP_PATH" ]]; then
    log_info "Backup file already exists, skipping dump."
    return 0
  fi

  log_info "Running pg_dumpall on pod $POSTGRES_POD..."

  local remote_path="/var/lib/postgresql/data/${POSTGRES_BACKUP_FILE}"

  # Temporarily disable md5 auth for dump (find pg_hba.conf dynamically)
  kubectl exec -n "$POSTGRES_NAMESPACE" "$POSTGRES_POD" -- /bin/bash -c '
    hba=$(find / -name pg_hba.conf -path "*/data/*" 2>/dev/null | head -1)
    if [[ -n "$hba" ]]; then
      cp "$hba" "${hba}.bak"
      sed -i "s/^\([^#]*\)md5/\1trust/g" "$hba"
      pg_ctl reload -D "$(dirname "$hba")" 2>/dev/null || true
    fi
  '

  if kubectl exec -n "$POSTGRES_NAMESPACE" "$POSTGRES_POD" -- /bin/bash -c \
    "pg_dumpall -U $POSTGRES_USERNAME -f '$remote_path'"; then
    kubectl cp "$POSTGRES_NAMESPACE/$POSTGRES_POD:$remote_path" "$POSTGRES_BACKUP_PATH"
    # Keep a raw copy and strip role definitions from the working copy
    cp "$POSTGRES_BACKUP_PATH" "${POSTGRES_BACKUP_PATH}.bak"
    sed -ni '1,/-- Roles/p;/-- User Configurations/,$p' "$POSTGRES_BACKUP_PATH"
    log_info "PostgreSQL database dump saved to $POSTGRES_BACKUP_PATH"
  else
    log_error "pg_dumpall failed!"
    exit 1
  fi

  # Re-enable md5 auth
  kubectl exec -n "$POSTGRES_NAMESPACE" "$POSTGRES_POD" -- /bin/bash -c '
    hba=$(find / -name pg_hba.conf -path "*/data/*" 2>/dev/null | head -1)
    if [[ -n "$hba" && -f "${hba}.bak" ]]; then
      cp "${hba}.bak" "$hba"
      pg_ctl reload -D "$(dirname "$hba")" 2>/dev/null || true
    fi
  '
}

################################
# Phase 4: PostgreSQL Service Passwords
################################

phase4_postgres_passwords() {
  log_info "=== Phase 4: Backing up PostgreSQL service passwords ==="

  local pw_file="${UPGRADE_BACKUP_DIR}/postgres-secrets-password.txt"
  if [[ -s "$pw_file" ]]; then
    log_info "postgres-secrets-password.txt already exists, skipping."
    return 0
  fi

  # Collect passwords from all known services
  local -A services=(
    [Alerting]="alerting-local-postgresql:orch-infra:PGPASSWORD"
    [CatalogService]="app-orch-catalog-local-postgresql:orch-app:PGPASSWORD"
    [Inventory]="inventory-local-postgresql:orch-infra:PGPASSWORD"
    [IAMTenancy]="iam-tenancy-local-postgresql:orch-iam:PGPASSWORD"
    [PlatformKeycloak]="platform-keycloak-local-postgresql:orch-platform:PGPASSWORD"
    [Vault]="vault-local-postgresql:orch-platform:PGPASSWORD"
    [PostgreSQL]="orch-database-postgresql:orch-database:password"
    [Mps]="mps-local-postgresql:orch-infra:PGPASSWORD"
    [Rps]="rps-local-postgresql:orch-infra:PGPASSWORD"
  )

  : > "$pw_file"
  local value
  for label in "${!services[@]}"; do
    IFS=':' read -r secret ns key <<< "${services[$label]}"
    value=$(kubectl get secret "$secret" -n "$ns" -o jsonpath="{.data.$key}" 2>/dev/null || true)
    echo "$label: $value" >> "$pw_file"
  done

  log_info "PostgreSQL service passwords saved."
}

################################
# Phase 5: MPS/RPS Connection Secrets
################################

phase5_mps_rps_secrets() {
  log_info "=== Phase 5: Backing up MPS/RPS secrets ==="

  for name in mps rps; do
    if kubectl get secret "$name" -n orch-infra >/dev/null 2>&1; then
      kubectl get secret "$name" -n orch-infra -o yaml > "${UPGRADE_BACKUP_DIR}/${name}_secret.yaml"
      log_info "$name secret backed up."
    else
      log_info "$name secret not found, skipping."
    fi
  done
}

################################
# Summary
################################

print_summary() {
  log_info "════════════════════════════════════════════════════════════"
  log_info "         Pre-Upgrade Backup Summary"
  log_info "════════════════════════════════════════════════════════════"
  log_info "Backup directory: $UPGRADE_BACKUP_DIR"
  log_info ""

  local count=0
  for f in "$UPGRADE_BACKUP_DIR"/*; do
    [[ -f "$f" ]] || continue
    log_info "  $(basename "$f")"
    count=$((count + 1))
  done

  log_info ""
  log_info "Total files: $count"
  log_info ""
  log_info "Next steps:"
  log_info "  1. cd pre-orch  && ./pre-orch.sh <provider> upgrade"
  log_info "  2. cd post-orch && ./post-orch-deploy.sh upgrade"
  log_info "════════════════════════════════════════════════════════════"
}

################################
# CLI
################################

usage() {
  cat >&2 <<EOF
Pre-upgrade backup for Edge Orchestrator (helmfile-based deployment).

Usage:
  $(basename "$0") [options]

Options:
  -h    Show this help

Backup artifacts (saved to helmfile-deploy/upgrade-backup/):
  postgres_secret.yaml            PostgreSQL superuser K8s secret
  ${POSTGRES_NAMESPACE}_backup.sql  Full PostgreSQL database dump
  postgres-secrets-password.txt   Base64-encoded service passwords
  mps_secret.yaml                 MPS connection secret
  rps_secret.yaml                 RPS connection secret

Execution order:
  1. ./pre-orch-backup.sh                          <-- this script
  2. cd pre-orch  && ./pre-orch.sh <provider> upgrade
  3. cd post-orch && ./post-orch-deploy.sh upgrade
EOF
}

################################
# Main
################################

main() {
  if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
    usage
    exit 0
  fi

  check_prerequisites
  phase1_postgres_health
  phase2_postgres_secret
  phase3_postgres_dump
  phase4_postgres_passwords
  phase5_mps_rps_secrets
  print_summary
}

main "$@"
