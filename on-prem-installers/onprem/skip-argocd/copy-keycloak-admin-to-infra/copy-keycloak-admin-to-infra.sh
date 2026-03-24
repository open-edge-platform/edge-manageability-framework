#!/bin/bash
# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -e
source ../onprem.env

ACTION="${1:-install}"     # install | uninstall | verify
NAMESPACE="orch-infra"
RELEASE_NAME="copy-keycloak-admin-to-infra"
CHART="oci://${RELEASE_SERVICE_URL}/edge-orch/common/charts/copy-secret"
CHART_VERSION="26.0.0"
VALUES_FILE="values.yaml"

# --------------------------
# Pre-checks
# --------------------------
command -v helm >/dev/null 2>&1 || { echo "❌ Helm not installed"; exit 1; }
command -v kubectl >/dev/null 2>&1 || { echo "❌ kubectl not installed"; exit 1; }

kubectl cluster-info >/dev/null 2>&1 || { echo "❌ Kubernetes cluster unreachable"; exit 1; }

# --------------------------
# External Secrets Check
# --------------------------
check_external_secrets() {
    echo "🔍 Checking External Secrets Operator..."
    if ! kubectl get crd externalsecrets.external-secrets.io >/dev/null 2>&1; then
        echo "❌ External Secrets CRDs not found. Installing..."
        kubectl apply -f https://raw.githubusercontent.com/external-secrets/external-secrets/main/deploy/crds/bundle.yaml
    else
        echo "✅ External Secrets CRDs present"
    fi

    if ! kubectl get pods -n external-secrets-system >/dev/null 2>&1; then
        echo "⚠️ External Secrets Operator not running."
        echo "   Install it if required:"
        echo "   helm repo add external-secrets https://charts.external-secrets.io"
        echo "   helm install external-secrets external-secrets/external-secrets -n external-secrets-system --create-namespace"
    fi
}

# --------------------------
# Namespace Setup
# --------------------------
create_namespaces() {
    echo "🔧 Creating namespace..."
    kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -
}

# --------------------------
# Verify Deployment
# --------------------------
verify_deployment() {
    echo "🔍 Verifying deployment..."

    echo "📊 Helm Release Status:"
    helm status "$RELEASE_NAME" -n "$NAMESPACE" || echo "Release not found"

    echo -e "\n📦 External Secrets Resources:"
    kubectl get externalsecrets,secretstores -n "$NAMESPACE" -o wide || true

    echo -e "\n📦 Secrets:"
    kubectl get secrets -n "$NAMESPACE" || true

    echo -e "\n📋 Recent Events:"
    kubectl get events -n "$NAMESPACE" --sort-by='.lastTimestamp' | tail -10 || true

    echo -e "\n🔍 ExternalSecret Details:"
    kubectl describe externalsecret -n "$NAMESPACE" 2>/dev/null || echo "No ExternalSecrets found"

    echo -e "\n🔍 SecretStore Details:"
    kubectl describe secretstore -n "$NAMESPACE" 2>/dev/null || echo "No SecretStores found"
}

# --------------------------
# Optional Source Secret Check
# --------------------------
check_source_secret() {
    echo "🔧 Checking for source secret..."
    if ! kubectl get secret platform-keycloak -n orch-platform >/dev/null 2>&1; then
        echo "⚠️ Source secret 'platform-keycloak' not found in namespace 'orch-platform'"
        echo "   This is required for copying admin credentials."
        return 1
    fi
    echo "✅ Source secret found"
}

# --------------------------
# Install / Uninstall
# --------------------------
case "$ACTION" in
  install)
    check_external_secrets
    create_namespaces

    echo "🚀 Installing ${RELEASE_NAME}..."

    # Optional: login if OCI registry is private
    # helm registry login $RELEASE_SERVICE_URL -u <user> -p <password>

    helm upgrade --install "$RELEASE_NAME" "$CHART" \
      --version "$CHART_VERSION" \
      -n "$NAMESPACE" \
      --create-namespace \
      -f "$VALUES_FILE" \
      --wait

    echo "✅ Deployment successful"

    verify_deployment
    check_source_secret || echo "⚠️ Please create the source secret to complete setup"
    ;;

  uninstall)
    echo "🗑 Uninstalling ${RELEASE_NAME}..."
    helm uninstall "$RELEASE_NAME" -n "$NAMESPACE" || true
    echo "✅ Uninstallation completed"
    ;;

  verify)
    verify_deployment
    ;;

  *)
    echo "Usage: $0 [install|uninstall|verify]"
    exit 1
    ;;
esac
