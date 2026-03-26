#!/bin/bash
set -euo pipefail

# =========================
# Vault Deployment Script
# Supports: install | uninstall
# Reads proxy settings from ../onprem.env
# Uses Helm chart version 0.32.0
# =========================

NAMESPACE="orch-platform"
HELM_RELEASE="vault"
VALUES_FILE="./values.yaml"
CHART_VERSION="0.32.0"

# Usage
if [ $# -ne 1 ]; then
    echo "Usage: $0 [install|uninstall]"
    exit 1
fi

ACTION="$1"

# Load environment variables (proxy, etc.)
if [ -f "../onprem.env" ]; then
    echo "Sourcing ../onprem.env..."
    source ../onprem.env
else
    echo "Warning: ../onprem.env not found, proceeding without proxy settings."
fi

# Create namespace if it doesn't exist
kubectl get namespace "$NAMESPACE" >/dev/null 2>&1 || kubectl create namespace "$NAMESPACE"

# Prepare temporary values-proxy.yaml
PROXY_VALUES_FILE=$(mktemp)
cat > "$PROXY_VALUES_FILE" << EOF
server:
  extraEnvironmentVars:
EOF

[ -n "${HTTP_PROXY:-}" ] && echo "    http_proxy: \"$HTTP_PROXY\"" >> "$PROXY_VALUES_FILE"
[ -n "${HTTPS_PROXY:-}" ] && echo "    https_proxy: \"$HTTPS_PROXY\"" >> "$PROXY_VALUES_FILE"
[ -n "${NO_PROXY:-}" ] && echo "    no_proxy: \"$NO_PROXY\"" >> "$PROXY_VALUES_FILE"

# Action handling
case "$ACTION" in
    install)
        echo "Installing/upgrading Vault in namespace '$NAMESPACE'..."
        helm repo add hashicorp https://helm.releases.hashicorp.com >/dev/null 2>&1 || true
        helm repo update >/dev/null 2>&1

        helm upgrade --install "$HELM_RELEASE" hashicorp/vault \
            --namespace "$NAMESPACE" \
	     --values /home/ubuntu/skip-argocd/edge-manageability-framework/argocd/applications/configs/vault.yaml \
 --values /home/ubuntu/skip-argocd/edge-manageability-framework/argocd/applications/custom/vault.tpl \
 --values /home/ubuntu/skip-argocd/edge-manageability-framework/orch-configs/profiles/enable-platform-vpro.yaml \
 --values /home/ubuntu/skip-argocd/edge-manageability-framework/orch-configs/profiles/profile-onprem.yaml \
  --values /home/ubuntu/skip-argocd/edge-manageability-framework/on-prem-installers/onprem/onprem-vpro.yaml \
            --values "$PROXY_VALUES_FILE" \
            --version "$CHART_VERSION"

        echo "Vault installation/upgrade initiated with chart version $CHART_VERSION."
        echo "Check pods: kubectl get pods -n $NAMESPACE"
        ;;
    uninstall)
        echo "Uninstalling Vault from namespace '$NAMESPACE'..."
        # Only uninstall if release exists
        if helm status "$HELM_RELEASE" -n "$NAMESPACE" >/dev/null 2>&1; then
            helm uninstall "$HELM_RELEASE" --namespace "$NAMESPACE"
            echo "Vault uninstallation completed."
        else
            echo "Vault release '$HELM_RELEASE' not found in namespace '$NAMESPACE'. Skipping uninstall."
        fi
        ;;
    *)
        echo "Invalid action: $ACTION"
        echo "Usage: $0 [install|uninstall]"
        exit 1
        ;;
esac

# Cleanup
rm -f "$PROXY_VALUES_FILE"
