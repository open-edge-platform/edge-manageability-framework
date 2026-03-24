#!/bin/bash
# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -e

ACTION="${1:-install}"
VALUES_FILE="values.yaml"
ENV_FILE="../onprem.env"

APP_NAME="botkube"
NAMESPACE="orch-gateway"

CHART_VERSION="v1.11.0"
TMP_VALUES="/tmp/botkube-values.yaml"

# --------------------------
# Load env
# --------------------------
load_env() {
  if [ -f "$ENV_FILE" ]; then
    echo "📦 Loading env from $ENV_FILE"
    source "$ENV_FILE"
  else
    echo "❌ Env file not found: $ENV_FILE"
    exit 1
  fi

  if [ -z "$RELEASE_SERVICE_URL" ]; then
    echo "❌ RELEASE_SERVICE_URL not set"
    exit 1
  fi

  CHART="oci://${RELEASE_SERVICE_URL}/edge-orch/common/charts/botkube"
  echo "🌐 Using chart: $CHART"
}

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
# Build values with proxy
# --------------------------
build_values() {
  echo "🔧 Building values with proxy settings..."
  cp "$VALUES_FILE" "$TMP_VALUES"

  # Remove existing extraEnv safely
  awk '
    BEGIN {skip=0}
    /^extraEnv:/ {skip=1; next}
    /^[^[:space:]]/ {skip=0}
    skip==0 {print}
  ' "$VALUES_FILE" > "$TMP_VALUES"

  if [ -n "$HTTP_PROXY" ] || [ -n "$HTTPS_PROXY" ] || [ -n "$NO_PROXY" ]; then
    cat <<EOF >> "$TMP_VALUES"

extraEnv:
EOF

    [ -n "$HTTP_PROXY" ] && cat <<EOF >> "$TMP_VALUES"
  - name: HTTP_PROXY
    value: "$HTTP_PROXY"
  - name: http_proxy
    value: "$HTTP_PROXY"
EOF

    [ -n "$HTTPS_PROXY" ] && cat <<EOF >> "$TMP_VALUES"
  - name: HTTPS_PROXY
    value: "$HTTPS_PROXY"
  - name: https_proxy
    value: "$HTTPS_PROXY"
EOF

    [ -n "$NO_PROXY" ] && cat <<EOF >> "$TMP_VALUES"
  - name: NO_PROXY
    value: "$NO_PROXY"
  - name: no_proxy
    value: "$NO_PROXY"
EOF
  fi
}

# --------------------------
# Verify
# --------------------------
verify() {
  echo "🔍 Verifying deployment..."

  echo "📊 Helm Status:"
  helm status "$APP_NAME" -n "$NAMESPACE" || true

  echo -e "\n📦 Pods:"
  kubectl get pods -n "$NAMESPACE" -o wide || true

  echo -e "\n📋 Events:"
  kubectl get events -n "$NAMESPACE" --sort-by='.lastTimestamp' | tail -10 || true
}

# --------------------------
# Main
# --------------------------
case "$ACTION" in
  install)
    load_env
    create_namespace
    build_values

    echo "🚀 Installing ${APP_NAME}..."

    helm upgrade --install "$APP_NAME" "$CHART" \
      --version "$CHART_VERSION" \
      -n "$NAMESPACE" \
      --create-namespace \
      -f "$TMP_VALUES" \
      --wait

    echo "✅ Installation completed"
    verify
    ;;

  uninstall)
    echo "🗑 Uninstalling ${APP_NAME}..."
    helm uninstall "$APP_NAME" -n "$NAMESPACE" || true
    echo "✅ Uninstallation completed"
    ;;

  verify)
    verify
    ;;

  *)
    echo "Usage: $0 [install|uninstall|verify]"
    exit 1
    ;;
esac
