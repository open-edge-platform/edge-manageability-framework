#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

APP_NAME="keycloak-operator"
NAMESPACE="orch-platform"
CHART="oci://registry-rs.edgeorchestration.intel.com/edge-orch/common/charts/keycloak-operator"
VERSION="26.1.2"

VALUES_FILE="${VALUES_FILE:-values.yaml}"
TIMEOUT="300s"   # 5 minutes

# -----------------------------
# Common: check dependencies
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
# Wait for pods Ready
# -----------------------------
wait_for_pods() {
  echo "⏳ Waiting for pods to be Ready (timeout: ${TIMEOUT})..."

  if ! kubectl wait \
    --namespace "${NAMESPACE}" \
    --for=condition=Ready pod \
    -l app.kubernetes.io/name=${APP_NAME} \
    --timeout="${TIMEOUT}"; then

    echo "❌ Pods not Ready within ${TIMEOUT}"
    echo "📊 Current pod status:"
    kubectl get pods -n "${NAMESPACE}"

    echo "📄 Describe pods:"
    kubectl describe pods -n "${NAMESPACE}" -l app.kubernetes.io/name=${APP_NAME} || true

    exit 1
  fi

  echo "✅ Pods are Ready!"
}

# -----------------------------
# Install function
# -----------------------------
install() {
  echo "🚀 Installing ${APP_NAME}..."

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

  echo "🔍 Final pod status:"
  kubectl get pods -n "${NAMESPACE}"

  echo "✅ ${APP_NAME} installed successfully!"
}

# -----------------------------
# Uninstall function
# -----------------------------
uninstall() {
  echo "🗑️ Uninstalling ${APP_NAME}..."

  check_deps

  helm uninstall "${APP_NAME}" -n "${NAMESPACE}" || true

  echo "🧹 Cleaning namespace (optional)..."
  # kubectl delete ns "${NAMESPACE}" --ignore-not-found

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
