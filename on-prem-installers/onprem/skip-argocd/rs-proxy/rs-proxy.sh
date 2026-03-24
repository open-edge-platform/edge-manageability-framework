#!/bin/bash
set -e

# ----------------------------
# Source environment
# ----------------------------
source ../onprem.env

ACTION="${1:-install}"
CUSTOM_VALUES="$2"

# ----------------------------
# Configuration / Variables
# ----------------------------
RELEASE_NAME="rs-proxy"
NAMESPACE="orch-platform"
CHART="oci://${RELEASE_SERVICE_URL}/edge-orch/common/charts/rs-proxy"
VERSION="25.2.3"
BASE_VALUES="values.yaml"

# Argo-like values
FILE_SERVER_URL="${FILE_SERVER_URL:-files-rs.edgeorchestration.intel.com}"
CHART_REPO_URL="${CHART_REPO_URL:-${RELEASE_SERVICE_URL}/edge-orch}"
RS_CHART_REPO_URL="${RS_CHART_REPO_URL:-${RELEASE_SERVICE_URL}/edge-orch}"
CONTAINER_REGISTRY_URL="${CONTAINER_REGISTRY_URL:-${RELEASE_SERVICE_URL}/edge-orch}"
TOKEN_REFRESH="${TOKEN_REFRESH:-false}"

# Proxy (can also come from env)
HTTP_PROXY="${HTTP_PROXY:-}"
HTTPS_PROXY="${HTTPS_PROXY:-}"
NO_PROXY="${NO_PROXY:-localhost,127.0.0.1}"

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
# Generate temp values.yaml
# ----------------------------
TMP_VALUES=$(mktemp)
cat <<EOF > "$TMP_VALUES"
withReleaseServiceToken: ${TOKEN_REFRESH}

proxyTargetRegistry: "${CONTAINER_REGISTRY_URL}"
proxyTargetFiles: "${FILE_SERVER_URL}"
proxyTargetCA: ""

env:
$( [[ -n "$HTTPS_PROXY" ]] && echo "  - name: https_proxy
    value: \"$HTTPS_PROXY\"" )
$( [[ -n "$HTTP_PROXY" ]] && echo "  - name: http_proxy
    value: \"$HTTP_PROXY\"" )
$( [[ -n "$NO_PROXY" ]] && echo "  - name: no_proxy
    value: \"$NO_PROXY\"" )
EOF

# ----------------------------
# Functions
# ----------------------------
install_app() {
  echo "🚀 Installing ${RELEASE_NAME}..."
  echo "Using values: $VALUES_ARGS"

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
echo "Registry  : $RELEASE_SERVICE_URL"
echo "-----------------------------------"

case "$ACTION" in
  install)   install_app ;;
  uninstall) uninstall_app ;;
  *)         echo "❌ Invalid action: $ACTION"; exit 1 ;;
esac

# Cleanup
[[ -f "$TMP_VALUES" ]] && rm -f "$TMP_VALUES"
