#!/bin/bash
# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -e

ACTION="${1:-install}"
VALUES_FILE="${2:-values-loadbalancer.yaml}"

APP_NAME="traefik"
NAMESPACE="orch-gateway"

REPO_NAME="traefik"
REPO_URL="https://helm.traefik.io/traefik"

CHART_NAME="traefik"
CHART_VERSION="37.2.0"

JWT_PLUGIN_DIR="./traefik-jwt-plugin"

# --------------------------
# Pre-checks
# --------------------------
command -v helm >/dev/null 2>&1 || { echo "❌ Helm not installed"; exit 1; }
command -v kubectl >/dev/null 2>&1 || { echo "❌ kubectl not installed"; exit 1; }

kubectl cluster-info >/dev/null 2>&1 || { echo "❌ Kubernetes cluster unreachable"; exit 1; }

if [ ! -f "$VALUES_FILE" ]; then
  echo "❌ Values file not found: $VALUES_FILE"
  exit 1
fi

# --------------------------
# Namespace
# --------------------------
create_namespace() {
    echo "🔧 Ensuring namespace exists..."
    kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -
}

# --------------------------
# Create JWT Plugin ConfigMap
# --------------------------
create_jwt_configmap() {
    echo "🔧 Checking jwt-plugin ConfigMap..."

    if kubectl get configmap jwt-plugin -n "$NAMESPACE" >/dev/null 2>&1; then
        echo "✅ ConfigMap jwt-plugin already exists"
    else
        if [ -d "$JWT_PLUGIN_DIR" ]; then
            echo "📦 Creating ConfigMap from $JWT_PLUGIN_DIR"
            kubectl create configmap jwt-plugin \
              -n "$NAMESPACE" \
              --from-file="$JWT_PLUGIN_DIR"
            echo "✅ ConfigMap created"
        else
            echo "⚠️  Directory $JWT_PLUGIN_DIR not found"
            echo "👉 Creating empty ConfigMap (plugin may not work)"
            kubectl create configmap jwt-plugin -n "$NAMESPACE"
        fi
    fi
}

# --------------------------
# Verify
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
    create_jwt_configmap

    echo "📦 Adding Helm repo..."
    helm repo add "$REPO_NAME" "$REPO_URL" 2>/dev/null || true
    helm repo update

    echo "🚀 Installing ${APP_NAME} using ${VALUES_FILE}..."

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

    echo "🧹 Deleting jwt-plugin ConfigMap..."
    kubectl delete configmap jwt-plugin -n "$NAMESPACE" --ignore-not-found

    echo "✅ Uninstallation completed"
    ;;

  verify)
    verify_deployment
    ;;

  *)
    echo "Usage:"
    echo "  $0 install [values-file]"
    echo "  $0 uninstall"
    echo "  $0 verify"
    exit 1
    ;;
esac
