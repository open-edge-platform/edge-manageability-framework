#!/bin/bash

# SPDX-License-Identifier: Apache-2.0
#
# Helmfile deployment: install, uninstall, or list individual/all charts
# Uses helmfile with labels for individual chart targeting.
#
# Usage:
#   ./post-orch-deploy.sh install                  # Install all charts
#   ./post-orch-deploy.sh install traefik           # Install single chart
#   ./post-orch-deploy.sh uninstall traefik         # Uninstall single chart
#   ./post-orch-deploy.sh uninstall                 # Uninstall all charts
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

  # Required: cluster name and domain
  if [[ -z "${EMF_CLUSTER_NAME:-}" ]]; then
    echo "❌ EMF_CLUSTER_NAME is required"
    ((errors++))
  fi
  if [[ -z "${EMF_CLUSTER_DOMAIN:-}" ]]; then
    echo "❌ EMF_CLUSTER_DOMAIN is required"
    ((errors++))
  fi

  # Required: registry
  if [[ -z "${EMF_REGISTRY:-}" ]]; then
    echo "❌ EMF_REGISTRY is required"
    ((errors++))
  fi

  # Validate IPs for on-prem profiles (LoadBalancer)
  if [[ "${EMF_SERVICE_TYPE:-}" == "LoadBalancer" ]]; then
    if [[ -n "${EMF_TRAEFIK_IP:-}" ]]; then
      if ! is_valid_ip "$EMF_TRAEFIK_IP"; then
        echo "❌ Invalid Traefik IP: $EMF_TRAEFIK_IP"
        ((errors++))
      fi
    else
      echo "⚠️  EMF_TRAEFIK_IP not set (required for LoadBalancer service type)"
    fi

    if [[ -n "${EMF_HAPROXY_IP:-}" ]]; then
      if ! is_valid_ip "$EMF_HAPROXY_IP"; then
        echo "❌ Invalid HAProxy IP: $EMF_HAPROXY_IP"
        ((errors++))
      fi
    else
      echo "⚠️  EMF_HAPROXY_IP not set (required for LoadBalancer service type)"
    fi
  fi



  # SMTP validation (if email enabled)
  if [[ "${EMF_ENABLE_EMAIL:-false}" == "true" ]]; then
    if [[ -z "${EMF_SMTP_ADDRESS:-}" ]]; then
      echo "⚠️  EMF_ENABLE_EMAIL=true but EMF_SMTP_ADDRESS not set — SMTP secrets will be skipped"
    fi
  fi

  # SRE validation
  if [[ -n "${EMF_SRE_USERNAME:-}" && -z "${EMF_SRE_PASSWORD:-}" ]]; then
    echo "⚠️  EMF_SRE_USERNAME is set but EMF_SRE_PASSWORD is empty"
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
  installed_releases=$(helm list -A -a --no-headers 2>/dev/null | awk -F'\t' '{gsub(/^[ \t]+|[ \t]+$/, "", $1); gsub(/^[ \t]+|[ \t]+$/, "", $2); gsub(/^[ \t]+|[ \t]+$/, "", $5); print $1, $2, $5}')

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
  echo ""
  echo "═══════════════════════════════════════════════════════════════"
  echo "  DEPLOYMENT COMPLETE  (env: $HELMFILE_ENV)"
  echo "  Total time: $(( total_duration / 60 ))m $(( total_duration % 60 ))s"
  if (( sync_exit != 0 )); then
    echo "  ⚠️  helmfile sync exited with code $sync_exit"
  else
    echo "  ✅ All charts installed successfully"
  fi
  echo "═══════════════════════════════════════════════════════════════"
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
    return 0
  fi

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
    return 0
  fi

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
# MAIN
################################
if [[ -f "$MAIN_ENV_CONFIG" ]]; then
  set -a
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
    export "$arg"
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
