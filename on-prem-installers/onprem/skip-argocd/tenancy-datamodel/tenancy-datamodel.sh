#!/bin/bash
# tenancy-datamodel.sh
# Usage: ./tenancy-datamodel.sh install|uninstall

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
# Hardcoded Helm chart config
# ----------------------------
CHART="oci://${RELEASE_SERVICE_URL}/edge-orch/common/charts/tenancy-datamodel"
CHART_VERSION="26.0.4"
NAMESPACE="orch-iam"
RELEASE_NAME="tenancy-datamodel"
VALUES_FILE="values.yaml"

if [[ -z "$ACTION" ]]; then
  echo "❌ Usage: $0 install|uninstall"
  exit 1
fi

case "$ACTION" in
  install)
    echo "🚀 Installing $RELEASE_NAME version $CHART_VERSION in namespace $NAMESPACE..."

    # Ensure namespace exists
    kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

    # Install / upgrade Helm chart
    helm upgrade --install "$RELEASE_NAME" "$CHART" \
      -n "$NAMESPACE" \
      --version "$CHART_VERSION" \
      -f "$VALUES_FILE" \
      --wait

    echo "✅ Helm chart installed successfully."
    ;;

  uninstall)
    echo "🧹 Uninstalling $RELEASE_NAME from namespace $NAMESPACE..."

    # Safe uninstall (never fail)
    helm uninstall "$RELEASE_NAME" -n "$NAMESPACE" 2>/dev/null || \
      echo "⚠️ Release $RELEASE_NAME not found or already removed, skipping"

    # Always succeed so run-all.sh continues
    true
    ;;

  *)
    echo "❌ Unknown action: $ACTION. Use install|uninstall"
    exit 1
    ;;
esac
