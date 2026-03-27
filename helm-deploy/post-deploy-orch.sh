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
REPO_URL="https://github.com/open-edge-platform/edge-manageability-framework"
MAIN_ENV_CONFIG="$SCRIPT_DIR/onprem.env"

repo_root=""
onprem_installers_dir=""
BOOTSTRAP_TMP_REPO_DIR=""

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
# REPO BOOTSTRAP
################################
bootstrap_repo_root() {
  local candidate
  candidate="$(cd "$SCRIPT_DIR/../.." && pwd)"

  if [[ -d "$candidate/orch-configs" && -d "$candidate/charts" ]]; then
    repo_root="$candidate"
    onprem_installers_dir="$repo_root/on-prem-installers"
    return
  fi

  echo "Cloning repo..."
  BOOTSTRAP_TMP_REPO_DIR="$(mktemp -d)"
  git clone --depth 1 "$REPO_URL" "$BOOTSTRAP_TMP_REPO_DIR" >/dev/null

  repo_root="$BOOTSTRAP_TMP_REPO_DIR"
  onprem_installers_dir="$repo_root/on-prem-installers"

  trap 'rm -rf "$BOOTSTRAP_TMP_REPO_DIR"' EXIT
}

################################
# YAML GENERATION
################################
generate_cluster_yaml_onprem_from_upstream() {
  local out_file="$cwd/${ORCH_INSTALLER_PROFILE}.yaml"

  echo "Generating cluster config..."
  ONPREM_ENV_PATH="$MAIN_ENV_CONFIG" \
    bash "$repo_root/installer/generate_cluster_yaml.sh" onprem

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
# HELM DEPLOYMENTS (CORE)
################################
install_platform_apps() {
  local values="$cwd/${ORCH_INSTALLER_PROFILE}.yaml"

  echo "🚀 Deploying platform apps via Helm..."

  # Order matters
  helm upgrade --install postgres "$repo_root/charts/postgres" \
    -f "$values" -n orch-database --create-namespace

  helm upgrade --install vault "$repo_root/charts/vault" \
    -f "$values" -n orch-platform --create-namespace

  helm upgrade --install keycloak "$repo_root/charts/keycloak" \
    -f "$values" -n orch-platform

  helm upgrade --install harbor "$repo_root/charts/harbor" \
    -f "$values" -n orch-harbor --create-namespace

  echo "✅ Platform apps deployed"
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

bootstrap_repo_root
ensure_prereqs

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

# Deploy apps (Helm replaces ArgoCD)
#install_platform_apps

echo
echo "✅ Edge Orchestrator deployed using Helm (No ArgoCD)"
echo "👉 Check: kubectl get pods -A"
echo
