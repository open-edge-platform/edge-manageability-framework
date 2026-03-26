#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

APP_NAME="platform-keycloak"
RELEASE_NAME="platform-keycloak"
NAMESPACE="orch-platform"
CHART="oci://registry-rs.edgeorchestration.intel.com/edge-orch/common/charts/keycloak-instance"
VERSION="26.1.2"

VALUES_FILE="${VALUES_FILE:-$ROOT_DIR/values.yaml}"
TIMEOUT="300s"

usage() {
  cat <<EOF
Usage: $(basename "$0") {install|uninstall}

Actions:
  install    Install or upgrade the Helm release using ${VALUES_FILE}
  uninstall  Uninstall the Helm release

Notes:
  - This script does NOT install Helm or kubectl. Ensure `helm` and `kubectl` are available in PATH.
  - Edit ${VALUES_FILE} for cluster-specific configuration before running install.
EOF
}

check_deps() {
  for cmd in helm kubectl; do
    command -v "$cmd" >/dev/null 2>&1 || {
      echo "❌ $cmd not installed" >&2
      exit 1
    }
  done
}

wait_for_pods() {
  echo "⏳ Waiting for pods to be Ready (timeout: ${TIMEOUT})..."

  # try common selectors first
  SELECTORS=("app.kubernetes.io/instance=${RELEASE_NAME}" "app.kubernetes.io/name=${APP_NAME}")
  for sel in "${SELECTORS[@]}"; do
    if kubectl get pods -n "${NAMESPACE}" -l "$sel" >/dev/null 2>&1; then
      if kubectl wait --namespace "${NAMESPACE}" --for=condition=Ready pod -l "$sel" --timeout="${TIMEOUT}"; then
        echo "✅ Pods are Ready (selector: $sel)!"
        return 0
      else
        echo "❌ Pods not Ready within ${TIMEOUT} for selector: $sel"
        echo "📊 Current pod status (selector: $sel):"
        kubectl get pods -n "${NAMESPACE}" -l "$sel" || true
        echo "📄 Describe pods:"
        kubectl describe pods -n "${NAMESPACE}" -l "$sel" || true
        return 1
      fi
    fi
  done

  # fallback: look for pods with 'keycloak' in name
  NAMES=$(kubectl get pods -n "${NAMESPACE}" --no-headers -o custom-columns=NAME:.metadata.name 2>/dev/null | grep -E 'keycloak' || true)
  if [ -z "$NAMES" ]; then
    echo "error: no matching resources found"
    return 1
  fi

  # parse TIMEOUT seconds
  TIMEOUT_SEC=${TIMEOUT%s}
  END=$((SECONDS + TIMEOUT_SEC))
  while [ $SECONDS -lt $END ]; do
    ALL_READY=true
    for n in $NAMES; do
      # check pod ready conditions
      ready=$(kubectl get pod "$n" -n "${NAMESPACE}" -o jsonpath='{.status.containerStatuses[*].ready}' 2>/dev/null || echo "")
      if [ -z "$ready" ] || echo "$ready" | grep -q false; then
        ALL_READY=false
        break
      fi
    done
    if [ "$ALL_READY" = true ]; then
      echo "✅ Pods are Ready!"
      return 0
    fi
    sleep 5
  done

  echo "❌ Pods not Ready within ${TIMEOUT}"
  echo "📊 Current pod status:"
  kubectl get pods -n "${NAMESPACE}" || true
  echo "📄 Describe pods:"
  kubectl describe pods -n "${NAMESPACE}" || true
  return 1
}

install() {
  echo "🚀 Installing ${APP_NAME}..."

  check_deps

  echo "📦 Ensuring namespace exists: ${NAMESPACE}"
  kubectl get ns "${NAMESPACE}" >/dev/null 2>&1 || kubectl create ns "${NAMESPACE}"

  echo "📥 Deploying Helm chart..."
  set -x
  if [ -n "$VERSION" ]; then
    helm upgrade --install "${RELEASE_NAME}" "${CHART}" --version "${VERSION}" --namespace "${NAMESPACE}" -f "${VALUES_FILE}" --create-namespace
  else
    helm upgrade --install "${RELEASE_NAME}" "${CHART}" --namespace "${NAMESPACE}" -f "${VALUES_FILE}" --create-namespace
  fi
  set +x

  if ! wait_for_pods; then
    echo "Install succeeded but pods did not become Ready in time. Inspect cluster." >&2
    #exit 1
  fi

  echo "🔍 Final pod status:"
  kubectl get pods -n "${NAMESPACE}"

  echo "✅ ${APP_NAME} installed successfully!"
}

uninstall() {
  echo "🗑️ Uninstalling ${APP_NAME}..."

  check_deps

  helm uninstall "${RELEASE_NAME}" -n "${NAMESPACE}" || true

  echo "✅ ${APP_NAME} uninstalled!"
}

if [ $# -ne 1 ]; then
  usage
  exit 1
fi

case "$1" in
  install)
    install
    ;;
  uninstall)
    uninstall
    ;;
  *)
    usage
    exit 1
    ;;
esac

