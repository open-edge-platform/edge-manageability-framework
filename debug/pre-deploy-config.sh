#!/bin/bash

# SPDX-License-Identifier: Apache-2.0
#
# Pre-deployment configuration: namespaces, secrets, passwords, Gitea
# Run this before helmfile-deploy.sh, or use post-deploy-orch.sh which calls both.
#
# Usage:
#   ./pre-deploy-config.sh setup       # Create namespaces, secrets, passwords
#   ./pre-deploy-config.sh cleanup     # Remove secrets and namespaces

set -e
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MAIN_ENV_CONFIG="$SCRIPT_DIR/onprem.env"

# Import shared functions
source "$SCRIPT_DIR/functions.sh"

gitea_ns="gitea"
GITEA_CHART_VERSION="10.4.0"

################################
# PREREQUISITES
################################
require_cmd() { command -v "$1" >/dev/null 2>&1; }

ensure_prereqs() {
  require_cmd kubectl || { echo "❌ kubectl not found"; exit 1; }
  require_cmd helm || { echo "❌ helm not found"; exit 1; }
}

################################
# NAMESPACES
################################
create_namespaces() {
  echo "📁 Creating namespaces..."
  local ns_list=(
    onprem orch-boots orch-database orch-platform
    orch-app orch-cluster orch-infra orch-sre
    orch-ui orch-secret orch-gateway orch-harbor
    cattle-system
  )

  for ns in "${ns_list[@]}"; do
    kubectl create ns "$ns" --dry-run=client -o yaml | kubectl apply -f -
  done
  echo "✅ Namespaces created"
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

  echo "🔐 Creating SMTP secrets..."
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

  echo "🔐 Creating SRE secrets..."
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

create_passwords() {
  echo "🔐 Creating passwords..."
  local harbor_password
  harbor_password=$(openssl rand -hex 50)
  local keycloak_password
  keycloak_password=$(generate_password)
  local postgres_password
  postgres_password=$(generate_password)

  create_harbor_secret orch-harbor "$harbor_password"
  create_harbor_password orch-harbor "$harbor_password"
  create_keycloak_password orch-platform "$keycloak_password"
  create_postgres_password orch-database "$postgres_password"
  echo "✅ Passwords created"
}

################################
# GITEA
################################
install_gitea() {
  if [[ "${EMF_GITEA_ENABLED:-false}" != "true" ]]; then
    echo "Skipping Gitea (EMF_GITEA_ENABLED=${EMF_GITEA_ENABLED:-false})"
    return
  fi

  echo "📦 Installing Gitea..."
  helm repo add gitea-charts https://dl.gitea.com/charts/ --force-update >/dev/null
  kubectl create ns gitea >/dev/null 2>&1 || true

  helm upgrade --install gitea gitea-charts/gitea \
    --version "$GITEA_CHART_VERSION" \
    -n gitea --wait

  echo "✅ Gitea installed"
}

################################
# CLEANUP
################################
cleanup_all() {
  echo "🗑️  Removing pre-deploy resources..."

  # Remove secrets
  kubectl -n orch-infra delete secret smtp --ignore-not-found
  kubectl -n orch-sre delete secret basic-auth-username --ignore-not-found
  kubectl -n orch-harbor delete secret harbor-admin-credential --ignore-not-found
  kubectl -n orch-harbor delete secret harbor-admin-password --ignore-not-found
  kubectl -n orch-platform delete secret platform-keycloak --ignore-not-found
  kubectl -n orch-database delete secret orch-database-postgresql --ignore-not-found

  # Remove namespaces
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

  echo "✅ Cleanup complete"
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

export KUBECONFIG="${KUBECONFIG:-/home/${SUDO_USER:-$USER}/.kube/config}"

ensure_prereqs

ACTION="${1:-setup}"

case "$ACTION" in
  install)
    install_gitea
    create_namespaces
    create_sre_secrets
    create_smtp_secrets
    create_passwords
    echo
    echo "✅ Pre-deploy configuration complete"
    ;;
  uninstall)
    cleanup_all
    ;;
  *)
    echo "Usage: $0 [setup|cleanup]"
    exit 1
    ;;
esac
