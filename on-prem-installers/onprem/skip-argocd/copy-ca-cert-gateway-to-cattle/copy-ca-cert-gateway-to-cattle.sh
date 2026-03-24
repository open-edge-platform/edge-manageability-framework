#!/bin/bash
# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -e
source ../onprem.env

ACTION="${1:-install}"     # install or uninstall
NAMESPACE="cattle-system"
RELEASE_NAME="copy-ca-cert-gateway-to-cattle"
CHART="oci://${RELEASE_SERVICE_URL}/edge-orch/common/charts/copy-secret"
CHART_VERSION="26.0.0"


# Check dependencies
command -v helm >/dev/null 2>&1 || { echo "❌ Helm not installed"; exit 1; }
command -v kubectl >/dev/null 2>&1 || { echo "❌ kubectl not installed"; exit 1; }

# Ensure cluster is reachable
kubectl cluster-info >/dev/null 2>&1 || { echo "❌ Kubernetes cluster unreachable"; exit 1; }

# Check External Secrets Operator
check_external_secrets() {
    echo "🔍 Checking External Secrets Operator..."
    if ! kubectl get crd externalsecrets.external-secrets.io >/dev/null 2>&1; then
        echo "❌ External Secrets Operator CRDs not found. Installing..."
        kubectl apply -f https://raw.githubusercontent.com/external-secrets/external-secrets/main/deploy/crds/bundle.yaml
    fi
    
    if ! kubectl get pods -n external-secrets-system >/dev/null 2>&1; then
        echo "⚠️  External Secrets Operator not running. You may need to install it:"
        echo "   helm repo add external-secrets https://charts.external-secrets.io"
        echo "   helm install external-secrets external-secrets/external-secrets -n external-secrets-system --create-namespace"
    fi
}

# Verification function
verify_deployment() {
    echo "🔍 Verifying deployment..."
    
    echo "📊 Helm Release Status:"
    helm status "$RELEASE_NAME" -n "$NAMESPACE"
    
    echo -e "\n📦 External Secrets Resources:"
    kubectl get externalsecrets,secretstores -n cattle-system -o wide
    
    echo -e "\n📦 Secrets in cattle-system:"
    kubectl get secrets -n cattle-system
    
    echo -e "\n📋 Recent Events:"
    kubectl get events -n cattle-system --sort-by='.lastTimestamp' | tail -10
    
    echo -e "\n🔍 ExternalSecret Details:"
    kubectl describe externalsecret -n cattle-system 2>/dev/null || echo "No ExternalSecrets found"
    
    echo -e "\n🔍 SecretStore Details:"
    kubectl describe secretstore -n cattle-system 2>/dev/null || echo "No SecretStores found"
}

# Create source secret if it doesn't exist
create_source_secret() {
    echo "🔧 Checking for source secret..."
    if ! kubectl get secret source-name -n cattle-system >/dev/null 2>&1; then
        echo "⚠️  Source secret 'source-name' not found."
        echo "   You need to create the source secret that contains the CA certificate."
        echo "   Example:"
        echo "   kubectl create secret generic source-name -n cattle-system --from-file=ca.crt=/path/to/ca.crt"
        return 1
    fi
    echo "✅ Source secret found"
}

# Create required namespaces
create_namespaces() {
    echo "🔧 Creating required namespaces..."
    kubectl create namespace remote-ns --dry-run=client -o yaml | kubectl apply -f -
    kubectl create namespace cattle-system --dry-run=client -o yaml | kubectl apply -f -
}

# --------------------------
# Install / Uninstall
# --------------------------
case "$ACTION" in
  install)
    check_external_secrets
    create_namespaces
    echo "🚀 Installing ${RELEASE_NAME}..."
    helm upgrade --install "$RELEASE_NAME" "$CHART" \
      --version "$CHART_VERSION" \
      -n "$NAMESPACE" \
      --create-namespace \
      --wait
    echo "✅ Deployment successful"
    verify_deployment
    create_source_secret || echo "⚠️  Please create the source secret to complete the setup"
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
