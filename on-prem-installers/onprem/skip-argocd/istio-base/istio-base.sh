#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

APP_NAME="istio-base"
NAMESPACE="istio-system"
CHART_REPO="https://istio-release.storage.googleapis.com/charts"
CHART_NAME="istio/base"
VERSION="1.29.0"

VALUES_FILE="${VALUES_FILE:-istio-base-values.yaml}"
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
# Wait (safe for CRDs)
# -----------------------------
wait_for_resources() {
  echo "⏳ Checking resources in ${NAMESPACE}..."

  POD_COUNT=$(kubectl get pods -n "${NAMESPACE}" --no-headers 2>/dev/null | wc -l || echo 0)

  if [[ "${POD_COUNT}" -eq 0 ]]; then
    echo "ℹ️ No pods created (expected for istio-base CRDs), skipping wait."
    return 0
  fi

  echo "⏳ Waiting for pods to be Ready (timeout: ${TIMEOUT})..."

  kubectl wait \
    --namespace "${NAMESPACE}" \
    --for=condition=Ready pod \
    --all \
    --timeout="${TIMEOUT}" || {
      echo "❌ Timeout waiting for pods"
      kubectl get pods -n "${NAMESPACE}"
      kubectl describe pods -n "${NAMESPACE}" || true
      exit 1
    }

  echo "✅ Resources are ready!"
}

# -----------------------------
# Install
# -----------------------------
install() {
  echo "🚀 Installing ${APP_NAME}..."

  check_deps

  echo "📦 Adding Helm repo..."
  helm repo add istio "${CHART_REPO}" >/dev/null 2>&1 || true
  helm repo update >/dev/null

  echo "📦 Ensuring namespace exists..."
  kubectl get ns "${NAMESPACE}" >/dev/null 2>&1 || kubectl create ns "${NAMESPACE}"

  echo "📥 Installing Helm chart..."
  helm upgrade --install "${APP_NAME}" "${CHART_NAME}" \
    --version "${VERSION}" \
    --namespace "${NAMESPACE}" \
    -f "${VALUES_FILE}" \
    --create-namespace

  wait_for_resources

  echo "🔍 Installed CRDs:"
  kubectl get crds | grep istio || true

  echo "✅ ${APP_NAME} installed successfully!"
}

# -----------------------------
# Uninstall
# -----------------------------
uninstall() {
  echo "🗑️ Uninstalling ${APP_NAME}..."

  check_deps

  helm uninstall "${APP_NAME}" -n "${NAMESPACE}" || true

  echo "⚠️ CRDs are NOT removed automatically (expected for istio-base)"
  echo "👉 Manual cleanup if needed:"
  echo "kubectl get crds | grep istio"

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
