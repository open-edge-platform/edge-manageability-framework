#!/bin/bash
set -euo pipefail

# Configuration
NAMESPACE="istio-system"
CHART_OCI="oci://registry-rs.edgeorchestration.intel.com/edge-orch/common/charts/istio-policy"
CHART_VERSION="26.0.0"
RELEASE_NAME="istio-policy"
VALUES_FILE="./values.yaml"
WAIT_TIMEOUT="300s"

# Namespaces required by the chart
REQUIRED_NAMESPACES=(
  "orch-cluster"
  "orch-ui"
  "orch-app"
  "orch-sre"
  "orch-harbor"
)

create_namespace() {
  kubectl get namespace "$NAMESPACE" >/dev/null 2>&1 || {
    echo "📦 Creating namespace: $NAMESPACE"
    kubectl create namespace "$NAMESPACE"
  }
}

create_required_namespaces() {
  for ns in "${REQUIRED_NAMESPACES[@]}"; do
    kubectl get namespace "$ns" >/dev/null 2>&1 || {
      echo "📦 Creating namespace: $ns"
      kubectl create namespace "$ns"
    }
  done
}

wait_for_resources() {
  echo "⏳ Waiting for resources to be created..."

  # Wait until resources appear
  for i in {1..30}; do
    COUNT=$(kubectl get all -n "$NAMESPACE" --no-headers 2>/dev/null | wc -l)
    if [ "$COUNT" -gt 0 ]; then
      echo "✅ Resources detected"
      break
    fi
    sleep 5
  done

  echo "⏳ Waiting for pods (if any) to be ready..."

  kubectl wait --namespace "$NAMESPACE" \
    --for=condition=Ready pod \
    --all \
    --timeout="$WAIT_TIMEOUT" 2>/dev/null || true

  echo "✅ Istio policy deployment completed"
}

deploy_istio_policy() {
  echo "🚀 Deploying Istio Policy (OCI)..."

  create_namespace
  create_required_namespaces

  # Enable OCI support
  export HELM_EXPERIMENTAL_OCI=1

  # Optional: login if registry requires authentication
  # helm registry login registry-rs.edgeorchestration.intel.com

  helm upgrade --install "$RELEASE_NAME" "$CHART_OCI" \
    --version "$CHART_VERSION" \
    --namespace "$NAMESPACE" \
    -f "$VALUES_FILE" \
    --wait --timeout 10m

  wait_for_resources

  echo "✅ Istio Policy deployed successfully"
}

uninstall_istio_policy() {
  echo "🗑️ Uninstalling Istio Policy..."

  if helm status "$RELEASE_NAME" -n "$NAMESPACE" >/dev/null 2>&1; then
    helm uninstall "$RELEASE_NAME" -n "$NAMESPACE"
    echo "✅ Istio Policy uninstalled"
  else
    echo "ℹ️ Istio Policy not installed"
  fi
}

case "${1:-}" in
  install)
    deploy_istio_policy
    ;;
  uninstall)
    uninstall_istio_policy
    ;;
  *)
    echo "Usage: $0 install|uninstall"
    exit 1
    ;;
esac
