#!/bin/bash

set -e

# Usage:
# ./deploy.sh install [aws] [custom-values.yaml]
# ./deploy.sh uninstall

ACTION="${1:-install}"
MODE="${2:-onprem}"
CUSTOM_VALUES="$3"

# Load environment
source ../onprem.env

CHART="oci://${RELEASE_SERVICE_URL}/edge-orch/common/charts/cert-synchronizer"
VERSION="26.0.10"
RELEASE_NAME="cert-synchronizer"
NAMESPACE="orch-gateway"

BASE_VALUES="values.yaml"
AWS_VALUES="values-aws.yaml"

# Validate required env vars
: "${CLUSTER_NAME:?❌ CLUSTER_NAME not set in onprem.env}"
: "${CLUSTER_DOMAIN:?❌ CLUSTER_DOMAIN not set in onprem.env}"

echo "-----------------------------------"
echo "Action          : $ACTION"
echo "Mode            : $MODE"
echo "Cluster Name    : $CLUSTER_NAME"
echo "Cluster Domain  : $CLUSTER_DOMAIN"
echo "-----------------------------------"

# Pre-checks
command -v helm >/dev/null || { echo "❌ Helm not installed"; exit 1; }
command -v kubectl >/dev/null || { echo "❌ kubectl not installed"; exit 1; }
kubectl cluster-info >/dev/null 2>&1 || { echo "❌ Cluster not reachable"; exit 1; }

# ---------------- UNINSTALL ----------------
if [[ "$ACTION" == "uninstall" ]]; then
  echo "🗑️ Uninstalling ${RELEASE_NAME}..."
  helm uninstall ${RELEASE_NAME} -n ${NAMESPACE} 2>/dev/null || echo "Not installed"
  exit 0
fi

# ---------------- INSTALL ----------------
VALUES_ARGS="-f ${BASE_VALUES}"

if [[ "$MODE" == "aws" ]]; then
  VALUES_ARGS="$VALUES_ARGS -f ${AWS_VALUES}"
fi

if [[ -n "$CUSTOM_VALUES" ]]; then
  VALUES_ARGS="$VALUES_ARGS -f $CUSTOM_VALUES"
fi

echo "Using values: $VALUES_ARGS"

# Temp values (safe for proxy + env variables)
TMP_VALUES=$(mktemp)

cat <<EOF > "$TMP_VALUES"
proxy:
  httpProxy: "${HTTP_PROXY}"
  httpsProxy: "${HTTPS_PROXY}"
  noProxy: "${NO_PROXY}"
clusterName: "${CLUSTER_NAME}"
clusterDomain: "${CLUSTER_DOMAIN}"
certDomain: "${CLUSTER_NAME}"
image:
  registry: "${RELEASE_SERVICE_URL}"
EOF


echo "🚀 Deploying..."

helm upgrade --install ${RELEASE_NAME} ${CHART} \
  --version ${VERSION} \
  -n ${NAMESPACE} \
  --create-namespace \
  ${VALUES_ARGS} \
  -f "$TMP_VALUES" \
  --set-json 'imagePullSecrets=[]' \
  --wait

echo "-----------------------------------"
echo "✅ Deployment successful"
echo "Cluster: ${CLUSTER_NAME}.${CLUSTER_DOMAIN}"
echo "-----------------------------------"

kubectl get pods -n ${NAMESPACE}

rm -f "$TMP_VALUES"
