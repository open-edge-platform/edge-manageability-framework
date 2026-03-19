#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

APP_NAME="k8s-metrics-server"
NAMESPACE="kube-system"
CHART="oci://registry-rs.edgeorchestration.intel.com/edge-orch/common/charts/k8s-metrics-server"
VERSION="25.2.1"

VALUES_FILE="${VALUES_FILE:-k8s-metrics-server-values.yaml}"
TIMEOUT="300s"

# -----------------------------
# Check dependencies
# -----------------------------
check_deps() {
  for cmd in helm kubectl; do
    command -v $cmd >/dev/null || {
      echo "❌ $cmd not installed"
      exit 1
    }
  done
}

# -----------------------------
# Wait for pods
# -----------------------------
wait_for_pods() {
  echo "⏳ Waiting for metrics-server pods..."

  if ! kubectl wait \
    --namespace "${NAMESPACE}" \
    --for=condition=Ready pod \
    -l app.kubernetes.io/name=metrics-server \
    --timeout="${TIMEOUT}"; then

    echo "❌ Pods not Ready within ${TIMEOUT}"

    kubectl get pods -n "${NAMESPACE}"
    kubectl describe pods -n "${NAMESPACE}" || true

    exit 1
  fi

  echo "✅ Metrics-server is Ready!"
}

# -----------------------------
# Install
# -----------------------------
install() {
  echo "🚀 Installing ${APP_NAME}..."

  check_deps

  echo "📦 Deploying Helm chart..."
  helm upgrade --install "${APP_NAME}" "${CHART}" \
    --version "${VERSION}" \
    --namespace "${NAMESPACE}" \
    -f "${VALUES_FILE}"

  wait_for_pods

  echo "🔍 Final status:"
  kubectl get pods -n "${NAMESPACE}" | grep metrics || true

  echo "✅ ${APP_NAME} installed successfully!"
}

# -----------------------------
# Uninstall
# -----------------------------
uninstall() {
  echo "🗑️ Uninstalling ${APP_NAME}..."

  check_deps

  helm uninstall "${APP_NAME}" -n "${NAMESPACE}" || true

  echo "✅ ${APP_NAME} uninstalled!"
}

# -----------------------------
# Entry
# -----------------------------
case "${1:-}" in
  install)
    install
    ;;
  uninstall)
    uninstall
    ;;
  *)
    echo "Usage: $0 {install|uninstall}"
    exit 1
    ;;
esac
