#!/bin/bash

# SPDX-License-Identifier: Apache-2.0

set -e
set -o pipefail

# Import shared functions
source "$(dirname "$0")/functions.sh"

ASSUME_YES=false
ENABLE_TRACE=false

gitea_ns="gitea"
GITEA_CHART_VERSION="10.4.0"

cwd=$(pwd)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
MAIN_ENV_CONFIG="$SCRIPT_DIR/onprem.env"

################################
# PREREQUISITES
################################
require_cmd() { command -v "$1" >/dev/null 2>&1; }

ensure_prereqs() {
  require_cmd kubectl || { echo "❌ kubectl not found"; exit 1; }
  require_cmd helm || { echo "❌ helm not found"; exit 1; }

  if ! require_cmd yq; then
    echo "Installing yq..."
    install_yq
  fi
}

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
# YAML GENERATION
################################
generate_cluster_yaml_onprem_from_upstream() {
  local out_file="$cwd/${ORCH_INSTALLER_PROFILE}.yaml"

  echo "Generating cluster config..."
  ONPREM_ENV_PATH="$MAIN_ENV_CONFIG" \
    bash "$SCRIPT_DIR/generate_cluster_yaml.sh" onprem

  [[ -r "$out_file" ]] || { echo "❌ YAML not generated"; exit 1; }

  echo "Generated: $out_file"
}

################################
# NAMESPACES
################################
create_namespaces() {
  local ns_list=(
    onprem orch-boots orch-database orch-platform
    orch-app orch-cluster orch-infra orch-sre
    orch-ui orch-secret orch-gateway orch-harbor
    cattle-system
  )

  for ns in "${ns_list[@]}"; do
    kubectl create ns "$ns" --dry-run=client -o yaml | kubectl apply -f -
  done
}

################################
# SECRETS
################################
create_smtp_secrets() {
  if [[ "${EMF_ENABLE_EMAIL:-true}" != "true" ]]; then
    echo "Skipping SMTP secrets (email disabled)"
    return
  fi

  if [[ -z "${EMF_SMTP_ADDRESS:-}" ]]; then
    echo "Skipping SMTP secrets (EMF_SMTP_ADDRESS not set)"
    return
  fi

  kubectl -n orch-infra delete secret smtp --ignore-not-found

  kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: smtp
  namespace: orch-infra
stringData:
  smartHost: $EMF_SMTP_ADDRESS
  smartPort: "$EMF_SMTP_PORT"
  from: $EMF_SMTP_HEADER
  authUsername: $EMF_SMTP_USERNAME
EOF
}

create_sre_secrets() {
  if [[ -z "${EMF_SRE_USERNAME:-}" ]]; then
    echo "Skipping SRE secrets (EMF_SRE_USERNAME not set)"
    return
  fi

  kubectl -n orch-sre delete secret basic-auth-username --ignore-not-found

  kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: basic-auth-username
  namespace: orch-sre
stringData:
  username: $EMF_SRE_USERNAME
EOF
}

################################
# GITEA
################################
install_gitea_from_repo() {
  echo "Installing Gitea..."

  helm repo add gitea-charts https://dl.gitea.com/charts/ --force-update >/dev/null
  kubectl create ns gitea >/dev/null 2>&1 || true

  helm upgrade --install gitea gitea-charts/gitea \
    --version "$GITEA_CHART_VERSION" \
    -n gitea --wait

  echo "✅ Gitea installed"
}

################################
# UNINSTALL
################################
uninstall_all() {
  echo "🗑️  Removing resources created by post-deploy..."

  # Remove secrets created by this script
  kubectl -n orch-infra delete secret smtp --ignore-not-found
  kubectl -n orch-sre delete secret basic-auth-username --ignore-not-found
  kubectl -n orch-harbor delete secret harbor-admin-credential --ignore-not-found
  kubectl -n orch-harbor delete secret harbor-admin-password --ignore-not-found
  kubectl -n orch-platform delete secret platform-keycloak --ignore-not-found
  kubectl -n orch-database delete secret orch-database-postgresql --ignore-not-found

  # Remove namespaces created by this script
  local ns_list=(
    onprem orch-boots orch-database orch-platform
    orch-app orch-cluster orch-infra orch-sre
    orch-ui orch-secret orch-gateway orch-harbor
    cattle-system
  )
  echo "🗑️  Deleting namespaces..."
  for ns in "${ns_list[@]}"; do
    kubectl delete ns "$ns" --ignore-not-found --wait=false 2>/dev/null || true
  done

  echo "✅ Uninstall complete"
}

HELMFILE_ENV="${EMF_HELMFILE_ENV:-onprem}"
HELMFILE_DIR="$SCRIPT_DIR"

################################
# HELMFILE WRAPPER
################################
helmfile_cmd() {
  local action="$1"
  shift
  (cd "$HELMFILE_DIR" && helmfile -e "$HELMFILE_ENV" "$@" "$action")
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
  helmfile_cmd sync
  echo "✅ All charts installed"
}

helmfile_destroy_all() {
  echo "🗑️  Uninstalling all charts (env: $HELMFILE_ENV)"
  helmfile_cmd destroy
  echo "✅ All charts uninstalled"
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

ensure_prereqs
validate_config

usage() {
  cat <<EOF
Usage: $0 <action> [chart-name]

Actions:
  install              Install all charts (full deployment)
  install <chart>      Install a single chart (e.g., traefik, vault, harbor)
  uninstall            Uninstall all (secrets, namespaces, and helmfile charts)
  uninstall <chart>    Uninstall a single chart
  list                 List all available charts and their status

Environment:
  EMF_HELMFILE_ENV     Helmfile environment (default: onprem)

Examples:
  $0 install                             # Full deployment
  $0 install traefik                     # Install only traefik
  $0 uninstall traefik                   # Uninstall only traefik
  EMF_HELMFILE_ENV=eim $0 install        # Full deployment with eim profile
  $0 list                                # List all charts
EOF
}

ACTION="${1:-}"
CHART_NAME="${2:-}"

case "$ACTION" in
  install)
    # kubeconfig
    export KUBECONFIG="${KUBECONFIG:-/home/${SUDO_USER:-$USER}/.kube/config}"

    if [[ -n "$CHART_NAME" ]]; then
      # Individual chart install
      helmfile_sync_chart "$CHART_NAME"
    else
      # Full install
      # Gitea (optional)
      if [[ "${EMF_GITEA_ENABLED:-false}" == "true" ]]; then
        install_gitea_from_repo
      else
        echo "Skipping Gitea (EMF_GITEA_ENABLED=${EMF_GITEA_ENABLED:-false})"
      fi

      # Infra setup
      create_namespaces
      create_sre_secrets
      create_smtp_secrets

      # Passwords
      harbor_password=$(openssl rand -hex 50)
      keycloak_password=$(generate_password)
      postgres_password=$(generate_password)

      create_harbor_secret orch-harbor "$harbor_password"
      create_harbor_password orch-harbor "$harbor_password"
      create_keycloak_password orch-platform "$keycloak_password"
      create_postgres_password orch-database "$postgres_password"

      # Deploy all helmfile releases
      helmfile_sync_all

      echo
      echo "✅ Edge Orchestrator deployed using Helm (No ArgoCD)"
      echo "👉 Check: kubectl get pods -A"
      echo
    fi
    ;;
  uninstall)
    export KUBECONFIG="${KUBECONFIG:-/home/${SUDO_USER:-$USER}/.kube/config}"

    if [[ -n "$CHART_NAME" ]]; then
      # Individual chart uninstall
      helmfile_destroy_chart "$CHART_NAME"
    else
      # Full uninstall
      helmfile_destroy_all
      uninstall_all
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
