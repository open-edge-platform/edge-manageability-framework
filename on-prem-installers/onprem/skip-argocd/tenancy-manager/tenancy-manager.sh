#!/bin/bash
# tenancy-manager.sh
# Install/Upgrade or Uninstall tenancy-manager via Helm (OCI)
# Default registry: registry-rs.edgeorchestration.intel.com

set -e

usage() {
    echo "Usage: $0 [install|uninstall]"
    exit 1
}

# Load environment variables
source ../onprem.env

# Variables
RELEASE_NAME="tenancy-manager"
NAMESPACE="orch-iam"
VALUES_TEMPLATE="./values.yaml"
VALUES_FILE="./values-rendered.yaml"

# Default OCI registry
RELEASE_SERVICE_URL="${RELEASE_SERVICE_URL:-registry-rs.edgeorchestration.intel.com}"

CHART_NAME="edge-orch/common/charts/tenancy-manager"
CHART_VERSION="${CHART_VERSION:-26.0.3}"

# Full OCI chart path
CHART="oci://${RELEASE_SERVICE_URL}/${CHART_NAME}:${CHART_VERSION}"

# Render values.yaml with env variables
envsubst < $VALUES_TEMPLATE > $VALUES_FILE

# Ensure namespace exists (only for install path ideally, but safe here)
kubectl get ns $NAMESPACE &>/dev/null || kubectl create ns $NAMESPACE

# Function: Install or upgrade
install_tenancy_manager() {
    echo "🔹 Installing/upgrading tenancy-manager..."
    helm upgrade --install $RELEASE_NAME $CHART \
        --namespace $NAMESPACE \
        -f $VALUES_FILE \
        --atomic \
        --wait

    echo "✅ tenancy-manager installed/upgraded in namespace $NAMESPACE"
    echo "Chart: $CHART"
}

# Function: Uninstall (safe, no errors)
uninstall_tenancy_manager() {
    echo "🔹 Uninstalling tenancy-manager..."

    if helm status $RELEASE_NAME -n $NAMESPACE &>/dev/null; then
        helm uninstall $RELEASE_NAME --namespace $NAMESPACE
        echo "✅ tenancy-manager uninstalled from namespace $NAMESPACE"
    else
        echo "ℹ️ Release not found, nothing to uninstall"
    fi
}

# Main
if [[ $# -ne 1 ]]; then
    usage
fi

ACTION="$1"

case "$ACTION" in
    install)
        install_tenancy_manager
        ;;
    uninstall)
        uninstall_tenancy_manager
        ;;
    *)
        usage
        ;;
esac
