#!/bin/bash
set -euo pipefail

# =========================
# Vault Helm Deployment Script (Helm-only, values-only)
# Excludes onprem-vpro.yaml and .tpl; only merges YAML values
# =========================

BASE_DIR="${BASE_DIR:-/home/ubuntu/skip-argocd/edge-manageability-framework}"
NAMESPACE="orch-platform"
HELM_RELEASE="vault"
CHART_PATH="./vault-chart"   # Path to your local Helm chart
MERGED_VALUES="./merged-values.yaml"

# Merge only YAML values files (exclude onprem-vpro.yaml and tpl)
echo "Merging YAML values files into $MERGED_VALUES..."
yq eval-all 'select(fileIndex == 0) * select(fileIndex == 1)' \
  "$BASE_DIR/orch-configs/profiles/profile-onprem.yaml" \
  "$BASE_DIR/argocd/applications/configs/vault.yaml" \
  > "$MERGED_VALUES"

# Optionally, add proxy environment if needed
if [ -f "$BASE_DIR/onprem.env" ]; then
    echo "Adding proxy environment variables from onprem.env..."
    source "$BASE_DIR/onprem.env"
    cat >> "$MERGED_VALUES" << EOF
server:
  extraEnvironmentVars:
EOF
    [ -n "${HTTP_PROXY:-}" ] && echo "    http_proxy: \"$HTTP_PROXY\"" >> "$MERGED_VALUES"
    [ -n "${HTTPS_PROXY:-}" ] && echo "    https_proxy: \"$HTTPS_PROXY\"" >> "$MERGED_VALUES"
    [ -n "${NO_PROXY:-}" ] && echo "    no_proxy: \"$NO_PROXY\"" >> "$MERGED_VALUES"
fi

# Ensure namespace exists
kubectl get namespace "$NAMESPACE" >/dev/null 2>&1 || kubectl create namespace "$NAMESPACE"

# Deploy with Helm
echo "Deploying Vault with Helm..."
helm upgrade --install "$HELM_RELEASE" "$CHART_PATH" \
  --namespace "$NAMESPACE" \
  -f "$MERGED_VALUES"

echo "Vault deployment completed using Helm."
echo "Check pods: kubectl get pods -n $NAMESPACE"
