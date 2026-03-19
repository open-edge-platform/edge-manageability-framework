#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

APP_NAME="postgresql-operator"
NAMESPACE="postgresql-operator"
CHART="oci://ghcr.io/cloudnative-pg/charts/cloudnative-pg"
VERSION="0.27.1"

VALUES_FILE="${VALUES_FILE:-postgresql-operator-values.yaml}"
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
    -l app.kubernetes.io/name=cloudnative-pg \
    --timeout="${TIMEOUT}"; then

    echo "❌ Pods not Ready within ${TIMEOUT}"

    echo "📊 Pod status:"
    kubectl get pods -n "${NAMESPACE}"

    echo "📄 Describe:"
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
# Uninstall
# -----------------------------
uninstall() {
  echo "🗑️ Uninstalling ${APP_NAME}..."

  check_deps

  helm uninstall "${APP_NAME}" -n "${NAMESPACE}" || true

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
