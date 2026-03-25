#!/bin/bash
# haproxy-ingress-pxe-boots.sh
# Install/Upgrade or Uninstall HAProxy Ingress via Helm using OCI chart directly

set -euo pipefail

usage() {
    echo "Usage: $0 [install|uninstall]"
    exit 1
}

# ----------------------------
# Load environment variables
# ----------------------------
ENV_FILE="../onprem.env"
if [[ -f "$ENV_FILE" ]]; then
    echo "Sourcing environment variables from $ENV_FILE"
    source "$ENV_FILE"
else
    echo "❌ Environment file $ENV_FILE not found!"
    exit 1
fi

# ----------------------------
# Variables
# ----------------------------
RELEASE_NAME="haproxy-ingress-pxe-boots"
NAMESPACE="orch-boots"
VALUES_TEMPLATE="./values.yaml"
VALUES_FILE="./values-rendered.yaml"

RELEASE_SERVICE_URL="${RELEASE_SERVICE_URL:-registry-rs.edgeorchestration.intel.com}"
CHART_NAME="edge-orch/common/charts/haproxy-ingress-pxe-boots"
CHART_VERSION="1.0.1"

CHART="oci://${RELEASE_SERVICE_URL}/${CHART_NAME}:${CHART_VERSION}"

# ----------------------------
# Prepare values file
# ----------------------------
envsubst < "$VALUES_TEMPLATE" > "$VALUES_FILE"

# Ensure namespace exists
kubectl get ns "$NAMESPACE" &>/dev/null || kubectl create ns "$NAMESPACE"

# ----------------------------
# Install
# ----------------------------
install_ha_proxy() {
    echo "🚀 Installing/upgrading HAProxy Ingress..."

    helm upgrade --install "$RELEASE_NAME" "$CHART" \
        --namespace "$NAMESPACE" \
        -f "$VALUES_FILE" \
        --atomic \
        --wait

    echo "✅ HAProxy Ingress installed/upgraded successfully in namespace $NAMESPACE"
    echo "Chart: $CHART"
}

# ----------------------------
# Uninstall (SAFE)
# ----------------------------
uninstall_ha_proxy() {
    echo "🧹 Uninstalling HAProxy Ingress..."

    # Safe helm uninstall (never fail)
    helm uninstall "$RELEASE_NAME" -n "$NAMESPACE" 2>/dev/null || \
        echo "⚠️ Release $RELEASE_NAME not found or already removed, skipping"

    echo "🧹 Cleaning up TLS resources..."

    # Safe cleanup (never fail)
    kubectl delete certificate tls-boots -n "$NAMESPACE" --ignore-not-found || true
    kubectl delete secret tls-boots -n "$NAMESPACE" --ignore-not-found || true

    echo "✅ HAProxy Ingress and TLS resources cleaned up"

    # Ensure success for run-all.sh
    true
}

# ----------------------------
# Main
# ----------------------------
if [[ $# -ne 1 ]]; then
    usage
fi

ACTION="$1"

case "$ACTION" in
    install)
        install_ha_proxy
        ;;
    uninstall)
        uninstall_ha_proxy
        ;;
    *)
        usage
        ;;
esac
