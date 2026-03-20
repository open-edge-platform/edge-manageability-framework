#!/bin/bash
set -euo pipefail

NAMESPACE="istio-system"
CHART_REPO="https://istio-release.storage.googleapis.com/charts"
CHART_NAME="istiod"
RELEASE_NAME="istiod"
VALUES_FILE="./istiod-values.yaml"
WAIT_TIMEOUT="300s"  # Wait up to 5 minutes for pods to be ready

create_namespace() {
  kubectl get namespace "$NAMESPACE" >/dev/null 2>&1 || \
    kubectl create namespace "$NAMESPACE"
}

wait_for_pods_ready() {
  echo "⏳ Waiting for Istiod pods to be ready..."
  kubectl wait --namespace "$NAMESPACE" \
    --for=condition=Ready pod \
    --selector=app=istiod \
    --timeout="$WAIT_TIMEOUT"
  echo "✅ All Istiod pods are ready"
}

deploy_istiod() {
  echo "🚀 Deploying Istiod..."

  create_namespace

  helm repo add istio $CHART_REPO >/dev/null 2>&1 || true
  helm repo update >/dev/null 2>&1

  helm upgrade --install "$RELEASE_NAME" istio/$CHART_NAME \
    --namespace "$NAMESPACE" \
    --version "$(yq e '.istiod.version' $VALUES_FILE)" \
    -f "$VALUES_FILE" \
    --wait --timeout 10m

  # Wait for Istiod pods to be ready
  wait_for_pods_ready

  echo "✅ Istiod deployed successfully"
}

uninstall_istiod() {
  echo "🗑️ Uninstalling Istiod..."

  if helm status "$RELEASE_NAME" -n "$NAMESPACE" >/dev/null 2>&1; then
    helm uninstall "$RELEASE_NAME" -n "$NAMESPACE"
    echo "✅ Istiod uninstalled"
  else
    echo "ℹ️ Istiod not installed"
  fi
}

case "${1:-}" in
  install)
    deploy_istiod
    ;;
  uninstall)
    uninstall_istiod
    ;;
  *)
    echo "Usage: $0 install|uninstall"
    exit 1
    ;;
esac
