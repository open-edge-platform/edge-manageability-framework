#!/bin/bash
set -euo pipefail

NAMESPACE="$(yq e '.namespace' ./values-wait-istio-job.yaml)"
CHART_URL="$(yq e '.chartURL' ./values-wait-istio-job.yaml)"
RELEASE_NAME="$(yq e '.releaseName' ./values-wait-istio-job.yaml)"
VALUES_FILE="./values-wait-istio-job.yaml"
WAIT_TIMEOUT="180s"

create_namespace() {
  kubectl get namespace "$NAMESPACE" >/dev/null 2>&1 || \
    kubectl create namespace "$NAMESPACE"
}

wait_for_job_completion() {
  echo "⏳ Waiting for Istio wait job to complete..."

  # Dynamically find the Job created by Helm
  JOB_NAME=$(kubectl get jobs -n "$NAMESPACE" -o name | grep "$RELEASE_NAME" | head -n1)

  if [ -z "$JOB_NAME" ]; then
    echo "⚠️ No Job found for release $RELEASE_NAME in namespace $NAMESPACE"
    exit 1
  fi

  kubectl wait --namespace "$NAMESPACE" \
    --for=condition=complete "$JOB_NAME" \
    --timeout="$WAIT_TIMEOUT"

  echo "✅ Wait job completed"
}

deploy_wait_istio_job() {
  echo "🚀 Deploying Wait-Istio-Job from OCI chart..."

  create_namespace

  helm upgrade --install "$RELEASE_NAME" "$CHART_URL" \
    --namespace "$NAMESPACE" \
    --version "$(yq e '.job.version' $VALUES_FILE)" \
    -f "$VALUES_FILE" \
    --wait --timeout 10m

  wait_for_job_completion

  echo "✅ Wait-Istio-Job deployed successfully"
}

uninstall_wait_istio_job() {
  echo "🗑️ Uninstalling Wait-Istio-Job..."

  if helm status "$RELEASE_NAME" -n "$NAMESPACE" >/dev/null 2>&1; then
    helm uninstall "$RELEASE_NAME" -n "$NAMESPACE"
    echo "✅ Wait-Istio-Job uninstalled"
  else
    echo "ℹ️ Wait-Istio-Job not installed"
  fi
}

case "${1:-}" in
  install)
    deploy_wait_istio_job
    ;;
  uninstall)
    uninstall_wait_istio_job
    ;;
  *)
    echo "Usage: $0 install|uninstall"
    exit 1
    ;;
esac
