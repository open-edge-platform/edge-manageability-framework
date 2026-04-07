#!/bin/bash

# Namespaces you want to KEEP (do NOT delete)
EXCLUDE_REGEX="kube-system|metallb-system|openebs-system|default|kube-public|kube-node-lease"

echo "🔍 Finding namespaces to delete..."

namespaces=$(kubectl get ns --no-headers | awk '{print $1}' | grep -Ev "$EXCLUDE_REGEX")

echo "🗑️ Namespaces to delete:"
echo "$namespaces"
echo "----------------------------------"

for ns in $namespaces; do
  echo "➡️ Deleting namespace: $ns"

  # Try normal delete first
  kubectl delete ns "$ns" --wait=false

  # Wait a bit
  sleep 5

  # Check if still exists (likely stuck in Terminating)
  if kubectl get ns "$ns" >/dev/null 2>&1; then
    echo "⚠️ Namespace $ns stuck. Forcing delete..."

    kubectl get ns "$ns" -o json \
    | jq '.spec.finalizers=[]' \
    | kubectl replace --raw "/api/v1/namespaces/$ns/finalize" -f -

    echo "🔥 Force deleted: $ns"
  else
    echo "✅ Deleted normally: $ns"
  fi

  echo "----------------------------------"
done

echo "🎯 Cleanup complete!"
