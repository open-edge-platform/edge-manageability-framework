#!/bin/bash

set -e

# Usage:
# ./deploy-self-signed-cert.sh install [custom-values.yaml]
# ./deploy-self-signed-cert.sh uninstall

ACTION="${1:-install}"
CUSTOM_VALUES="$2"

# Load env
source ../onprem.env

# Validate required env
: "${RELEASE_SERVICE_URL:?❌ RELEASE_SERVICE_URL not set}"
: "${CLUSTER_DOMAIN:?❌ CLUSTER_DOMAIN not set}"

CHART="oci://${RELEASE_SERVICE_URL}/edge-orch/common/charts/self-signed-cert"
VERSION="4.0.11"
RELEASE_NAME="self-signed-cert"
NAMESPACE="cert-manager"

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
certDomain: "${CLUSTER_DOMAIN}"
generateOrchCert: ${GENERATE_ORCH_CERT:-false}
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
    --wait

  echo "-----------------------------------"
  echo "✅ Deployment successful"
  echo "certDomain: ${CLUSTER_DOMAIN}"
  echo "-----------------------------------"

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
echo "Action          : $ACTION"
echo "Cluster Domain  : $CLUSTER_DOMAIN"
echo "Registry        : $RELEASE_SERVICE_URL"
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
