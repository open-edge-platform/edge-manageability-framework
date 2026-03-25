#!/bin/bash
# nexus-api-gw.sh
# Usage: ./nexus-api-gw.sh install|uninstall

set -euo pipefail

# ----------------------------
# Source environment variables
# ----------------------------
ENV_FILE="../onprem.env"
if [[ -f "$ENV_FILE" ]]; then
  echo "Sourcing environment variables from $ENV_FILE"
  source "$ENV_FILE"
else
  echo "❌ Environment file $ENV_FILE not found!"
  exit 1
fi

ACTION="${1:-}"

# ----------------------------
# Hardcoded Helm chart params
# ----------------------------
CHART="oci://${RELEASE_SERVICE_URL}/edge-orch/common/charts/nexus-api-gw"
CHART_VERSION="26.0.7"
NAMESPACE="orch-iam"
RELEASE_NAME="nexus-api-gw"
VALUES_FILE="values.yaml"

if [[ -z "$ACTION" ]]; then
  echo "❌ Usage: $0 install|uninstall"
  exit 1
fi

# ----------------------------
# Actions
# ----------------------------

case "$ACTION" in
  install)
    echo "🚀 Installing Helm chart $RELEASE_NAME version $CHART_VERSION in namespace $NAMESPACE..."

    # Ensure namespace exists
    kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

    # Install / upgrade
    helm upgrade --install "$RELEASE_NAME" "$CHART" \
      -n "$NAMESPACE" \
      --version "$CHART_VERSION" \
      -f "$VALUES_FILE" \
      --wait

    echo "✅ Helm chart installed successfully."
    ;;

  uninstall)
    echo "🧹 Uninstalling Helm chart $RELEASE_NAME from namespace $NAMESPACE..."

    # Safe uninstall (no error if not present)
    if helm uninstall "$RELEASE_NAME" -n "$NAMESPACE" 2>/dev/null; then
      echo "✅ Helm chart uninstalled successfully."
    else
      echo "⚠️ Release $RELEASE_NAME not found or already removed, skipping"
    fi

    # Always succeed so run-all.sh continues
    true
    ;;

  *)
    echo "❌ Unknown action: $ACTION. Use install|uninstall"
    exit 1
    ;;
esac
