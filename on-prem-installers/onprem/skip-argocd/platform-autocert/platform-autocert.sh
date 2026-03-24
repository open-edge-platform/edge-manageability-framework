#!/bin/bash
set -euo pipefail

# Configuration
NAMESPACE="cert-manager"
CHART_OCI="oci://registry-rs.edgeorchestration.intel.com/edge-orch/common/charts/platform-autocert"
CHART_VERSION="1.0.2"
RELEASE_NAME="platform-autocert"

WAIT_TIMEOUT=300

# Select values file
ENVIRONMENT="${2:-onprem}"

if [[ "$ENVIRONMENT" == "aws" ]]; then
  VALUES_FILE="./values-aws.yaml"
elif [[ "$ENVIRONMENT" == "onprem" ]]; then
  VALUES_FILE="./values-onprem.yaml"
else
  echo "❌ Invalid environment: $ENVIRONMENT"
  echo "Usage: $0 install|uninstall [onprem|aws]"
  exit 1
fi

create_namespace() {
  kubectl get namespace "$NAMESPACE" >/dev/null 2>&1 || {
    echo "📦 Creating namespace: $NAMESPACE"
    kubectl create namespace "$NAMESPACE"
  }
}

wait_for_certificates() {
  echo "⏳ Waiting for cert-manager pods..."

  kubectl wait --for=condition=Ready pod \
    --all -n "$NAMESPACE" \
    --timeout=${WAIT_TIMEOUT}s

  # ClusterIssuer (only for AWS)
  if [[ "$ENVIRONMENT" == "aws" ]]; then
    echo "⏳ Waiting for ClusterIssuer..."

    for i in $(seq 1 $WAIT_TIMEOUT); do
      READY=$(kubectl get clusterissuer -o jsonpath='{.items[*].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || true)

      if [[ "$READY" == *"True"* ]]; then
        echo "✅ ClusterIssuer ready"
        break
      fi
      sleep 2
    done
  else
    echo "ℹ️ Skipping ClusterIssuer check (on-prem)"
  fi

  echo "⏳ Waiting for Certificates..."

  for i in $(seq 1 $WAIT_TIMEOUT); do
    COUNT=$(kubectl get certificates -A --no-headers 2>/dev/null | wc -l || true)

    if [[ "$COUNT" -gt 0 ]]; then
      break
    fi
    sleep 2
  done

  echo "⏳ Waiting for Certificates READY..."

  for i in $(seq 1 $WAIT_TIMEOUT); do
    NOT_READY=$(kubectl get certificates -A --no-headers 2>/dev/null | grep -v True || true)

    if [[ -z "$NOT_READY" ]]; then
      echo "✅ Certificates ready"
      break
    fi
    sleep 3
  done

  echo "🎉 Certificates verified"
}

deploy() {
  echo "🚀 Deploying platform-autocert ($ENVIRONMENT)..."

  create_namespace

  export HELM_EXPERIMENTAL_OCI=1

  helm upgrade --install "$RELEASE_NAME" "$CHART_OCI" \
    --version "$CHART_VERSION" \
    --namespace "$NAMESPACE" \
    -f "$VALUES_FILE" \
    --wait --timeout 10m

  #wait_for_certificates

  echo "✅ Deployment complete"
}

uninstall() {
  echo "🗑️ Uninstalling..."

  if helm status "$RELEASE_NAME" -n "$NAMESPACE" >/dev/null 2>&1; then
    helm uninstall "$RELEASE_NAME" -n "$NAMESPACE"
    echo "✅ Uninstalled"
  else
    echo "ℹ️ Not installed"
  fi
}

case "${1:-}" in
  install)
    deploy
    ;;
  uninstall)
    uninstall
    ;;
  *)
    echo "Usage: $0 install|uninstall [onprem|aws]"
    exit 1
    ;;
esac
