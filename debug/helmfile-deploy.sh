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

set -e
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MAIN_ENV_CONFIG="$SCRIPT_DIR/onprem.env"

################################
# VALIDATION
################################
VALID_PROFILES="onprem onprem-1k onprem-oxm onprem-explicit-proxy aws vpro eim eim-co eim-co-ao eim-co-ao-o11y dev dev-minimal bkc"

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

  # Try a full sync first (normal case)
  if helmfile_cmd sync; then
    echo "✅ All charts installed (full sync)"
    return 0
  fi

  # Fallback: sync each labeled release individually to ensure every chart is attempted.
  echo "⚠️  Full sync failed or only partially applied — falling back to per-release sync"
  local labels
  labels=$(awk '/^\s*labels:/ {getline; if ($0 ~ /app:/) {gsub(/.*app:[[:space:]]*/, "", $0); print $0}}' "$SCRIPT_DIR/helmfile.yaml" | sed 's/\r//g' | tr -d '"')
  if [[ -z "$labels" ]]; then
    echo "❌ No labeled releases found in helmfile.yaml — aborting"
    return 1
  fi

  local failed=0
  for lbl in $labels; do
    echo "📦 Installing release with label app=$lbl"
    if ! helmfile_cmd sync -l "app=$lbl"; then
      echo "❌ Failed to sync release: $lbl"
      failed=$((failed+1))
    fi
  done

  if (( failed == 0 )); then
    echo "✅ All labeled charts attempted"
    return 0
  else
    echo "❌ Some releases failed: $failed"
    return 1
  fi
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
  EMF_HELMFILE_ENV=eim $0 install        # Install with eim profile
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
