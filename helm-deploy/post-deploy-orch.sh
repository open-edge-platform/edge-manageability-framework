#!/bin/bash

# SPDX-License-Identifier: Apache-2.0

set -e
set -o pipefail

# Import shared functions
source "$(dirname "$0")/functions.sh"

ASSUME_YES=false
ENABLE_TRACE=false
INSTALL_GITEA="flase"

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
  if [[ -z "${SMTP_ADDRESS:-}" ]]; then
    echo "Skipping SMTP secrets"
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
  smartHost: $SMTP_ADDRESS
  smartPort: "$SMTP_PORT"
  from: $SMTP_HEADER
  authUsername: $SMTP_USERNAME
EOF
}

create_sre_secrets() {
  if [[ -z "${SRE_USERNAME:-}" ]]; then
    echo "Skipping SRE secrets"
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
  username: $SRE_USERNAME
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

################################
# MAIN
################################
if [[ -f "$MAIN_ENV_CONFIG" ]]; then
  source "$MAIN_ENV_CONFIG"
else
  echo "❌ Missing onprem.env"
  exit 1
fi

ensure_prereqs

ACTION="${1:-install}"

case "$ACTION" in
  uninstall)
    uninstall_all
    exit 0
    ;;
  install)
    ;;
  *)
    echo "Usage: $0 [install|uninstall]"
    exit 1
    ;;
esac

generate_cluster_yaml_onprem_from_upstream

# kubeconfig
export KUBECONFIG="${KUBECONFIG:-/home/${SUDO_USER:-$USER}/.kube/config}"

# Gitea (optional)
#if [[ "${DISABLE_AO_PROFILE:-false}" != "true" ]]; then
  #install_gitea_from_repo
#  echo "skip gitea"
#else
#  echo "Skipping Gitea"
#fi

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

echo
echo "✅ Edge Orchestrator deployed using Helm (No ArgoCD)"
echo "👉 Check: kubectl get pods -A"
echo
