#!/bin/bash

# SPDX-License-Identifier: Apache-2.0
#
# Pre-deployment configuration: namespaces, secrets, passwords, Gitea
# Run this before post-orch-deploy.sh.
#
# Usage:
#   ./post-orch-setup.sh setup       # Create namespaces, secrets, passwords
#   ./post-orch-setup.sh cleanup     # Remove secrets and namespaces

set -e
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MAIN_ENV_CONFIG="$SCRIPT_DIR/post-orch.env"

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
    orch-boots orch-database orch-platform
    orch-infra
    orch-ui orch-secret orch-gateway
    cattle-system
  )
  #namespace not require for eim profile onprem orch-cluster orch-app orch-sre orch-harbor

  for ns in "${ns_list[@]}"; do
    kubectl create ns "$ns" --dry-run=client -o yaml | kubectl apply -f -
  done
  echo "✅ Namespaces created"
}

################################
# SECRETS
################################
create_smtp_secrets() {
  if [[ "${EMF_ENABLE_EMAIL:-false}" != "true" ]]; then
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
  #local harbor_password
  #harbor_password=$(openssl rand -hex 50)
  local keycloak_password
  keycloak_password=$(generate_password)
  local postgres_password
  postgres_password=$(generate_password)

  #create_harbor_secret orch-harbor "$harbor_password"
  #create_harbor_password orch-harbor "$harbor_password"
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

# Remove orphaned CAPI finalizers and stale webhooks that prevent namespace deletion.
# When capi-providers-config or cluster-manager is uninstalled before its CRDs/webhooks
# are removed, namespaces like capi-system, capk-system and tenant cluster namespaces
# get stuck in Terminating.
cleanup_capi_finalizers() {
  echo "🔧 Cleaning up CAPI finalizers and stale webhooks..."

  # Remove stale validating webhooks that reference deleted CAPI services
  kubectl get validatingwebhookconfiguration -o name 2>/dev/null | grep -E 'cluster\.x-k8s\.io|capi' | while read -r wh; do
    echo "  Removing stale webhook: $wh"
    kubectl delete "$wh" --ignore-not-found 2>/dev/null || true
  done || true
  kubectl get mutatingwebhookconfiguration -o name 2>/dev/null | grep -E 'cluster\.x-k8s\.io|capi' | while read -r wh; do
    echo "  Removing stale webhook: $wh"
    kubectl delete "$wh" --ignore-not-found 2>/dev/null || true
  done || true

  # Patch CAPI provider resources to remove finalizers in known namespaces
  local capi_ns_list=(capi-system capk-system capi-operator-system)
  local capi_kinds=(
    coreproviders.operator.cluster.x-k8s.io
    controlplaneproviders.operator.cluster.x-k8s.io
    infrastructureproviders.operator.cluster.x-k8s.io
    bootstrapproviders.operator.cluster.x-k8s.io
  )
  for ns in "${capi_ns_list[@]}"; do
    if kubectl get ns "$ns" >/dev/null 2>&1; then
      for kind in "${capi_kinds[@]}"; do
        kubectl get "$kind" -n "$ns" -o name 2>/dev/null | while read -r res; do
          echo "  Removing finalizers from $res in $ns"
          kubectl patch "$res" -n "$ns" --type merge -p '{"metadata":{"finalizers":null}}' 2>/dev/null || true
        done || true
      done
    fi
  done

  # Patch CAPI resources in GUID (tenant cluster) namespaces
  local tenant_kinds=(
    clusters.cluster.x-k8s.io
    clusterclasses.cluster.x-k8s.io
    machines.cluster.x-k8s.io
    machinesets.cluster.x-k8s.io
    machinedeployments.cluster.x-k8s.io
    kthreescontrolplanes.controlplane.cluster.x-k8s.io
    intelclusters.infrastructure.cluster.x-k8s.io
    intelmachines.infrastructure.cluster.x-k8s.io
    clustertemplates.edge-orchestrator.intel.com
  )
  local GUID_REGEX='^[a-f0-9\-]{36}$'
  kubectl get ns -o jsonpath='{.items[*].metadata.name}' | tr ' ' '\n' | \
  grep -E "$GUID_REGEX" | while read -r ns; do
    for kind in "${tenant_kinds[@]}"; do
      kubectl get "$kind" -n "$ns" -o name 2>/dev/null | while read -r res; do
        echo "  Removing finalizers from $res in $ns"
        kubectl patch "$res" -n "$ns" --type merge -p '{"metadata":{"finalizers":null}}' 2>/dev/null || true
      done || true
    done
  done || true

  echo "✅ CAPI finalizer cleanup complete"
}

cleanup_all() {
  echo "🗑️  Removing pre-deploy resources..."

  # Remove secrets
  kubectl -n orch-infra delete secret smtp --ignore-not-found
  kubectl -n orch-sre delete secret basic-auth-username --ignore-not-found
  kubectl -n orch-harbor delete secret harbor-admin-credential --ignore-not-found
  kubectl -n orch-harbor delete secret harbor-admin-password --ignore-not-found
  kubectl -n orch-platform delete secret platform-keycloak --ignore-not-found
  kubectl -n orch-database delete secret orch-database-postgresql --ignore-not-found

  # Delete cluster manager / template controller stale resources in default namespace
  kubectl -n default get svc -o name 2>/dev/null | grep -E 'cluster-manager|cluster-template' | while read -r svc; do
    echo "Deleting $svc"
    kubectl -n default delete "$svc" --ignore-not-found
  done || true
  kubectl -n default get deploy -o name 2>/dev/null | grep -E 'cluster-manager|cluster-template' | while read -r dep; do
    echo "Deleting $dep"
    kubectl -n default delete "$dep" --ignore-not-found
  done || true

   sleep 5

  # Clean up CAPI finalizers and stale webhooks before deleting namespaces
  cleanup_capi_finalizers

  # Remove namespaces
  local ns_list=(
    onprem orch-boots orch-platform
    orch-app orch-cluster orch-infra orch-sre
    orch-ui orch-secret orch-gateway orch-harbor
    cattle-system orch-iam capi-variables capi-operator-system
    capi-system capk-system cert-manager ns-label orch-database
  )
  echo "🗑️  Deleting namespaces..."
  for ns in "${ns_list[@]}"; do
    kubectl delete ns "$ns" --ignore-not-found --wait=false 2>/dev/null || true
  done

 
  REGEX='^[a-f0-9\-]{36}$'

  kubectl get ns -o jsonpath='{.items[*].metadata.name}' | tr ' ' '\n' | \
  grep -E "$REGEX" | while read -r ns; do
    echo "Deleting $ns"
    kubectl delete ns "$ns" --wait=false 2>/dev/null || true
  done || true

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
  echo "❌ Missing post-orch.env"
  exit 1
fi

export KUBECONFIG="${KUBECONFIG:-/home/${SUDO_USER:-$USER}/.kube/config}"

ensure_prereqs

ACTION="${1:-setup}"

case "$ACTION" in
  install)
    #install_gitea
    create_namespaces
    #create_sre_secrets
    #create_smtp_secrets
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
