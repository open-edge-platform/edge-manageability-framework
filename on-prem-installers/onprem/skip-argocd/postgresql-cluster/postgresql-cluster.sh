#!/bin/bash
set -euo pipefail

# Configuration
NAMESPACE="orch-database"
CHART_REPO="https://cloudnative-pg.github.io/charts"
CHART_NAME="cluster"
RELEASE_NAME="postgresql-cluster"
VALUES_FILE="./values.yaml"
WAIT_TIMEOUT="600s"

create_namespace() {
  kubectl get namespace "$NAMESPACE" >/dev/null 2>&1 || \
    kubectl create namespace "$NAMESPACE"
}

wait_for_pods_ready() {
  echo "⏳ Waiting for PostgreSQL pods to be created..."

  # Wait until pods exist
  for i in {1..30}; do
    POD_COUNT=$(kubectl get pods -n "$NAMESPACE" --no-headers 2>/dev/null | wc -l)
    if [ "$POD_COUNT" -gt 0 ]; then
      echo "✅ Pods detected"
      break
    fi
    sleep 5
  done

  if [ "$POD_COUNT" -eq 0 ]; then
    echo "❌ No pods found in namespace $NAMESPACE"
    exit 1
  fi

  echo "⏳ Waiting for all pods to become Ready..."

  kubectl wait --namespace "$NAMESPACE" \
    --for=condition=Ready pod \
    --all \
    --timeout="$WAIT_TIMEOUT"

  echo "✅ All PostgreSQL pods are ready"
}

deploy_postgresql() {
  echo "🚀 Deploying PostgreSQL Helm chart..."

  create_namespace

  helm repo add cpg "$CHART_REPO" >/dev/null 2>&1 || true
  helm repo update >/dev/null 2>&1

  helm upgrade --install "$RELEASE_NAME" cpg/$CHART_NAME \
    --namespace "$NAMESPACE" \
    -f "$VALUES_FILE" \
    --wait --timeout 10m

  wait_for_pods_ready

  echo "✅ PostgreSQL deployed successfully"
}

uninstall_postgresql() {
  echo "🗑️ Uninstalling PostgreSQL Helm release..."

  if helm status "$RELEASE_NAME" -n "$NAMESPACE" >/dev/null 2>&1; then
    helm uninstall "$RELEASE_NAME" -n "$NAMESPACE"
    echo "✅ PostgreSQL uninstalled"
  else
    echo "ℹ️ PostgreSQL not installed"
  fi
}

case "${1:-}" in
  install)
    deploy_postgresql
    ;;
  uninstall)
    uninstall_postgresql
    ;;
  *)
    echo "Usage: $0 install|uninstall"
    exit 1
    ;;
esac
