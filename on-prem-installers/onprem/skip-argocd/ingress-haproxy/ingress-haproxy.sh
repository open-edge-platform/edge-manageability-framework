#!/bin/bash
# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -e

ACTION="${1:-install}"   # install | uninstall | verify

APP_NAME="ingress-haproxy"
NAMESPACE="orch-boots"

REPO_NAME="haproxytech"
REPO_URL="https://haproxytech.github.io/helm-charts"

CHART_NAME="kubernetes-ingress"
CHART_VERSION="1.41.0"
VALUES_FILE="${2:-values-loadbalancer.yaml}"


# --------------------------
# Pre-checks
# --------------------------
command -v helm >/dev/null 2>&1 || { echo "❌ Helm not installed"; exit 1; }
command -v kubectl >/dev/null 2>&1 || { echo "❌ kubectl not installed"; exit 1; }

kubectl cluster-info >/dev/null 2>&1 || { echo "❌ Kubernetes cluster unreachable"; exit 1; }

# --------------------------
# Namespace
# --------------------------
create_namespace() {
    echo "🔧 Ensuring namespace exists..."
    kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -
}

# --------------------------
# Verify Deployment
# --------------------------
verify_deployment() {
    echo "🔍 Verifying deployment..."

    echo "📊 Helm Release Status:"
    helm status "$APP_NAME" -n "$NAMESPACE" || echo "Release not found"

    echo -e "\n📦 Pods:"
    kubectl get pods -n "$NAMESPACE" -o wide || true

    echo -e "\n📦 Services:"
    kubectl get svc -n "$NAMESPACE" || true

    echo -e "\n📋 Recent Events:"
    kubectl get events -n "$NAMESPACE" --sort-by='.lastTimestamp' | tail -10 || true
}

# --------------------------
# Install / Uninstall
# --------------------------
case "$ACTION" in
  install)
    create_namespace

    echo "📦 Adding Helm repo..."
    helm repo add "$REPO_NAME" "$REPO_URL" 2>/dev/null || true
    helm repo update

    echo "🚀 Installing ${APP_NAME}..."

    helm upgrade --install "$APP_NAME" "$REPO_NAME/$CHART_NAME" \
      --version "$CHART_VERSION" \
      -n "$NAMESPACE" \
      --create-namespace \
      -f "$VALUES_FILE" \
      --wait

    echo "✅ Deployment successful"
    verify_deployment
    ;;

  uninstall)
    echo "🗑 Uninstalling ${APP_NAME}..."
    helm uninstall "$APP_NAME" -n "$NAMESPACE" || true
    echo "✅ Uninstallation completed"
    ;;

  verify)
    verify_deployment
    ;;

  *)
    echo "Usage: $0 [install|uninstall|verify]"
    exit 1
    ;;
esac
