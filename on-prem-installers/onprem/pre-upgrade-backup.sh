#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2026 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Script Name: pre-upgrade-backup.sh
# Description: Standalone backup script for Edge Orchestrator pre-upgrade.
#              Creates backups of all critical data before running the upgrade:
#               - PostgreSQL database dump (pg_dumpall)
#               - PostgreSQL superuser secret
#               - PostgreSQL service passwords (9 services)
#               - MPS/RPS connection secrets
#               - Gitea secrets cleanup (pre-backup)
#               - PersistentVolume LVM snapshots (optional, -b flag)
#               - RKE2 etcd snapshot (optional, -b flag, RKE2 only)
#               - K3s etcd snapshot (optional, -b flag, K3s only)
#
# This script should be run BEFORE pre-orch-upgrade.sh and post-orch-upgrade.sh.
#
# Usage:
#   ./pre-upgrade-backup.sh [options]
#
# Options:
#   -b    Enable PV and etcd backup (requires LVM + sufficient VG space)
#   -s    Skip interactive prompts (non-interactive mode)
#   -h    Show help

set -euo pipefail

export PATH="/usr/local/bin:${PATH}"
export KUBECONFIG="${KUBECONFIG:-/home/$USER/.kube/config}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# shellcheck disable=SC1091
source "${SCRIPT_DIR}/onprem.env"

# shellcheck disable=SC1091
# Provides: check_postgres, backup_postgres, restore_postgres, local_backup_path, etc.
source "${SCRIPT_DIR}/upgrade_postgres.sh"

################################
# Logging
################################

LOG_FILE="pre_upgrade_backup_$(date +'%Y%m%d_%H%M%S').log"
LOG_DIR="/var/log/orch-upgrade"

sudo mkdir -p "$LOG_DIR"
sudo chown "$(whoami):$(whoami)" "$LOG_DIR"

FULL_LOG_PATH="$LOG_DIR/$LOG_FILE"

log_message() {
  echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*" | tee -a "$FULL_LOG_PATH"
}

log_info() {
  log_message "INFO: $*"
}

log_warn() {
  log_message "WARN: $*"
}

log_error() {
  log_message "ERROR: $*"
}

# Redirect all output to both console and log file
exec > >(tee -a "$FULL_LOG_PATH")
exec 2> >(tee -a "$FULL_LOG_PATH" >&2)

log_info "Starting pre-upgrade backup script"
log_info "Log file: $FULL_LOG_PATH"

################################
# Configuration
################################

BACKUP_PVS="${BACKUP_PVS:-false}"
SKIP_INTERACTIVE="${SKIP_INTERACTIVE:-false}"
BACKUP_DIR="$(pwd)"

# From upgrade_postgres.sh (sourced above):
#   postgres_namespace=orch-database
#   local_backup_path=./orch-database_backup.sql
#   podname=postgresql-cluster-1
#   POSTGRES_USERNAME=postgres
#   application_namespace=onprem

################################
# Kubernetes Provider Detection
################################

detect_k8s_provider() {
  if systemctl is-active --quiet rke2-server.service 2>/dev/null; then
    echo "rke2"
  elif systemctl is-active --quiet k3s.service 2>/dev/null; then
    echo "k3s"
  elif command -v kind &>/dev/null && kind get clusters 2>/dev/null | grep -q .; then
    echo "kind"
  elif [[ -d /etc/rancher/rke2 ]]; then
    echo "rke2"
  elif [[ -d /etc/rancher/k3s ]]; then
    echo "k3s"
  else
    echo "unknown"
  fi
}

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

  log_info "Prerequisites met."
}

################################
# Phase 1: PostgreSQL Health Check
################################

pre_backup_postgres_check() {
  log_info "=== Phase 1: PostgreSQL health check ==="

  # Do NOT call check_postgres() from upgrade_postgres.sh here — it has an
  # interactive read -rp prompt that blocks in non-interactive / piped runs.
  # Instead, perform the health check inline.

  if [[ -f "$local_backup_path" ]]; then
    log_warn "Existing backup file found: $local_backup_path"
    if [[ "$SKIP_INTERACTIVE" == "true" ]]; then
      log_info "Non-interactive mode (-s): continuing with existing backup file."
    else
      read -rp "A backup file already exists. Type 'Continue' to proceed or Ctrl-C to abort: " confirm
      if [[ ! "$confirm" =~ ^[Cc][Oo][Nn][Tt][Ii][Nn][Uu][Ee]$ ]]; then
        log_error "User aborted."
        exit 1
      fi
    fi
    log_info "PostgreSQL health check skipped (recovery from previous run)."
    return 0
  fi

  # shellcheck disable=SC2154  # podname, postgres_namespace from upgrade_postgres.sh
  local pod_status
  pod_status=$(kubectl get pods -n "$postgres_namespace" "$podname" \
    -o jsonpath='{.status.phase}' 2>/dev/null || true)

  if [[ "$pod_status" != "Running" ]]; then
    log_error "PostgreSQL pod $podname is not running (status: ${pod_status:-not found})."
    exit 1
  fi

  log_info "PostgreSQL pod $podname is healthy (Running)."
}

################################
# Phase 2: PostgreSQL Superuser Secret Backup
################################

backup_postgres_secret() {
  log_info "=== Phase 2: Backing up PostgreSQL superuser secret ==="

  if [[ -f "${BACKUP_DIR}/postgres_secret.yaml" ]]; then
    log_info "postgres_secret.yaml already exists, skipping."
    return 0
  fi

  if kubectl get secret -n orch-database postgresql-cluster-superuser >/dev/null 2>&1; then
    kubectl get secret -n orch-database postgresql-cluster-superuser \
      -o yaml > "${BACKUP_DIR}/postgres_secret.yaml"
    log_info "PostgreSQL superuser secret saved to postgres_secret.yaml"
  else
    log_warn "postgresql-cluster-superuser secret not found, skipping."
  fi
}

################################
# Phase 3: PostgreSQL Database Dump
################################

backup_postgres_database() {
  log_info "=== Phase 3: Backing up PostgreSQL databases ==="

  # backup_postgres() from upgrade_postgres.sh handles idempotency
  backup_postgres

  # shellcheck disable=SC2154  # local_backup_path defined in upgrade_postgres.sh
  if [[ -f "$local_backup_path" ]]; then
    log_info "PostgreSQL database backup saved to $local_backup_path"
  else
    log_error "PostgreSQL database backup failed!"
    exit 1
  fi
}

################################
# Phase 4: Gitea Secrets Cleanup
################################

cleanup_gitea_secrets() {
  log_info "=== Phase 4: Cleaning up Gitea secrets before backup ==="

  local install_gitea="${INSTALL_GITEA:-true}"

  if [[ "$install_gitea" != "true" ]]; then
    log_info "Gitea not installed, skipping secrets cleanup."
    return 0
  fi

  local secrets=("gitea-apporch-token" "gitea-argocd-token" "gitea-clusterorch-token")
  for secret in "${secrets[@]}"; do
    if kubectl get secret "$secret" -n gitea >/dev/null 2>&1; then
      kubectl delete secret "$secret" -n gitea
      log_info "Deleted Gitea secret: $secret"
    fi
  done

  log_info "Gitea secrets cleanup completed."
}

################################
# Phase 5: PostgreSQL Service Passwords
################################

backup_postgres_passwords() {
  log_info "=== Phase 5: Backing up PostgreSQL service passwords ==="

  if [[ -s "${BACKUP_DIR}/postgres-secrets-password.txt" ]]; then
    log_info "postgres-secrets-password.txt already exists, skipping."
    return 0
  fi

  local alerting catalog inventory iam_tenancy platform_keycloak vault_pw postgresql mps rps

  alerting=$(kubectl get secret alerting-local-postgresql -n orch-infra \
    -o jsonpath='{.data.PGPASSWORD}' 2>/dev/null || true)
  catalog=$(kubectl get secret app-orch-catalog-local-postgresql -n orch-app \
    -o jsonpath='{.data.PGPASSWORD}' 2>/dev/null || true)
  inventory=$(kubectl get secret inventory-local-postgresql -n orch-infra \
    -o jsonpath='{.data.PGPASSWORD}' 2>/dev/null || true)
  iam_tenancy=$(kubectl get secret iam-tenancy-local-postgresql -n orch-iam \
    -o jsonpath='{.data.PGPASSWORD}' 2>/dev/null || true)
  platform_keycloak=$(kubectl get secret platform-keycloak-local-postgresql -n orch-platform \
    -o jsonpath='{.data.PGPASSWORD}' 2>/dev/null || true)
  vault_pw=$(kubectl get secret vault-local-postgresql -n orch-platform \
    -o jsonpath='{.data.PGPASSWORD}' 2>/dev/null || true)
  postgresql=$(kubectl get secret orch-database-postgresql -n orch-database \
    -o jsonpath='{.data.password}' 2>/dev/null || true)
  mps=$(kubectl get secret mps-local-postgresql -n orch-infra \
    -o jsonpath='{.data.PGPASSWORD}' 2>/dev/null || true)
  rps=$(kubectl get secret rps-local-postgresql -n orch-infra \
    -o jsonpath='{.data.PGPASSWORD}' 2>/dev/null || true)

  {
    echo "Alerting: $alerting"
    echo "CatalogService: $catalog"
    echo "Inventory: $inventory"
    echo "IAMTenancy: $iam_tenancy"
    echo "PlatformKeycloak: $platform_keycloak"
    echo "Vault: $vault_pw"
    echo "PostgreSQL: $postgresql"
    echo "Mps: $mps"
    echo "Rps: $rps"
  } > "${BACKUP_DIR}/postgres-secrets-password.txt"

  log_info "PostgreSQL service passwords saved to postgres-secrets-password.txt"
}

################################
# Phase 6: MPS/RPS Secret Backup
################################

backup_mps_rps_secrets() {
  log_info "=== Phase 6: Backing up MPS/RPS secrets ==="

  if kubectl get secret mps -n orch-infra >/dev/null 2>&1; then
    kubectl get secret mps -n orch-infra -o yaml > "${BACKUP_DIR}/mps_secret.yaml"
    log_info "MPS secret backed up to mps_secret.yaml"
  else
    log_info "MPS secret not found, skipping."
  fi

  if kubectl get secret rps -n orch-infra >/dev/null 2>&1; then
    kubectl get secret rps -n orch-infra -o yaml > "${BACKUP_DIR}/rps_secret.yaml"
    log_info "RPS secret backed up to rps_secret.yaml"
  else
    log_info "RPS secret not found, skipping."
  fi
}

################################
# Phase 7: PV LVM Backup (optional)
################################

# Get LV size and format it for lvcreate command
get_lv_size() {
  local lv_path="$1"

  local size_output size unit formatted_size
  size_output=$(sudo lvdisplay "$lv_path" | grep "LV Size" | awk '{print $3, $4}')
  size=$(echo "$size_output" | awk '{print $1}')
  unit=$(echo "$size_output" | awk '{print $2}')

  case $unit in
    GiB)  formatted_size="${size}G" ;;
    MiB)  formatted_size="${size}M" ;;
    TiB)  formatted_size="${size}T" ;;
    *)
      log_error "Unsupported LV size unit: $unit"
      exit 1
      ;;
  esac

  echo "$formatted_size"
}

check_space_for_backup() {
  local vg_info vsize vfree vused margin enough_space

  vg_info=$(sudo vgs --noheadings --units g --nosuffix -o vg_size,vg_free 2>/dev/null)
  vsize=$(echo "$vg_info" | awk '{print $1}')
  vfree=$(echo "$vg_info" | awk '{print $2}')
  vused=$(echo "$vsize - $vfree" | bc)

  margin=$(echo "$vused * 0.05" | bc)
  enough_space=$(echo "$vfree > ($vused + $margin)" | bc)

  echo "$enough_space"
}

backup_pvs() {
  log_info "=== Phase 7a: Backing up PersistentVolumes (LVM snapshots) ==="

  local space_check_result
  space_check_result=$(check_space_for_backup)
  if [[ "$space_check_result" -eq 0 ]]; then
    log_error "Not enough space for PVs backup in VG"
    return 1
  fi

  local vg_name="lvmvg"
  local vol_snap_class_name="openebs-lvm-vsc"
  local backup_date
  backup_date=$(date +'%Y-%m-%d-%H_%M')

  local pvs_to_backup
  pvs_to_backup=$(kubectl get pvc --all-namespaces \
    -o jsonpath='{range .items[?(@.status.phase=="Bound")]}{.metadata.name}{" "}{.metadata.namespace}{" "}{.spec.volumeName}{"\n"}{end}')

  echo "$pvs_to_backup" | while IFS= read -r line; do
    local pvc_name pvc_namespace lv_name
    read -r pvc_name pvc_namespace lv_name <<<"$line"

    # dkam-pvc data gets re-populated, no need to backup
    if [[ "$pvc_name" == "dkam-pvc" ]]; then
      continue
    fi

    log_info "Backing up PVC: $pvc_name (namespace: $pvc_namespace)"

    # Create VolumeSnapshot
    kubectl apply -f - <<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: ${pvc_name}-snap
  namespace: ${pvc_namespace}
spec:
  volumeSnapshotClassName: ${vol_snap_class_name}
  source:
    persistentVolumeClaimName: ${pvc_name}
EOF

    # Wait for snapshot readiness
    local attempts=0 max_attempts=40
    while [[ "$(kubectl get volumesnapshot -n "$pvc_namespace" "${pvc_name}-snap" \
      -o jsonpath='{.status.readyToUse}')" != "true" ]]; do
      log_info "Waiting for VolumeSnapshot ${pvc_name}-snap..."
      sleep 5
      attempts=$((attempts + 1))
      if (( attempts >= max_attempts )); then
        log_error "Timeout waiting for VolumeSnapshot ${pvc_name}-snap"
        exit 1
      fi
    done

    # Create backup LV on VG
    local formatted_size bak_lv_name
    formatted_size=$(get_lv_size "/dev/$vg_name/$lv_name")
    bak_lv_name="${pvc_name}-${pvc_namespace}-bak-${backup_date}"

    sudo lvcreate -n "$bak_lv_name" -L "$formatted_size" "$vg_name" -y
    sudo mkfs.ext4 "/dev/$vg_name/$bak_lv_name"

    # Copy data from snapshot to backup LV
    sudo mkdir -p /mnt/original-lv /mnt/backup-lv

    local snap_name
    snap_name=$(sudo lvs --options lv_name,origin --noheadings \
      | grep "$lv_name" \
      | awk -v lv_name="$lv_name" '$1 != lv_name {print $1}')

    sudo mount "/dev/$vg_name/$snap_name" /mnt/original-lv
    sudo mount "/dev/$vg_name/$bak_lv_name" /mnt/backup-lv
    sudo cp -a /mnt/original-lv/. /mnt/backup-lv/
    sudo umount "/dev/$vg_name/$snap_name"
    sudo umount "/dev/$vg_name/$bak_lv_name"

    sudo rm -rf /mnt/original-lv /mnt/backup-lv

    # Cleanup snapshot
    kubectl delete volumesnapshot -n "$pvc_namespace" "${pvc_name}-snap"

    log_info "PVC $pvc_name backed up to LV $bak_lv_name"
  done

  log_info "PV backup completed."
}

################################
# Phase 8: etcd Snapshot (optional)
################################

backup_etcd() {
  log_info "=== Phase 7b: Taking etcd snapshot ==="

  local provider
  provider=$(detect_k8s_provider)

  case "$provider" in
    rke2)
      local snapshot_name
      # Use DEPLOY_VERSION or current RKE2 version instead of dpkg-query
      local rke2_version
      rke2_version=$(rke2 --version 2>/dev/null | head -1 | awk '{print $3}' || echo "unknown")
      snapshot_name="pre-upgrade-snapshot-${rke2_version}-$(date +'%Y%m%d_%H%M%S')"

      log_info "Taking RKE2 etcd snapshot: $snapshot_name"
      sudo rke2 etcd-snapshot save --name "$snapshot_name"

      sudo mkdir -p /var/orch-backups/
      sudo find /var/lib/rancher/rke2/server/db/snapshots/ \
        -name "pre-upgrade-snapshot-*" \
        -exec mv {} /var/orch-backups/ \;

      log_info "RKE2 etcd snapshot saved to /var/orch-backups/"
      ;;

    k3s)
      local snapshot_name
      local k3s_version
      k3s_version=$(k3s --version 2>/dev/null | head -1 | awk '{print $3}' || echo "unknown")
      snapshot_name="pre-upgrade-snapshot-${k3s_version}-$(date +'%Y%m%d_%H%M%S')"

      log_info "Taking K3s etcd snapshot: $snapshot_name"
      sudo k3s etcd-snapshot save --name "$snapshot_name"

      sudo mkdir -p /var/orch-backups/
      sudo find /var/lib/rancher/k3s/server/db/snapshots/ \
        -name "pre-upgrade-snapshot-*" \
        -exec mv {} /var/orch-backups/ \;

      log_info "K3s etcd snapshot saved to /var/orch-backups/"
      ;;

    kind)
      log_info "KIND clusters are ephemeral — etcd snapshots not applicable."
      ;;

    *)
      log_warn "Unknown K8s provider '$provider' — skipping etcd snapshot."
      ;;
  esac
}

################################
# Summary
################################

print_summary() {
  log_info "================================================"
  log_info "         Pre-Upgrade Backup Summary"
  log_info "================================================"

  local files=()
  [[ -f "${BACKUP_DIR}/postgres_secret.yaml" ]] && \
    files+=("  postgres_secret.yaml (PostgreSQL superuser secret)")
  [[ -f "$local_backup_path" ]] && \
    files+=("  ${local_backup_path} (PostgreSQL database dump)")
  [[ -f "${BACKUP_DIR}/postgres-secrets-password.txt" ]] && \
    files+=("  postgres-secrets-password.txt (9 service passwords)")
  [[ -f "${BACKUP_DIR}/mps_secret.yaml" ]] && \
    files+=("  mps_secret.yaml (MPS connection secret)")
  [[ -f "${BACKUP_DIR}/rps_secret.yaml" ]] && \
    files+=("  rps_secret.yaml (RPS connection secret)")

  if [[ ${#files[@]} -gt 0 ]]; then
    log_info "Backup files created:"
    for f in "${files[@]}"; do
      log_info "$f"
    done
  fi

  if [[ "$BACKUP_PVS" == "true" ]]; then
    log_info "PV LVM snapshots: created in VG lvmvg"
    log_info "etcd snapshots: saved to /var/orch-backups/"
  fi

  log_info "================================================"
  log_info "Backups complete. You can now run:"
  log_info "  1. ./pre-orch-upgrade.sh   (K8s + OS upgrade)"
  log_info "  2. ./post-orch-upgrade.sh  (Gitea, ArgoCD, orchestrator)"
  log_info "================================================"
}

################################
# CLI
################################

usage() {
  cat >&2 <<EOF
Purpose:
  Pre-upgrade backup for OnPrem Edge Orchestrator.
  Creates backups of all critical data before the upgrade.

Usage:
  $(basename "$0") [options]

Options:
  -b    Enable PV (LVM) and etcd backup (requires LVM VG space)
  -s    Skip interactive prompts (non-interactive mode)
  -h    Show this help message

Backup artifacts created (in current directory):
  postgres_secret.yaml            PostgreSQL superuser K8s secret
  orch-database_backup.sql        Full PostgreSQL database dump
  postgres-secrets-password.txt   Base64-encoded passwords for 9 services
  mps_secret.yaml                 MPS connection secret
  rps_secret.yaml                 RPS connection secret

With -b flag (optional):
  LVM backup LVs                  One per bound PVC (in lvmvg)
  /var/orch-backups/              etcd snapshots (RKE2/K3s)

Execution order:
  1. ./pre-upgrade-backup.sh      <-- this script
  2. ./pre-orch-upgrade.sh        K8s cluster + OS upgrade
  3. ./post-orch-upgrade.sh       Gitea, ArgoCD, orchestrator upgrade
  4. ./after_upgrade_restart.sh   ArgoCD app sync
EOF
}

################################
# Main
################################

main() {
  local help_flag="" backup_flag="" skip_flag=""

  while getopts 'hbs' flag; do
    case "${flag}" in
      h) help_flag="true" ;;
      b) backup_flag="true" ;;
      s) skip_flag="true" ;;
      *) help_flag="true" ;;
    esac
  done

  if [[ "${help_flag:-}" == "true" ]]; then
    usage
    exit 0
  fi

  if [[ "${backup_flag:-}" == "true" ]]; then
    BACKUP_PVS="true"
  fi

  if [[ "${skip_flag:-}" == "true" ]]; then
    SKIP_INTERACTIVE="true"
  fi

  check_prerequisites

  # Phase 1: PostgreSQL health check
  pre_backup_postgres_check

  # Phase 2: PostgreSQL superuser secret
  backup_postgres_secret

  # Phase 3: PostgreSQL database dump
  backup_postgres_database

  # Phase 4: Gitea secrets cleanup
  cleanup_gitea_secrets

  # Phase 5: PostgreSQL service passwords
  backup_postgres_passwords

  # Phase 6: MPS/RPS secrets
  backup_mps_rps_secrets

  # Phase 7-8: PV + etcd backups (optional)
  if [[ "$BACKUP_PVS" == "true" ]]; then
    backup_pvs
    backup_etcd
  fi

  # Summary
  print_summary
}

main "$@"
