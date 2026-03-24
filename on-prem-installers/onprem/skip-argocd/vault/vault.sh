#!/bin/bash

set -e

# Usage:
# ./deploy-vault.sh install [custom-values.yaml]
# ./deploy-vault.sh uninstall

ACTION="${1:-install}"
CUSTOM_VALUES="$2"

# Load env
source ../onprem.env

NAMESPACE="orch-platform"
RELEASE_NAME="vault"

CHART="hashicorp/vault"
VERSION="0.32.0"

BASE_VALUES="values.yaml"

# ---------------- COMMON ----------------

prechecks() {
  command -v helm >/dev/null || { echo "❌ Helm not installed"; exit 1; }
  command -v kubectl >/dev/null || { echo "❌ kubectl not installed"; exit 1; }
  kubectl cluster-info >/dev/null 2>&1 || { echo "❌ Cluster not reachable"; exit 1; }
}

build_values_args() {
  VALUES_ARGS="-f ${BASE_VALUES}"
  if [[ -n "$CUSTOM_VALUES" ]]; then
    VALUES_ARGS="$VALUES_ARGS -f $CUSTOM_VALUES"
  fi
}

create_temp_values() {
  TMP_VALUES=$(mktemp)

  cat <<EOF > "$TMP_VALUES"
global:
  imagePullSecrets: []

server:
  extraEnvironmentVars:
    http_proxy: "${HTTP_PROXY}"
    https_proxy: "${HTTPS_PROXY}"
    no_proxy: "${NO_PROXY}"
EOF
}

cleanup() {
  [[ -f "$TMP_VALUES" ]] && rm -f "$TMP_VALUES"
}

# ---------------- INSTALL ----------------

install_app() {
  echo "🚀 Installing Vault..."

  helm repo add hashicorp https://helm.releases.hashicorp.com || true
  helm repo update

  build_values_args
  create_temp_values

  helm upgrade --install ${RELEASE_NAME} ${CHART} \
    --version ${VERSION} \
    -n ${NAMESPACE} \
    --create-namespace \
    ${VALUES_ARGS} \
    -f "$TMP_VALUES" \
    --wait

  echo "✅ Vault deployed"
  kubectl get pods -n ${NAMESPACE}

  cleanup
}

# ---------------- UNINSTALL ----------------

uninstall_app() {
  echo "🗑️ Uninstalling Vault..."

  if helm status ${RELEASE_NAME} -n ${NAMESPACE} >/dev/null 2>&1; then
    helm uninstall ${RELEASE_NAME} -n ${NAMESPACE}
    echo "✅ Uninstalled"
  else
    echo "⚠️ Not installed"
  fi
}

# ---------------- MAIN ----------------

echo "-----------------------------------"
echo "Action    : $ACTION"
echo "Namespace : $NAMESPACE"
echo "-----------------------------------"

prechecks

case "$ACTION" in
  install)
    install_app
    ;;
  uninstall)
    uninstall_app
    ;;
  *)
    echo "❌ Invalid action"
    exit 1
    ;;
esac
