#!/bin/bash
set -e

# ----------------------------
# Source environment
# ----------------------------
source ../onprem.env

ACTION="${1:-install}"
CUSTOM_VALUES="$2"

# ----------------------------
# Config
# ----------------------------
RELEASE_NAME="secret-wait-tls-orch"
NAMESPACE="orch-gateway"
CHART="oci://${RELEASE_SERVICE_URL}/edge-orch/common/charts/secret-wait"
VERSION="25.2.3"
BASE_VALUES="values.yaml"

# Proxy (optional, can override via env)
HTTP_PROXY="${HTTP_PROXY:-}"
HTTPS_PROXY="${HTTPS_PROXY:-}"
NO_PROXY="${NO_PROXY:-localhost,127.0.0.1}"

# Cluster info
CLUSTER_NAME="${CLUSTER_NAME:-onprem}"
CLUSTER_DOMAIN="${CLUSTER_DOMAIN:-cluster.onprem}"

# ----------------------------
# Pre-checks
# ----------------------------
command -v helm >/dev/null || { echo "❌ Helm not installed"; exit 1; }
command -v kubectl >/dev/null || { echo "❌ kubectl not installed"; exit 1; }
kubectl cluster-info >/dev/null 2>&1 || { echo "❌ Cluster not reachable"; exit 1; }

# ----------------------------
# Build Helm values args
# ----------------------------
VALUES_ARGS="-f ${BASE_VALUES}"
[[ -n "$CUSTOM_VALUES" ]] && VALUES_ARGS="$VALUES_ARGS -f $CUSTOM_VALUES"

# ----------------------------
# Generate temporary values.yaml overrides
# ----------------------------
TMP_VALUES=$(mktemp)
cat <<EOF > "$TMP_VALUES"
clusterName: "${CLUSTER_NAME}"
clusterDomain: "${CLUSTER_DOMAIN}"

proxy:
  httpProxy: "${HTTP_PROXY}"
  httpsProxy: "${HTTPS_PROXY}"
  noProxy: "${NO_PROXY}"

releaseService:
  ociRegistry: "${RELEASE_SERVICE_URL}"
  fileServer: "files-rs.edgeorchestration.intel.com"
  tokenRefresh: false
EOF

# ----------------------------
# Functions
# ----------------------------
install_app() {
  echo "🚀 Installing ${RELEASE_NAME}..."

  helm upgrade --install "${RELEASE_NAME}" "${CHART}" \
    --version "${VERSION}" \
    -n "${NAMESPACE}" \
    --create-namespace \
    ${VALUES_ARGS} \
    -f "$TMP_VALUES" \
    --wait

  echo "✅ Deployment successful"
  kubectl get pods -n "${NAMESPACE}"
}

uninstall_app() {
  echo "🗑️ Uninstalling ${RELEASE_NAME}..."
  helm status "${RELEASE_NAME}" -n "${NAMESPACE}" >/dev/null 2>&1 && \
    helm uninstall "${RELEASE_NAME}" -n "${NAMESPACE}" && echo "✅ Uninstalled" || \
    echo "⚠️ Release not found"
}

# ----------------------------
# Main
# ----------------------------
echo "-----------------------------------"
echo "Action    : $ACTION"
echo "Release   : $RELEASE_NAME"
echo "Namespace : $NAMESPACE"
echo "Chart     : $CHART"
echo "Version   : $VERSION"
echo "Registry  : $RELEASE_SERVICE_URL"
echo "-----------------------------------"

case "$ACTION" in
  install)   install_app ;;
  uninstall) uninstall_app ;;
  *)         echo "❌ Invalid action: $ACTION"; exit 1 ;;
esac

# Cleanup
[[ -f "$TMP_VALUES" ]] && rm -f "$TMP_VALUES"
