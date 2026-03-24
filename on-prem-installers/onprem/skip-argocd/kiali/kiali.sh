#!/bin/bash
set -euo pipefail

# Configuration
NAMESPACE="istio-system"
CHART_REPO="https://kiali.org/helm-charts"
CHART_NAME="kiali-server"
CHART_VERSION="2.22.0"
RELEASE_NAME="kiali"
VALUES_FILE="./values.yaml"
WAIT_TIMEOUT="300s"

create_namespace() {
  kubectl get namespace "$NAMESPACE" >/dev/null 2>&1 || {
    echo "📦 Creating namespace: $NAMESPACE"
    kubectl create namespace "$NAMESPACE"
  }
}

wait_for_kiali() {
  echo "⏳ Waiting for Kiali deployment..."

  # Wait until deployment appears
  for i in {1..30}; do
    kubectl get deployment kiali -n "$NAMESPACE" >/dev/null 2>&1 && break
    sleep 5
  done

  echo "⏳ Waiting for Kiali pods to be ready..."

  kubectl rollout status deployment/kiali \
    -n "$NAMESPACE" \
    --timeout="$WAIT_TIMEOUT"

  echo "✅ Kiali is ready"
}

deploy_kiali() {
  echo "🚀 Deploying Kiali..."

  create_namespace

  helm repo add kiali "$CHART_REPO" >/dev/null 2>&1 || true
  helm repo update >/dev/null 2>&1

  helm upgrade --install "$RELEASE_NAME" kiali/$CHART_NAME \
    --version "$CHART_VERSION" \
    --namespace "$NAMESPACE" \
    -f "$VALUES_FILE" \
    --wait --timeout 10m

  wait_for_kiali

  echo "✅ Kiali deployed successfully"
}

uninstall_kiali() {
  echo "🗑️ Uninstalling Kiali..."

  if helm status "$RELEASE_NAME" -n "$NAMESPACE" >/dev/null 2>&1; then
    helm uninstall "$RELEASE_NAME" -n "$NAMESPACE"
    echo "✅ Kiali uninstalled"
  else
    echo "ℹ️ Kiali not installed"
  fi
}

case "${1:-}" in
  install)
    deploy_kiali
    ;;
  uninstall)
    uninstall_kiali
    ;;
  *)
    echo "Usage: $0 install|uninstall"
    exit 1
    ;;
esac
