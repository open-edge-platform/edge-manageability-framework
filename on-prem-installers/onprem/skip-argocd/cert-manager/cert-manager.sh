#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

APP_NAME="cert-manager"
NAMESPACE="cert-manager"
CHART_REPO="https://charts.jetstack.io"
CHART_NAME="jetstack/cert-manager"
VERSION="v1.19.3"

VALUES_FILE="${VALUES_FILE:-cert-manager-values.yaml}"
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
  echo "⏳ Waiting for cert-manager pods..."

  if ! kubectl wait \
    --namespace "${NAMESPACE}" \
    --for=condition=Ready pod \
    -l app.kubernetes.io/instance=${APP_NAME} \
    --timeout="${TIMEOUT}"; then

    echo "❌ Pods not Ready within ${TIMEOUT}"

    kubectl get pods -n "${NAMESPACE}"
    kubectl describe pods -n "${NAMESPACE}" || true

    exit 1
  fi

  echo "✅ Pods are Ready!"
}

# -----------------------------
# Install
# -----------------------------
install() {
  echo "🚀 Installing ${APP_NAME}..."

  check_deps

  echo "📦 Adding Helm repo..."
  helm repo add jetstack "${CHART_REPO}" >/dev/null 2>&1 || true
  helm repo update >/dev/null

  echo "📦 Ensuring namespace exists"
  kubectl get ns "${NAMESPACE}" >/dev/null 2>&1 || kubectl create ns "${NAMESPACE}"

  echo "📥 Deploying Helm chart..."
  helm upgrade --install "${APP_NAME}" "${CHART_NAME}" \
    --version "${VERSION}" \
    --namespace "${NAMESPACE}" \
    -f "${VALUES_FILE}" \
    --create-namespace

  wait_for_pods

  echo "🔍 Final status:"
  kubectl get pods -n "${NAMESPACE}"

  echo "✅ ${APP_NAME} installed successfully!"
}

# -----------------------------
# Uninstall
# -----------------------------
uninstall() {
  echo "🗑️ Uninstalling ${APP_NAME}..."

  check_deps

  helm uninstall "${APP_NAME}" -n "${NAMESPACE}" || true

  echo "⚠️ CRDs are NOT removed automatically"
  echo "👉 If needed:"
  echo "kubectl delete crd $(kubectl get crd | grep cert-manager | awk '{print \$1}')"

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
