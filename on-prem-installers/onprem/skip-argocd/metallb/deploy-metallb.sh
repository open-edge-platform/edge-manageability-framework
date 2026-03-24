#!/bin/bash

set -euo pipefail

#############################################
# GLOBAL CONFIG
#############################################

NAMESPACE="metallb-system"

# MetalLB (Helm repo)
METALLB_CHART="metallb/metallb"
METALLB_VERSION="0.15.2"
VALUES_METALLB="./values-metallb.yaml"
METALLB_REPO="${METALLB_REPO:-registry-rs.edgeorchestration.intel.com/edge-orch}"

# Correct chart path (use local charts directory by default)
METALLB_CONFIG_PATH="metallb-config"

VALUES_CONFIG="./values-metallb-config.yaml"
METALLB_CONFIG_VERSION="${METALLB_CONFIG_VERSION:-26.0.1}"

#############################################

#############################################
# COMMON FUNCTIONS
#############################################

log() {
  echo -e "\n======================================"
  echo "$1"
  echo "======================================"
}

create_namespace() {
  kubectl get namespace "${NAMESPACE}" >/dev/null 2>&1 || \
  kubectl create namespace "${NAMESPACE}"
}

validate_file() {
  local file=$1
  if [ ! -f "${file}" ]; then
    echo "❌ ERROR: Required file not found: ${file}"
    exit 1
  fi
}


#############################################
# INSTALL FUNCTIONS
#############################################

deploy_metallb() {

  log "🚀 Deploying MetalLB (Wave 100)"

  validate_file "${VALUES_METALLB}"

  helm repo add metallb "${METALLB_REPO}" >/dev/null 2>&1 || true
  helm repo update >/dev/null 2>&1

  create_namespace

  helm upgrade --install metallb "${METALLB_CHART}" \
    --namespace "${NAMESPACE}" \
    --version "${METALLB_VERSION}" \
    -f "${VALUES_METALLB}" \
    --create-namespace \
    --wait --timeout 10m

  echo "✅ MetalLB deployed (version ${METALLB_VERSION})"
}

deploy_metallb_config() {

  log "🚀 Deploying MetalLB Config (Wave 150)"

  validate_file "${VALUES_CONFIG}"

  create_namespace
  # Install from local chart path (preferred)
  if [ -d "${METALLB_CONFIG_PATH}" ] && [ -f "${METALLB_CONFIG_PATH}/Chart.yaml" ]; then
    echo "🔎 Installing local chart: ${METALLB_CONFIG_PATH}"
    if helm upgrade --install metallb-config "${METALLB_CONFIG_PATH}" \
      --namespace "${NAMESPACE}" \
      -f "${VALUES_CONFIG}" \
      --wait --timeout 10m; then
      echo "✅ MetalLB Config deployed from local chart"
    else
      echo "❌ ERROR: Local chart install failed. Check helm output for details."
      exit 1
    fi
  else
    echo "❌ ERROR: Local chart not found at ${METALLB_CONFIG_PATH} or missing Chart.yaml."
    echo "Place the metallb-config chart at ${METALLB_CONFIG_PATH} or set METALLB_CONFIG_PATH to the chart directory."
    echo "Alternatively, pull the OCI chart locally:"
    echo "  helm pull oci://${METALLB_REPO}/common/charts/metallb-config --version ${METALLB_CONFIG_VERSION} --untar --untardir ."
    exit 1
  fi
}

#############################################
# UNINSTALL FUNCTIONS
#############################################

uninstall_metallb_config() {

  log "🗑️ Uninstalling MetalLB Config (reverse order)"

  if helm status metallb-config -n "${NAMESPACE}" >/dev/null 2>&1; then
    helm uninstall metallb-config -n "${NAMESPACE}"
    echo "✅ metallb-config removed"
  else
    echo "ℹ️ metallb-config not installed"
  fi
}

uninstall_metallb() {

  log "🗑️ Uninstalling MetalLB"

  if helm status metallb -n "${NAMESPACE}" >/dev/null 2>&1; then
    helm uninstall metallb -n "${NAMESPACE}"
    echo "✅ metallb removed"
  else
    echo "ℹ️ metallb not installed"
  fi

  # Cleanup namespace if empty
  if kubectl get namespace "${NAMESPACE}" >/dev/null 2>&1; then
    if [ -z "$(kubectl get all -n "${NAMESPACE}" --no-headers 2>/dev/null)" ]; then
      kubectl delete namespace "${NAMESPACE}"
      echo "🧹 Namespace ${NAMESPACE} deleted"
    else
      echo "ℹ️ Namespace not empty, skipping delete"
    fi
  fi
}

#############################################
# MAIN
#############################################

case "${1:-}" in
  install)
    deploy_metallb
    deploy_metallb_config
    ;;
  uninstall)
    uninstall_metallb_config
    uninstall_metallb
    ;;
  install-metallb)
    deploy_metallb
    ;;
  install-config)
    deploy_metallb_config
    ;;
  uninstall-config)
    uninstall_metallb_config
    ;;
  uninstall-metallb)
    uninstall_metallb
    ;;
  *)
    echo "Usage:"
    echo "  $0 install-all | uninstall-all | install-metallb | install-config"
    exit 1
    ;;
esac

