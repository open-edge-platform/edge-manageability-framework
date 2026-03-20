#!/bin/bash
set -euo pipefail

VALUES_FILE="./values-postgresql-secrets.yaml"
NAMESPACE="$(yq e '.namespace' $VALUES_FILE)"
CHART_URL="$(yq e '.chartURL' $VALUES_FILE)"
RELEASE_NAME="$(yq e '.releaseName' $VALUES_FILE)"
CHART_VERSION="$(yq e '.version' $VALUES_FILE)"
WAIT_TIMEOUT="180s"

create_namespace() {
  kubectl get namespace "$NAMESPACE" >/dev/null 2>&1 || \
    kubectl create namespace "$NAMESPACE"
}

wait_for_secrets() {
  echo "⏳ Waiting for PostgreSQL database secrets to be ready..."

  # Count the databases in values.yaml
  DB_COUNT=$(yq e '.databases | length' $VALUES_FILE)
  if [[ $DB_COUNT -eq 0 ]]; then
    echo "ℹ️ No databases defined in values file. Skipping wait."
    return
  fi

  for i in $(seq 0 $((DB_COUNT - 1))); do
    DB_NAME=$(yq e ".databases[$i].name" $VALUES_FILE)
    SECRET_NAME="${DB_NAME}-local-postgresql"

    # Only check for database secrets, ignore Helm release secret
    if kubectl get secret "$SECRET_NAME" -n "$NAMESPACE" >/dev/null 2>&1; then
      echo "✅ Secret $SECRET_NAME exists"
    else
      echo "ℹ️ Secret $SECRET_NAME not found yet, waiting..."
      kubectl wait --namespace "$NAMESPACE" \
        --for=condition=available secret/"$SECRET_NAME" \
        --timeout="$WAIT_TIMEOUT" 2>/dev/null || \
        echo "ℹ️ Secret $SECRET_NAME may already exist or does not have a condition"
    fi
  done

  echo "✅ All PostgreSQL database secrets are ready"
}

deploy_postgresql_secrets() {
  echo "🚀 Deploying PostgreSQL secrets..."

  create_namespace

  helm upgrade --install "$RELEASE_NAME" "$CHART_URL" \
    --namespace "$NAMESPACE" \
    --version "$CHART_VERSION" \
    -f "$VALUES_FILE" \
    --wait --timeout 10m

  wait_for_secrets

  echo "✅ PostgreSQL secrets deployed successfully"
}

uninstall_postgresql_secrets() {
  echo "🗑️ Uninstalling PostgreSQL secrets..."

  if helm status "$RELEASE_NAME" -n "$NAMESPACE" >/dev/null 2>&1; then
    helm uninstall "$RELEASE_NAME" -n "$NAMESPACE"
    echo "✅ PostgreSQL secrets uninstalled"
  else
    echo "ℹ️ PostgreSQL secrets not installed"
  fi
}

case "${1:-}" in
  install)
    deploy_postgresql_secrets
    ;;
  uninstall)
    uninstall_postgresql_secrets
    ;;
  *)
    echo "Usage: $0 install|uninstall"
    exit 1
    ;;
esac
