#!/bin/bash

# SPDX-License-Identifier: Apache-2.0
#
# Pre-deployment configuration: namespaces, secrets, passwords
# Run this before post-orch-deploy.sh.
#
# Usage:
#   ./pre-orch-config.sh install     # Create namespaces, secrets, passwords
#   ./pre-orch-config.sh uninstall   # Remove secrets and namespaces

set -e
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Import shared functions (generate_password, create_keycloak_password, create_postgres_password)
source "$SCRIPT_DIR/functions.sh"

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
    orch-boots orch-database orch-platform
    orch-infra orch-iam
    orch-ui orch-secret orch-gateway
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

create_passwords() {
  echo "🔐 Creating passwords..."
  local keycloak_password
  keycloak_password=$(generate_password)
  local postgres_password
  postgres_password=$(generate_password)
  

  create_keycloak_password orch-platform "$keycloak_password"
  create_postgres_password orch-database "$postgres_password"
  echo "✅ Passwords created"
}

################################
# VAULT CLEANUP
################################
remove_stale_vault_keys() {
  # If vault-keys secret exists but the root token is invalid, remove it
  # so secrets-config can re-initialize vault with a fresh token.
  local ns="orch-platform"
  local token

  if ! kubectl get secret vault-keys -n "$ns" >/dev/null 2>&1; then
    return
  fi

  # Check if vault pod is running
  if ! kubectl get pod vault-0 -n "$ns" >/dev/null 2>&1; then
    echo "🔑 Removing vault-keys secret (vault not running)"
    kubectl delete secret vault-keys -n "$ns" 2>/dev/null || true
    return
  fi

  # Extract root token and validate it
  token=$(kubectl get secret vault-keys -n "$ns" -o jsonpath='{.data.vault-keys}' 2>/dev/null \
    | base64 -d 2>/dev/null | grep -o '"root_token":"[^"]*"' | cut -d'"' -f4)

  if [[ -n "$token" ]]; then
    if ! kubectl exec vault-0 -n "$ns" -c vault -- vault token lookup "$token" >/dev/null 2>&1; then
      echo "🔑 Removing stale vault-keys secret (invalid root token)"
      kubectl delete secret vault-keys -n "$ns" 2>/dev/null || true
    else
      echo "✅ vault-keys secret is valid"
    fi
  fi
}

################################
# CLEANUP
################################

cleanup_all() {
  echo "🗑️  Removing pre-deploy resources..."

  # Remove secrets
  kubectl -n orch-platform delete secret platform-keycloak --ignore-not-found
  kubectl -n orch-database delete secret orch-database-postgresql --ignore-not-found

  # Remove namespaces
  local ns_list=(
    orch-boots orch-platform
    orch-infra orch-iam
    orch-ui orch-secret orch-gateway
    cattle-system istio-system kyverno
    cert-manager ns-label orch-database
    postgresql-operator
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
export KUBECONFIG="${KUBECONFIG:-/home/${SUDO_USER:-$USER}/.kube/config}"

ensure_prereqs

ACTION="${1:-setup}"

case "$ACTION" in
  install)
    create_namespaces
    create_passwords
    remove_stale_vault_keys
    echo
    echo "✅ Pre-deploy configuration complete"
    ;;
  uninstall)
    cleanup_all
    ;;
  *)
    echo "Usage: $0 [install|uninstall]"
    exit 1
    ;;
esac
