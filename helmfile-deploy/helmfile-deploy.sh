#!/bin/bash

# SPDX-License-Identifier: Apache-2.0
#
# Helmfile deployment: install, uninstall, or list individual/all charts
# Uses helmfile with labels for individual chart targeting.
#
# Usage:
#   ./helmfile-deploy.sh install                  # Install all charts
#   ./helmfile-deploy.sh install traefik           # Install single chart
#   ./helmfile-deploy.sh uninstall traefik         # Uninstall single chart
#   ./helmfile-deploy.sh uninstall                 # Uninstall all charts
#   ./helmfile-deploy.sh list                      # List all charts
#   ./helmfile-deploy.sh diff                      # Preview changes
#   ./helmfile-deploy.sh diff traefik              # Preview single chart changes

set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MAIN_ENV_CONFIG="$SCRIPT_DIR/onprem.env"

################################
# VALIDATION
################################
VALID_PROFILES="onprem onprem-1k onprem-oxm onprem-explicit-proxy aws onprem-vpro onprem-eim onprem-eim-co onprem-eim-co-ao onprem-eim-co-ao-o11y dev dev-minimal bkc"

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

  # OXM profile requires PXE variables
  if [[ "$HELMFILE_ENV" == "onprem-oxm" ]]; then
    if [[ -z "${EMF_OXM_PXE_SERVER_INT:-}" || -z "${EMF_OXM_PXE_SERVER_IP:-}" || -z "${EMF_OXM_PXE_SERVER_SUBNET:-}" ]]; then
      echo "❌ OXM profile requires: EMF_OXM_PXE_SERVER_INT, EMF_OXM_PXE_SERVER_IP, EMF_OXM_PXE_SERVER_SUBNET"
      ((errors++))
    fi
  fi

  # SMTP validation (if email enabled)
  if [[ "${EMF_ENABLE_EMAIL:-true}" == "true" ]]; then
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
  echo "   Deploying each release individually — continues on failure"
  echo ""

  local passed=()
  local failed=()
  local start_time=$SECONDS

  # Get enabled releases, then order them by helmfile.yaml wave order
  local enabled_list
  enabled_list=$(cd "$SCRIPT_DIR" && helmfile -e "$HELMFILE_ENV" list 2>/dev/null \
    | awk 'NR>1 && $3=="true" {print $1}')

  if [[ -z "$enabled_list" ]]; then
    echo "❌ No enabled releases found for environment: $HELMFILE_ENV"
    exit 1
  fi

  # Extract release names from helmfile.yaml.gotmpl in wave order, filter to enabled only
  local releases
  releases=$(awk '/^releases:/{found=1} found && /^  - name: /{print $NF}' "$SCRIPT_DIR/helmfile.yaml.gotmpl" \
    | while read -r name; do echo "$enabled_list" | grep -qx "$name" && echo "$name"; done)

  local total
  total=$(echo "$releases" | wc -l)
  local current=0

  # Add repos once before the loop to avoid re-adding on every release
  echo "📡 Adding helm repositories..."
  (cd "$SCRIPT_DIR" && helmfile -e "$HELMFILE_ENV" repos 2>&1) || true
  echo ""

  for release in $releases; do
    ((current++))
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "[$current/$total] 📦 Deploying: $release"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

    local release_start=$SECONDS
    if (cd "$SCRIPT_DIR" && helmfile -e "$HELMFILE_ENV" -l "app=$release" --skip-deps sync 2>&1); then
      local duration=$(( SECONDS - release_start ))
      echo "✅ $release (${duration}s)"
      passed+=("$release|${duration}")
    else
      local duration=$(( SECONDS - release_start ))
      echo "❌ $release FAILED (${duration}s)"
      failed+=("$release|${duration}")
    fi

    # ─── Live Progress ───
    local elapsed=$(( SECONDS - start_time ))
    echo ""
    echo "  ┌─ Progress: $current/$total  |  ✅ ${#passed[@]}  ❌ ${#failed[@]}  |  Elapsed: $(( elapsed / 60 ))m $(( elapsed % 60 ))s"
    if (( ${#failed[@]} > 0 )); then
      echo "  │  Failed so far: $(printf '%s ' "${failed[@]}" | sed 's/|[0-9]*s*//g; s/|[0-9]*//g')"
    fi
    echo "  └──────────────────────────────────────────────────────────"
    echo ""
  done

  local total_duration=$(( SECONDS - start_time ))

  # ─── Summary ─────────────────────────────────────────────────────────────────
  echo ""
  echo "═══════════════════════════════════════════════════════════════"
  echo "  DEPLOYMENT SUMMARY  (env: $HELMFILE_ENV)"
  echo "  Total time: $(( total_duration / 60 ))m $(( total_duration % 60 ))s"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""

  if (( ${#passed[@]} > 0 )); then
    echo "✅ PASSED (${#passed[@]}):"
    printf "   %-40s %s\n" "RELEASE" "DURATION"
    for r in "${passed[@]}"; do
      local name="${r%%|*}"
      local dur="${r##*|}"
      printf "   ✓ %-38s %ss\n" "$name" "$dur"
    done
    echo ""
  fi

  if (( ${#failed[@]} > 0 )); then
    echo "❌ FAILED (${#failed[@]}):"
    printf "   %-40s %s\n" "RELEASE" "DURATION"
    for r in "${failed[@]}"; do
      local name="${r%%|*}"
      local dur="${r##*|}"
      printf "   ✗ %-38s %ss\n" "$name" "$dur"
    done
    echo ""
  fi

  echo "───────────────────────────────────────────────────────────────"
  echo "  Passed: ${#passed[@]}  |  Failed: ${#failed[@]}  |  Total: $total"
  echo "───────────────────────────────────────────────────────────────"

  if (( ${#failed[@]} > 0 )); then
    echo ""
    echo "⚠️  To retry failed releases:"
    for r in "${failed[@]}"; do
      local name="${r%%|*}"
      echo "   helmfile -e $HELMFILE_ENV -l app=$name sync"
    done
    return 1
  fi

  echo ""
  echo "✅ All $total charts installed successfully"
}

helmfile_destroy_all() {
  echo "🗑️  Uninstalling all charts (env: $HELMFILE_ENV)"
  helmfile_cmd destroy
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
  echo "❌ Missing onprem.env"
  exit 1
fi

HELMFILE_ENV="${EMF_HELMFILE_ENV:-onprem}"

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
  list                 List all available charts and their status

Environment:
  EMF_HELMFILE_ENV     Helmfile environment (default: onprem)

Examples:
  $0 install                             # Install all charts
  $0 install traefik                     # Install only traefik
  $0 uninstall traefik                   # Uninstall only traefik
  $0 diff vault                          # Preview vault changes
  EMF_HELMFILE_ENV=onprem-eim $0 install        # Install with eim profile
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
  list)
    helmfile_list
    ;;
  *)
    usage
    exit 1
    ;;
esac
