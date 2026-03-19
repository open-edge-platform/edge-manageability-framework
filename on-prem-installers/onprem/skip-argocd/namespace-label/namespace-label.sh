#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

APP_NAME="namespace-label"
NAMESPACE="ns-label"
CHART="oci://registry-rs.edgeorchestration.intel.com/edge-orch/common/charts/namespace-label"
VERSION="25.2.3"

VALUES_FILE="${VALUES_FILE:-./namespace-label-values.yaml}"
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
# Wait for pods (safe mode)
# -----------------------------
wait_for_pods() {
  echo "⏳ Checking for pods in namespace ${NAMESPACE}..."

  POD_COUNT=$(kubectl get pods -n "${NAMESPACE}" --no-headers 2>/dev/null | wc -l || echo 0)

  if [[ "${POD_COUNT}" -eq 0 ]]; then
    echo "ℹ️ No pods found in ${NAMESPACE}, skipping wait."
    return 0
  fi

  echo "⏳ Waiting for pods to be Ready (timeout: ${TIMEOUT})..."

  if ! kubectl wait \
    --namespace "${NAMESPACE}" \
    --for=condition=Ready pod \
    --all \
    --timeout="${TIMEOUT}"; then

    echo "❌ Pods not Ready within ${TIMEOUT}"

    echo "📊 Pod status:"
    kubectl get pods -n "${NAMESPACE}"

    echo "📄 Describe pods:"
    kubectl describe pods -n "${NAMESPACE}" || true

    exit 1
  fi

  echo "✅ Pods are Ready!"
}

# -----------------------------
# Install
# -----------------------------
install() {
  echo "🚀 Installing ${APP_NAME} via Helm..."

  check_deps

  echo "📦 Ensuring namespace exists: ${NAMESPACE}"
  kubectl get ns "${NAMESPACE}" >/dev/null 2>&1 || kubectl create ns "${NAMESPACE}"

  echo "📥 Deploying Helm chart..."
  helm upgrade --install "${APP_NAME}" "${CHART}" \
    --version "${VERSION}" \
    --namespace "${NAMESPACE}" \
    -f "${VALUES_FILE}" \
    --create-namespace

  wait_for_pods

  echo "🔍 Final resources:"
  kubectl get all -n "${NAMESPACE}" || true

  echo "✅ ${APP_NAME} installed successfully!"
}

# -----------------------------
# Uninstall
# -----------------------------
uninstall() {
  echo "🗑️ Uninstalling ${APP_NAME}..."

  check_deps

  helm uninstall "${APP_NAME}" -n "${NAMESPACE}" || true

  echo "🧹 (Optional) Cleanup namespaces created by chart"
  echo "⚠️ Skipping namespace deletion for safety"

  echo "✅ ${APP_NAME} uninstalled!"
}

# -----------------------------
# Entry point
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
