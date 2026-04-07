#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
#
# Fix cluster-manager-credentials-script ConfigMap to handle Istio sidecar wait
# gracefully when Istio is disabled. The chart hardcodes wait_for_istio_sidecar()
# which returns 1 when no sidecar is present, and set -euo pipefail kills the script.
# This hook patches the main() call to add "|| true" so the script continues.

set -euo pipefail

NAMESPACE="orch-cluster"
CM_NAME="cluster-manager-credentials-script"

if ! kubectl get configmap "$CM_NAME" -n "$NAMESPACE" &>/dev/null; then
  echo "ConfigMap $CM_NAME not found in $NAMESPACE — skipping patch"
  exit 0
fi

# Check if already patched
if kubectl get configmap "$CM_NAME" -n "$NAMESPACE" -o jsonpath='{.data.credentials-m2m\.sh}' | grep -q "wait_for_istio_sidecar || true"; then
  echo "ConfigMap $CM_NAME already patched — skipping"
  exit 0
fi

echo "Patching $CM_NAME to handle Istio sidecar wait gracefully..."

kubectl get configmap "$CM_NAME" -n "$NAMESPACE" -o jsonpath='{.data.credentials-m2m\.sh}' > /tmp/credentials-m2m.sh
sed -i 's/^    wait_for_istio_sidecar$/    wait_for_istio_sidecar || true/' /tmp/credentials-m2m.sh

kubectl create configmap "$CM_NAME" -n "$NAMESPACE" \
  --from-file=credentials-m2m.sh=/tmp/credentials-m2m.sh \
  --dry-run=client -o yaml | kubectl apply -f -

rm -f /tmp/credentials-m2m.sh

# Delete the job so it re-runs with the patched script
if kubectl get job "$CM_NAME" -n "$NAMESPACE" &>/dev/null; then
  echo "Deleting job $CM_NAME so it re-runs with patched script..."
  kubectl delete job "$CM_NAME" -n "$NAMESPACE" --ignore-not-found
fi

echo "Done — credentials script patched for non-Istio environments"
