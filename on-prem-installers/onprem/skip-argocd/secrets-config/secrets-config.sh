#!/bin/bash

set -e

# Usage:
# ./deploy-secrets.sh install [custom-values.yaml]
# ./deploy-secrets.sh uninstall

ACTION="${1:-install}"
CUSTOM_VALUES="$2"

# Load environment
source ../onprem.env

# Validate required env
: "${RELEASE_SERVICE_URL:?❌ RELEASE_SERVICE_URL not set in onprem.env}"

CHART="oci://${RELEASE_SERVICE_URL}/edge-orch/common/charts/secrets-config"
VERSION="3.0.6"
RELEASE_NAME="secrets-config"
NAMESPACE="orch-platform"

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
proxy:
  httpProxy: "${HTTP_PROXY}"
  httpsProxy: "${HTTPS_PROXY}"
  noProxy: "${NO_PROXY}"
image:
  registry: "${RELEASE_SERVICE_URL}"
EOF
}

cleanup() {
  [[ -f "$TMP_VALUES" ]] && rm -f "$TMP_VALUES"
}

# ---------------- INSTALL ----------------

install_app() {
  echo "🚀 Installing ${RELEASE_NAME}..."

  build_values_args
  create_temp_values

  echo "Using values: $VALUES_ARGS"

  helm upgrade --install ${RELEASE_NAME} ${CHART} \
    --version ${VERSION} \
    -n ${NAMESPACE} \
    --create-namespace \
    ${VALUES_ARGS} \
    -f "$TMP_VALUES" \
    --set-json 'imagePullSecrets=[]' \
    --wait

  echo "✅ Deployment successful"
  kubectl get pods -n ${NAMESPACE}

  cleanup
}

# ---------------- UNINSTALL ----------------

uninstall_app() {
  echo "🗑️ Uninstalling ${RELEASE_NAME}..."

  if helm status ${RELEASE_NAME} -n ${NAMESPACE} >/dev/null 2>&1; then
    helm uninstall ${RELEASE_NAME} -n ${NAMESPACE}
    echo "✅ Uninstalled successfully"
  else
    echo "⚠️ Release not found"
  fi
}

# ---------------- MAIN ----------------

echo "-----------------------------------"
echo "Action    : $ACTION"
echo "Release   : $RELEASE_NAME"
echo "Namespace : $NAMESPACE"
echo "Registry  : $RELEASE_SERVICE_URL"
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
    echo "❌ Invalid action: $ACTION"
    echo "Usage: $0 [install|uninstall] [custom-values.yaml]"
    exit 1
    ;;
esac
