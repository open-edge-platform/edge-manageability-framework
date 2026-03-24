#!/bin/bash
# haproxy-ingress-pxe-boots.sh
# Install/Upgrade or Uninstall HAProxy Ingress via Helm using OCI chart directly
# Default registry: registry-rs.edgeorchestration.intel.com
# Removes TLS Certificate and Secret on uninstall
# Assumes cluster already has permission to pull OCI chart

set -e

usage() {
    echo "Usage: $0 [install|uninstall]"
    exit 1
}

# Load environment variables
source ../onprem.env

# Variables
RELEASE_NAME="haproxy-ingress-pxe-boots"
NAMESPACE="orch-boots"
VALUES_TEMPLATE="./values.yaml"
VALUES_FILE="./values-rendered.yaml"

# Default OCI registry if RELEASE_SERVICE_URL not set
RELEASE_SERVICE_URL="${RELEASE_SERVICE_URL:-registry-rs.edgeorchestration.intel.com}"

CHART_NAME="edge-orch/common/charts/haproxy-ingress-pxe-boots"
CHART_VERSION="${CHART_VERSION:-1.0.1}"   # Chart version variable

# Full OCI chart path
CHART="oci://${RELEASE_SERVICE_URL}/${CHART_NAME}:${CHART_VERSION}"

# Substitute environment variables in values.yaml
envsubst < $VALUES_TEMPLATE > $VALUES_FILE

# Ensure namespace exists
kubectl get ns $NAMESPACE &>/dev/null || kubectl create ns $NAMESPACE

# Function: Install or upgrade Helm release
install_ha_proxy() {
    echo "🔹 Installing/upgrading HAProxy Ingress..."
    helm upgrade --install $RELEASE_NAME $CHART \
        --namespace $NAMESPACE \
        -f $VALUES_FILE \
        --atomic \
        --wait
    echo "✅ HAProxy Ingress installed/upgraded successfully in namespace $NAMESPACE"
    echo "Chart: $CHART"
}

# Function: Uninstall Helm release and cleanup TLS resources
uninstall_ha_proxy() {
    echo "🔹 Uninstalling HAProxy Ingress..."
    helm uninstall $RELEASE_NAME --namespace $NAMESPACE || echo "Release not found, skipping..."
    
    echo "🔹 Cleaning up TLS Certificate and Secret..."
    kubectl delete certificate tls-boots -n $NAMESPACE --ignore-not-found
    kubectl delete secret tls-boots -n $NAMESPACE --ignore-not-found
    
    echo "✅ HAProxy Ingress and associated TLS resources removed from namespace $NAMESPACE"
}

# Main logic
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
