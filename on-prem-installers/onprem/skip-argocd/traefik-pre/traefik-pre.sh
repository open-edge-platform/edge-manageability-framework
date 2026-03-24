#!/bin/bash
# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -e
source ../onprem.env

ACTION="${1:-install}"     # install | uninstall | verify

NAMESPACE="orch-gateway"
RELEASE_NAME="traefik-pre"
CHART="oci://${RELEASE_SERVICE_URL}/edge-orch/common/charts/traefik-pre"
CHART_VERSION="3.0.1"
VALUES_FILE="values.yaml"

# --------------------------
# Pre-checks
# --------------------------
command -v helm >/dev/null 2>&1 || { echo "❌ Helm not installed"; exit 1; }
command -v kubectl >/dev/null 2>&1 || { echo "❌ kubectl not installed"; exit 1; }

kubectl cluster-info >/dev/null 2>&1 || { echo "❌ Kubernetes cluster unreachable"; exit 1; }

# --------------------------
# Namespace Setup
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
    helm status "$RELEASE_NAME" -n "$NAMESPACE" || echo "Release not found"

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

    echo "🚀 Installing ${RELEASE_NAME}..."

    # Optional: login if OCI private
    # helm registry login $RELEASE_SERVICE_URL -u <user> -p <password>

    helm upgrade --install "$RELEASE_NAME" "$CHART" \
      --version "$CHART_VERSION" \
      -n "$NAMESPACE" \
      --create-namespace \
      -f "$VALUES_FILE" \
      --wait

    echo "✅ Deployment successful"
    verify_deployment
    ;;

  uninstall)
    echo "🗑 Uninstalling ${RELEASE_NAME}..."
    helm uninstall "$RELEASE_NAME" -n "$NAMESPACE" || true
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
