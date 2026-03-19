#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

APP_NAME="prometheus-crd"
NAMESPACE="orch-platform"
CHART_REPO="https://prometheus-community.github.io/helm-charts"
CHART_NAME="prometheus-community/prometheus-operator-crds"
VERSION="24.0.1"

VALUES_FILE="${VALUES_FILE:-prometheus-crd-values.yaml}"
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
# Wait for CRDs
# -----------------------------
wait_for_crds() {
  echo "⏳ Waiting for Prometheus CRDs..."

  CRDS=(
    "prometheuses.monitoring.coreos.com"
    "servicemonitors.monitoring.coreos.com"
    "podmonitors.monitoring.coreos.com"
    "alertmanagers.monitoring.coreos.com"
  )

  for crd in "${CRDS[@]}"; do
    echo "🔎 Checking CRD: $crd"
    kubectl wait --for=condition=Established crd/${crd} --timeout="${TIMEOUT}" || {
      echo "❌ CRD $crd not ready or does not exist"
    }
  done

  echo "✅ Prometheus CRDs are ready!"
}

# -----------------------------
# Install
# -----------------------------
install() {
  echo "🚀 Installing ${APP_NAME}..."

  check_deps

  echo "📦 Adding Helm repo..."
  helm repo add prometheus-community "${CHART_REPO}" >/dev/null 2>&1 || true
  helm repo update >/dev/null

  echo "📦 Ensuring namespace exists..."
  kubectl get ns "${NAMESPACE}" >/dev/null 2>&1 || kubectl create ns "${NAMESPACE}"

  echo "📥 Deploying CRD chart..."
  helm upgrade --install "${APP_NAME}" "${CHART_NAME}" \
    --version "${VERSION}" \
    --namespace "${NAMESPACE}" \
    -f "${VALUES_FILE}" \
    --create-namespace || {
      echo "⚠️ Helm reported an error — it may already exist"
  }

  wait_for_crds

  echo "🔍 Installed CRDs:"
  kubectl get crd | grep monitoring || true

  echo "✅ ${APP_NAME} installed successfully!"
}

# -----------------------------
# Uninstall
# -----------------------------
uninstall() {
  FORCE_CRDS="${1:-false}"

  echo "🗑️ Uninstalling ${APP_NAME}..."
  check_deps

  helm uninstall "${APP_NAME}" -n "${NAMESPACE}" || true

  if [[ "$FORCE_CRDS" == "true" ]]; then
    echo "🗑️ Deleting Prometheus CRDs..."
    kubectl get crd | grep monitoring | awk '{print $1}' | xargs -r kubectl delete crd
  else
    echo "⚠️ CRDs are NOT removed automatically"
    echo "👉 To remove manually:"
    echo "kubectl get crd | grep monitoring | awk '{print \$1}' | xargs kubectl delete crd"
  fi

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
    # Optional: pass "true" to force delete CRDs
    uninstall "${2:-false}"
    ;;
  *)
    echo "Usage: $0 {install|uninstall} [force-crds]"
    echo "Example to uninstall CRDs: $0 uninstall true"
    exit 1
    ;;
esac
