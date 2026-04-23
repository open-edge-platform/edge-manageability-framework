#!/bin/bash
# SPDX-FileCopyrightText: 2026 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0
# Helmfile deployment: install, uninstall, or list individual/all charts
# Uses helmfile with labels for individual chart targeting.
# Usage:
#   ./post-orch-deploy.sh install                  # Install all charts
#   ./post-orch-deploy.sh install traefik           # Install single chart
#   ./post-orch-deploy.sh uninstall traefik         # Uninstall single chart
#   ./post-orch-deploy.sh uninstall                 # Uninstall all charts
#   ./post-orch-deploy.sh upgrade                   # Upgrade all charts + restore DB
#   ./post-orch-deploy.sh list                      # List all charts
#   ./post-orch-deploy.sh diff                      # Preview changes
#   ./post-orch-deploy.sh diff traefik              # Preview single chart changes
set -o pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MAIN_ENV_CONFIG="$SCRIPT_DIR/post-orch.env"
################################
# VALIDATION
################################
VALID_PROFILES="onprem-eim onprem-vpro"
is_valid_ip() {
  local ip=$1
  if [[ $ip =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]; then
    IFS='.' read -r -a octets <<< "$ip"
    for octet in "${octets[@]}"; do
      if (( octet < 0 || octet > 255 )); then
        return 1
      fi
    done
    return 0
  fi
  return 1
}
validate_config() {
  local errors=0
  echo "🔍 Validating configuration..."
  # Validate profile
  local profile_valid=false
  for p in $VALID_PROFILES; do
    [[ "$HELMFILE_ENV" == "$p" ]] && profile_valid=true && break
  done
  if [[ "$profile_valid" != "true" ]]; then
    echo "❌ Invalid profile: $HELMFILE_ENV"
    echo "   Valid profiles: $VALID_PROFILES"
    ((errors++))
  fi
  # Required: cluster domain
  if [[ -z "${EMF_CLUSTER_DOMAIN:-}" ]]; then
    echo "❌ EMF_CLUSTER_DOMAIN is required"
    ((errors++))
  fi
  # Required: registry
  if [[ -z "${EMF_REGISTRY:-}" ]]; then
    echo "❌ EMF_REGISTRY is required"
    ((errors++))
  fi
  # Validate IPs
  # Single-IP mode: EMF_ORCH_IP overrides both Traefik and HAProxy IPs
  if [[ -n "${EMF_ORCH_IP:-}" ]]; then
    if ! is_valid_ip "$EMF_ORCH_IP"; then
      echo "❌ Invalid EMF_ORCH_IP: $EMF_ORCH_IP"
      ((errors++))
    else
      export EMF_TRAEFIK_IP="$EMF_ORCH_IP"
      export EMF_HAPROXY_IP="$EMF_ORCH_IP"
      echo "ℹ️  Single-IP mode: EMF_TRAEFIK_IP and EMF_HAPROXY_IP set to $EMF_ORCH_IP"
    fi
  else
    echo "ℹ️  Multi-IP mode: Traefik=${EMF_TRAEFIK_IP:-<not set>}, HAProxy=${EMF_HAPROXY_IP:-<not set>}"
  fi
  if [[ -n "${EMF_TRAEFIK_IP:-}" ]]; then
    if ! is_valid_ip "$EMF_TRAEFIK_IP"; then
      echo "❌ Invalid Traefik IP: $EMF_TRAEFIK_IP"
      ((errors++))
    fi
  fi
  if [[ -n "${EMF_HAPROXY_IP:-}" ]]; then
    if ! is_valid_ip "$EMF_HAPROXY_IP"; then
      echo "❌ Invalid HAProxy IP: $EMF_HAPROXY_IP"
      ((errors++))
    fi
  fi
  # Proxy: warn if http set but no_proxy missing
  if [[ -n "${EMF_HTTP_PROXY:-}" && -z "${EMF_NO_PROXY:-}" ]]; then
    echo "⚠️  EMF_HTTP_PROXY is set but EMF_NO_PROXY is empty — cluster services may be proxied"
  fi
  if (( errors > 0 )); then
    echo "❌ Validation failed with $errors error(s). Aborting."
    exit 1
  fi
  echo "✅ Configuration validated (profile: $HELMFILE_ENV)"
}
################################
# VALUES DUMP
################################
VALUES_OUTPUT_DIR="$SCRIPT_DIR/.computed-values/$HELMFILE_ENV"
dump_computed_values() {
  echo "📄 Dumping computed values to $VALUES_OUTPUT_DIR"
  rm -rf "$VALUES_OUTPUT_DIR"
  mkdir -p "$VALUES_OUTPUT_DIR"
  (cd "$SCRIPT_DIR" && helmfile -e "$HELMFILE_ENV" write-values \
    --output-file-template "$VALUES_OUTPUT_DIR/{{ .Release.Name }}.yaml" 2>&1) || {
    echo "⚠️  Failed to dump computed values (non-fatal, continuing)"
  }
  local count
  count=$(find "$VALUES_OUTPUT_DIR" -name '*.yaml' 2>/dev/null | wc -l)
  echo "✅ Dumped values for $count releases → $VALUES_OUTPUT_DIR"
  echo ""
}
################################
# HELMFILE WRAPPER
################################
helmfile_cmd() {
  local action="$1"
  shift
  (cd "$SCRIPT_DIR" && helmfile -e "$HELMFILE_ENV" "$@" "$action")
}
helmfile_sync_chart() {
  local chart="$1"
  echo "📦 Installing chart: $chart (env: $HELMFILE_ENV)"
  helmfile_cmd sync -l "app=$chart"
  echo "✅ Chart $chart installed"
}
helmfile_destroy_chart() {
  local chart="$1"
  echo "🗑️  Uninstalling chart: $chart (env: $HELMFILE_ENV)"
  helmfile_cmd destroy -l "app=$chart"
  echo "✅ Chart $chart uninstalled"
}
helmfile_sync_all() {
  echo "📦 Installing all charts (env: $HELMFILE_ENV)"
  echo "   Using helmfile — releases will deploy in parallel based on needs:"
  echo ""
  # Dump computed values before deploying
  dump_computed_values
  local start_time=$SECONDS
  # Pre-flight: clean up stale Jobs and fix broken Helm releases
  # so helmfile sync doesn't fail on immutable resources or stuck releases.
  echo "🔧 Pre-flight cleanup..."
  local installed_releases
  installed_releases=$(helm list -A -a --no-headers 2>/dev/null \
    | awk '{for(i=1;i<=NF;i++) if($i ~ /^[A-Z]{2,5}$/ && $(i-1) ~ /^[+-][0-9]{4}$/){print $1, $2, $(i+1); break}}')
  if [[ -n "$installed_releases" ]]; then
    while read -r release ns status; do
      [[ -z "$release" ]] && continue
      # Clean up immutable Jobs from previous runs
      local jobs
      jobs=$(helm get manifest "$release" -n "$ns" 2>/dev/null \
        | awk '/^kind: Job/{found=1; next} found && /^  name:/{gsub(/[" ]/, "", $2); print $2; found=0} /^---/{found=0}')
      for job in $jobs; do
        if kubectl get job "$job" -n "$ns" --no-headers 2>/dev/null | grep -q .; then
          echo "  🧹 Deleting stale Job $job in $ns"
          kubectl delete job "$job" -n "$ns" --ignore-not-found 2>/dev/null || true
        fi
      done
      # Also clean up Jobs matching release-[hex] pattern
      local job_list
      job_list=$(kubectl get jobs -A --no-headers 2>/dev/null \
        | awk -v r="$release" '$2 ~ "^"r"-[a-z0-9]+$" {print $1, $2}')
      if [[ -n "$job_list" ]]; then
        while IFS=' ' read -r job_ns job; do
          echo "  🧹 Deleting stale Job $job in $job_ns"
          kubectl delete job "$job" -n "$job_ns" --ignore-not-found 2>/dev/null || true
        done <<< "$job_list"
      fi
      # Fix releases stuck in "failed" or "pending-*" state
      if [[ "$status" != "deployed" && -n "$status" ]]; then
        echo "  🔧 Release $release is in '$status' state — attempting recovery..."
        local good_rev
        good_rev=$(helm history "$release" -n "$ns" --no-headers 2>/dev/null \
          | awk '{gsub(/^[ \t]+|[ \t]+$/, "", $0)} /deployed/ {rev=$1} END{if(rev) print rev}')
        if [[ -n "$good_rev" && "$good_rev" != "0" ]]; then
          echo "  🔧 Rolling back $release to revision $good_rev"
          helm rollback "$release" "$good_rev" -n "$ns" 2>&1 | sed 's/^/  /'
        elif [[ "$status" == pending-* ]]; then
          echo "  🔧 Release stuck in '$status' — uninstalling for fresh install"
          helm uninstall "$release" -n "$ns" 2>&1 | sed 's/^/  /'
        else
          local last_rev
          last_rev=$(helm history "$release" -n "$ns" --no-headers 2>/dev/null \
            | awk '{rev=$1} END{if(rev) print rev}')
          if [[ -n "$last_rev" ]]; then
            echo "  🔧 No deployed revision — rolling back to rev $last_rev to clear state"
            helm rollback "$release" "$last_rev" -n "$ns" 2>&1 | sed 's/^/  /'
          fi
        fi
      fi
    done <<< "$installed_releases"
  fi
  echo ""
  echo "🚀 Running helmfile sync (parallel deployment)..."
  echo ""
  local sync_exit=0
  (cd "$SCRIPT_DIR" && helmfile -e "$HELMFILE_ENV" --skip-deps sync --concurrency 4) 2>&1
  sync_exit=$?
  local total_duration=$(( SECONDS - start_time ))

  # ─── Build per-chart summary from helm list ────────────────────────────────
  local deployed_list="" failed_list="" pending_list="" notinstalled_list=""
  local deployed_count=0 failed_count=0 pending_count=0 notinstalled_count=0

  # Get all enabled releases from helmfile
  local enabled_releases
  enabled_releases=$(cd "$SCRIPT_DIR" && helmfile -e "$HELMFILE_ENV" list 2>/dev/null \
    | awk 'NR>1 && $3=="true" {print $1}' | sort)

  # Build lookup: release -> "status namespace"
  # Note: helm list UPDATED column has spaces (e.g. "2026-04-10 12:00:00 +0530 IST")
  # The timezone token (UTC/IST/PST/etc.) separates the timestamp from the status field.
  declare -A helm_status_map helm_ns_map
  while read -r name ns status; do
    [[ -z "$name" ]] && continue
    helm_status_map["$name"]="$status"
    helm_ns_map["$name"]="$ns"
  done < <(helm list -A -a --no-headers 2>/dev/null \
    | awk '{for(i=1;i<=NF;i++) if($i ~ /^[A-Z]{2,5}$/ && $(i-1) ~ /^[+-][0-9]{4}$/){print $1, $2, $(i+1); break}}')

  while read -r release; do
    [[ -z "$release" ]] && continue
    local st="${helm_status_map[$release]:-}"
    local ns="${helm_ns_map[$release]:-}"
    case "$st" in
      deployed)
        ((deployed_count++))
        deployed_list+="  ✅  $release ($ns)"$'\n'
        ;;
      failed)
        ((failed_count++))
        failed_list+="  ❌  $release ($ns)"$'\n'
        ;;
      pending-*)
        ((pending_count++))
        pending_list+="  ⏳  $release ($ns) [$st]"$'\n'
        ;;
      *)
        ((notinstalled_count++))
        notinstalled_list+="  ⚪  $release"$'\n'
        ;;
    esac
  done <<< "$enabled_releases"

  local total_enabled=$(( deployed_count + failed_count + pending_count + notinstalled_count ))

  echo ""
  echo "═══════════════════════════════════════════════════════════════"
  echo "  DEPLOYMENT SUMMARY  (env: $HELMFILE_ENV)"
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Total time: $(( total_duration / 60 ))m $(( total_duration % 60 ))s"
  echo ""
  printf "  ✅ Deployed: %-4d  ❌ Failed: %-4d  ⏳ Pending: %-4d  ⚪ Not installed: %d\n" \
    "$deployed_count" "$failed_count" "$pending_count" "$notinstalled_count"
  printf "  Total enabled releases: %d\n" "$total_enabled"
  echo ""

  if [[ -n "$failed_list" ]]; then
    echo "  FAILED:"
    echo -n "$failed_list"
    echo ""
  fi
  if [[ -n "$pending_list" ]]; then
    echo "  PENDING:"
    echo -n "$pending_list"
    echo ""
  fi
  if [[ -n "$notinstalled_list" ]]; then
    echo "  NOT INSTALLED:"
    echo -n "$notinstalled_list"
    echo ""
  fi
  if [[ -n "$deployed_list" ]]; then
    echo "  DEPLOYED:"
    echo -n "$deployed_list"
    echo ""
  fi

  if (( failed_count == 0 && pending_count == 0 && notinstalled_count == 0 )); then
    echo "  ✅ ALL $total_enabled RELEASES DEPLOYED SUCCESSFULLY"
  else
    if (( failed_count > 0 )); then
      echo "  ❌ $failed_count chart(s) failed"
    fi
    if (( pending_count > 0 )); then
      echo "  ⚠️  $pending_count chart(s) still pending"
    fi
    if (( notinstalled_count > 0 )); then
      echo "  ⚠️  $notinstalled_count chart(s) not installed"
    fi
    if (( sync_exit != 0 )); then
      echo "  ❌ helmfile sync exited with code $sync_exit"
    fi
    echo "  ❌ DEPLOYMENT FAILED"
  fi
  echo "═══════════════════════════════════════════════════════════════"

  if (( failed_count > 0 || pending_count > 0 || notinstalled_count > 0 || sync_exit != 0 )); then
    exit 1
  fi
}
helmfile_destroy_all() {
  echo "🗑️  Uninstalling all charts (env: $HELMFILE_ENV)"
  echo "   Destroying each release individually in reverse order — continues on failure"
  echo ""
  local passed=()
  local failed=()
  local skipped=()
  # Build lookup of installed releases: "name namespace"
  local installed_map
  installed_map=$(helm list -A --no-headers 2>/dev/null | awk '{print $1, $2}')
  if [[ -z "$installed_map" ]]; then
    echo "ℹ️  No helm releases found — nothing to uninstall"
  else
  # Get release names from helmfile.yaml.gotmpl in definition order, then reverse
  local all_helmfile_releases
  all_helmfile_releases=$(awk '/^releases:/{found=1} found && /^  - name: /{print $NF}' "$SCRIPT_DIR/helmfile.yaml.gotmpl")
  # Filter to only releases that are actually installed, in reverse wave order
  local releases
  releases=$(echo "$all_helmfile_releases" \
    | while read -r name; do echo "$installed_map" | awk -v r="$name" '$1==r{print r; exit}'; done \
    | tac)
  if [[ -z "$releases" ]]; then
    echo "ℹ️  No helmfile-managed releases are currently installed"
  else
  local total
  total=$(echo "$releases" | wc -l)
  local current=0
  for release in $releases; do
    ((current++))
    # Find the namespace for this release from the installed map
    local ns
    ns=$(echo "$installed_map" | awk -v r="$release" '$1==r{print $2; exit}')
    if [[ -z "$ns" ]]; then
      echo "[$current/$total] ⏭️  $release — not found, skipping"
      skipped+=("$release")
      continue
    fi
    echo "[$current/$total] 🗑️  Destroying: $release (ns: $ns)"
    if helm uninstall "$release" -n "$ns" 2>&1; then
      echo "✅ $release uninstalled"
      passed+=("$release")
    else
      echo "❌ $release uninstall FAILED"
      failed+=("$release")
    fi
  done
  echo ""
  echo "═══════════════════════════════════════════════════════════════"
  echo "  UNINSTALL SUMMARY  (env: $HELMFILE_ENV)"
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Removed: ${#passed[@]}  |  Failed: ${#failed[@]}  |  Skipped: ${#skipped[@]}  |  Total: $total"
  if (( ${#failed[@]} > 0 )); then
    echo "  Failed: $(printf '%s ' "${failed[@]}")"
  fi
  echo "═══════════════════════════════════════════════════════════════"
  echo ""
  fi # end releases loop
  fi # end installed_map check

  # Clean up orphaned secrets created by operators/controllers (not managed by Helm)
  echo "🧹 Cleaning up orphaned secrets (operator/controller-created)..."
  local orphan_secrets=(
    "orch-gateway:tls-orch"
    "orch-gateway:tls-traefik"
    "orch-platform:tls-rs-proxy"
    "orch-boots:tls-boots"
    "orch-boots:ingress-haproxy-kubernetes-ingress-default-cert"
    "orch-platform:vault-keys"
    "cert-manager:cert-manager-webhook-ca"
  )
  for entry in "${orphan_secrets[@]}"; do
    local ns="${entry%%:*}"
    local name="${entry##*:}"
    if kubectl get secret "$name" -n "$ns" --no-headers 2>/dev/null | grep -q .; then
      kubectl delete secret "$name" -n "$ns" --ignore-not-found 2>/dev/null || true
      echo "  🗑️  $ns/$name"
    fi
  done

  echo "✅ All charts uninstalled"
}
helmfile_diff_chart() {
  local chart="$1"
  echo "🔍 Diff for chart: $chart (env: $HELMFILE_ENV)"
  helmfile_cmd diff -l "app=$chart"
}
helmfile_diff_all() {
  echo "🔍 Diff for all charts (env: $HELMFILE_ENV)"
  helmfile_cmd diff
}
helmfile_list() {
  helmfile_cmd list
}
################################
# UPGRADE
################################
HELMFILE_DEPLOY_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
UPGRADE_BACKUP_DIR="$HELMFILE_DEPLOY_DIR/upgrade-backup"
POSTGRES_NAMESPACE="orch-database"
POSTGRES_USERNAME="postgres"

_upgrade_get_postgres_pod() {
  kubectl get pods -n "$POSTGRES_NAMESPACE" \
    -l cnpg.io/cluster=postgresql-cluster,cnpg.io/instanceRole=primary \
    -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "postgresql-cluster-1"
}

_upgrade_wait_postgres_ready() {
  echo "⏳ Waiting for PostgreSQL to be ready..."
  local deadline=$(( SECONDS + 300 ))
  while true; do
    local pod
    pod=$(_upgrade_get_postgres_pod)
    local phase
    phase=$(kubectl get pod "$pod" -n "$POSTGRES_NAMESPACE" -o jsonpath='{.status.phase}' 2>/dev/null || true)
    if [[ "$phase" == "Running" ]]; then
      # Check if the container is actually ready
      local ready
      ready=$(kubectl get pod "$pod" -n "$POSTGRES_NAMESPACE" -o jsonpath='{.status.containerStatuses[0].ready}' 2>/dev/null || true)
      if [[ "$ready" == "true" ]]; then
        echo "✅ PostgreSQL pod $pod is ready"
        return 0
      fi
    fi
    if (( SECONDS >= deadline )); then
      echo "❌ Timed out waiting for PostgreSQL to be ready"
      kubectl get pods -n "$POSTGRES_NAMESPACE" || true
      return 1
    fi
    sleep 5
  done
}

_upgrade_restore_postgres() {
  local backup_file="${UPGRADE_BACKUP_DIR}/${POSTGRES_NAMESPACE}_backup.sql"

  if [[ ! -f "$backup_file" ]]; then
    echo "⚠️  No PostgreSQL backup found at $backup_file — skipping restore"
    return 0
  fi

  echo "🔄 Restoring PostgreSQL from backup..."

  local pod
  pod=$(_upgrade_get_postgres_pod)
  local remote_path="/var/lib/postgresql/data/${POSTGRES_NAMESPACE}_backup.sql"

  # Copy backup to pod
  kubectl cp "$backup_file" "$POSTGRES_NAMESPACE/$pod:$remote_path" -c postgres

  # Get password from secret
  local pgpassword
  pgpassword=$(kubectl get secret -n "$POSTGRES_NAMESPACE" orch-database-postgresql \
    -o jsonpath='{.data.password}' 2>/dev/null | base64 -d 2>/dev/null)

  if [[ -z "$pgpassword" ]]; then
    echo "❌ Cannot retrieve PostgreSQL password from secret orch-database-postgresql"
    echo "   Ensure the secret exists in namespace $POSTGRES_NAMESPACE"
    return 1
  fi

  # Restore using credentials
  if kubectl exec -n "$POSTGRES_NAMESPACE" "$pod" -c postgres -- \
    env PGPASSWORD="$pgpassword" psql -U "$POSTGRES_USERNAME" -f "$remote_path" 2>&1; then
    echo "✅ PostgreSQL restore completed"
  else
    echo "❌ PostgreSQL restore failed — check logs above"
    return 1
  fi
}

_upgrade_restore_secrets() {
  echo "🔄 Restoring backed-up secrets..."

  # Restore PostgreSQL superuser secret
  if [[ -f "${UPGRADE_BACKUP_DIR}/postgres_secret.yaml" ]]; then
    kubectl apply -f "${UPGRADE_BACKUP_DIR}/postgres_secret.yaml" 2>/dev/null || true
    echo "  ✅ PostgreSQL superuser secret restored"
  fi

  # Restore MPS/RPS secrets
  for name in mps rps; do
    if [[ -f "${UPGRADE_BACKUP_DIR}/${name}_secret.yaml" ]]; then
      kubectl apply -f "${UPGRADE_BACKUP_DIR}/${name}_secret.yaml" 2>/dev/null || true
      echo "  ✅ $name secret restored"
    fi
  done
}

helmfile_upgrade_all() {
  echo "🔄 Starting upgrade (env: $HELMFILE_ENV)"
  echo ""

  # Step 1: Validate backup directory exists
  if [[ ! -d "$UPGRADE_BACKUP_DIR" ]]; then
    echo "❌ Backup directory not found: $UPGRADE_BACKUP_DIR"
    echo "   Run pre-orch-backup.sh first to create backups."
    exit 1
  fi
  echo "✅ Backup directory found: $UPGRADE_BACKUP_DIR"
  ls -la "$UPGRADE_BACKUP_DIR/"
  echo ""

  # Step 2: Restore secrets before helm sync (ensures existing passwords are preserved)
  _upgrade_restore_secrets
  echo ""

  # Step 3: Deploy/upgrade all charts via helmfile sync (same as install — idempotent)
  # Run in subshell so that helmfile_sync_all's exit 1 doesn't kill the upgrade flow.
  echo "🚀 Step 3: Running helmfile sync for upgrade..."
  local sync_failed=0
  ( helmfile_sync_all ) || sync_failed=$?

  if (( sync_failed != 0 )); then
    echo "⚠️  helmfile sync had failures (exit code: $sync_failed) — continuing to DB restore"
  fi
  echo ""

  # Step 3.5: Clean stale Keycloak JGroups JDBC_PING entries
  # During rolling updates, the old pod's cluster registration may remain in the
  # jgroups_ping table, causing the new pod's health check to report DOWN.
  echo "🧹 Step 3.5: Cleaning stale Keycloak JGroups cluster entries..."
  local kc_db="orch-platform-platform-keycloak"
  local pg_pod
  pg_pod=$(_upgrade_get_postgres_pod)
  if kubectl exec -n "$POSTGRES_NAMESPACE" "$pg_pod" -c postgres -- \
    psql -U "$POSTGRES_USERNAME" -d "$kc_db" -c "DELETE FROM jgroups_ping;" 2>/dev/null; then
    echo "  ✅ Stale JGroups entries purged — Keycloak will re-register on startup"
  else
    echo "  ⚠️  Could not clean JGroups entries (table may not exist yet) — skipping"
  fi
  echo ""

  # Step 4: Wait for PostgreSQL and restore data
  if [[ -f "${UPGRADE_BACKUP_DIR}/${POSTGRES_NAMESPACE}_backup.sql" ]]; then
    echo "🚀 Step 4: Restoring PostgreSQL data..."
    _upgrade_wait_postgres_ready
    _upgrade_restore_postgres
  else
    echo "⏭️  Step 4: No PostgreSQL backup found, skipping restore"
  fi

  echo ""
  echo "═══════════════════════════════════════════════════════════════"
  echo "  UPGRADE COMPLETE  (env: $HELMFILE_ENV)"
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Backup dir:  $UPGRADE_BACKUP_DIR"
  echo "  Charts:      $(if (( sync_failed == 0 )); then echo "All deployed successfully"; else echo "Deployed with $sync_failed error(s) — review above"; fi)"
  echo "  PostgreSQL:  $(if [[ -f "${UPGRADE_BACKUP_DIR}/${POSTGRES_NAMESPACE}_backup.sql" ]]; then echo "Restored from backup"; else echo "No restore needed"; fi)"
  echo "═══════════════════════════════════════════════════════════════"

  if (( sync_failed != 0 )); then
    exit 1
  fi
}
################################
# MAIN
################################
if [[ -f "$MAIN_ENV_CONFIG" ]]; then
  set -a
  # shellcheck source=/dev/null
  source "$MAIN_ENV_CONFIG"
  set +a
else
  echo "❌ Missing post-orch.env"
  exit 1
fi
# Support inline KEY=VALUE arguments (e.g., ./post-orch-deploy.sh EMF_HELMFILE_ENV=onprem-eim values)
args=()
for arg in "$@"; do
  if [[ "$arg" =~ ^[A-Z_]+=.+$ ]]; then
    declare -x "${arg?}"
  else
    args+=("$arg")
  fi
done
set -- "${args[@]}"
HELMFILE_ENV="${EMF_HELMFILE_ENV:-onprem-eim}"
# ─── Logging: tee all output to timestamped log file ────────────────────────
LOG_DIR="$SCRIPT_DIR/logs"
mkdir -p "$LOG_DIR"
LOG_TIMESTAMP="$(date +%Y%m%d-%H%M%S)"
LOG_FILE="$LOG_DIR/${HELMFILE_ENV}_${1:-unknown}_${LOG_TIMESTAMP}.log"
exec > >(tee -a "$LOG_FILE") 2>&1
echo "═══ Log started: $(date -Iseconds) ═══"
echo "═══ Command: $0 $* ═══"
echo "═══ Environment: $HELMFILE_ENV ═══"
echo ""
SCRIPT_START_TIME=$SECONDS
SCRIPT_START_TS=$(date '+%Y-%m-%d %H:%M:%S')
validate_config
usage() {
  cat <<EOF
Usage: $0 <action> [chart-name]
Actions:
  install              Install all charts
  install <chart>      Install a single chart (e.g., traefik, vault, harbor)
  uninstall            Uninstall all charts (helmfile destroy)
  uninstall <chart>    Uninstall a single chart
  upgrade              Upgrade all charts + restore PostgreSQL from backup
  diff                 Preview changes for all charts
  diff <chart>         Preview changes for a single chart
  values               Dump computed values for all releases
  values <chart>       Dump computed values for a single release
  list                 List all available charts and their status
Environment:
  EMF_HELMFILE_ENV     Helmfile environment (default: onprem-eim)
                       Valid profiles: onprem-eim, onprem-vpro
Examples:
  $0 install                             # Install all charts (eim/vpro)
  $0 install traefik                     # Install only traefik
  $0 uninstall traefik                   # Uninstall only traefik
  $0 upgrade                             # Upgrade + restore DB from backup
  $0 diff vault                          # Preview vault changes
  $0 list                                # List all charts
EOF
}
ACTION="${1:-}"
CHART_NAME="${2:-}"
case "$ACTION" in
  install)
    if [[ -n "$CHART_NAME" ]]; then
      helmfile_sync_chart "$CHART_NAME"
    else
      helmfile_sync_all
    fi
    ;;
  uninstall)
    if [[ -n "$CHART_NAME" ]]; then
      helmfile_destroy_chart "$CHART_NAME"
    else
      helmfile_destroy_all
    fi
    ;;
  upgrade)
    helmfile_upgrade_all
    ;;
  diff)
    if [[ -n "$CHART_NAME" ]]; then
      helmfile_diff_chart "$CHART_NAME"
    else
      helmfile_diff_all
    fi
    ;;
  values)
    if [[ -n "$CHART_NAME" ]]; then
      echo "📄 Dumping computed values for: $CHART_NAME"
      mkdir -p "$VALUES_OUTPUT_DIR"
      (cd "$SCRIPT_DIR" && helmfile -e "$HELMFILE_ENV" -l "app=$CHART_NAME" write-values \
        --output-file-template "$VALUES_OUTPUT_DIR/{{ .Release.Name }}.yaml" 2>&1)
      echo "✅ Values written to $VALUES_OUTPUT_DIR/$CHART_NAME.yaml"
    else
      dump_computed_values
    fi
    ;;
  list)
    helmfile_list
    ;;
  *)
    usage
    exit 1
    ;;
esac
# ─── Script timing summary ──────────────────────────────────────────────────
SCRIPT_END_TS=$(date '+%Y-%m-%d %H:%M:%S')
SCRIPT_TOTAL=$(( SECONDS - SCRIPT_START_TIME ))
echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "  SCRIPT TIMING"
echo "  Start: $SCRIPT_START_TS"
echo "  End:   $SCRIPT_END_TS"
echo "  Total: $(( SCRIPT_TOTAL / 60 ))m $(( SCRIPT_TOTAL % 60 ))s"
echo "═══════════════════════════════════════════════════════════════"

