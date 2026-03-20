#!/bin/bash
set -euo pipefail

NAMESPACE="orch-platform"
CHART_REPO="https://stakater.github.io/stakater-charts"
CHART_NAME="reloader"
RELEASE_NAME="reloader"
VALUES_FILE="./values-reloader.yaml"
WAIT_TIMEOUT="180s"  # wait up to 3 minutes for pod ready

create_namespace() {
  kubectl get namespace "$NAMESPACE" >/dev/null 2>&1 || \
    kubectl create namespace "$NAMESPACE"
}

wait_for_pods_ready() {
  echo "⏳ Waiting for Reloader pods to be ready..."

  # Wait until at least 1 pod exists
  while [ $(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=reloader --no-headers 2>/dev/null | wc -l) -eq 0 ]; do
    echo "Waiting for Reloader pod to appear..."
    sleep 3
  done

  # Wait until pods are Ready
  kubectl wait --namespace "$NAMESPACE" \
    --for=condition=Ready pod \
    --selector=app.kubernetes.io/name=reloader \
    --timeout="$WAIT_TIMEOUT"

  echo "✅ All Reloader pods are ready"
}

deploy_reloader() {
  echo "🚀 Deploying Reloader..."

  create_namespace

  helm repo add stakater $CHART_REPO >/dev/null 2>&1 || true
  helm repo update >/dev/null 2>&1

  helm upgrade --install "$RELEASE_NAME" stakater/$CHART_NAME \
    --namespace "$NAMESPACE" \
    --version "$(yq e '.reloader.version' $VALUES_FILE)" \
    -f "$VALUES_FILE" \
    --wait --timeout 10m

  wait_for_pods_ready

  echo "✅ Reloader deployed successfully"
}

uninstall_reloader() {
  echo "🗑️ Uninstalling Reloader..."

  if helm status "$RELEASE_NAME" -n "$NAMESPACE" >/dev/null 2>&1; then
    helm uninstall "$RELEASE_NAME" -n "$NAMESPACE"
    echo "✅ Reloader uninstalled"
  else
    echo "ℹ️ Reloader not installed"
  fi
}

case "${1:-}" in
  install)
    deploy_reloader
    ;;
  uninstall)
    uninstall_reloader
    ;;
  *)
    echo "Usage: $0 install|uninstall"
    exit 1
    ;;
esac
