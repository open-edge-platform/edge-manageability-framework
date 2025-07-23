#!/bin/bash
# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Description: Vault unseal
# Usage:
#   source ./vault_unseal.sh
#   vault_unseal


# Function to delete Vault pod and unseal it when it comes back
vault_unseal() {
  local namespace="orch-platform"
  local pod_name="vault-0"

  echo "Deleting Vault pod: $pod_name in namespace: $namespace"
  kubectl delete pod -n "$namespace" "$pod_name"
  echo "Waiting for pod '$pod_name' in namespace '$namespace' to be in Running state..."
  sleep 30

while true; do
  status=$(kubectl get pod "$pod_name" -n "$namespace" 2>/dev/null | grep Running)
  if [[ -n "$status" ]]; then
    echo "Pod '$pod_name' is Running."
    break
  else
    echo "Still waiting... checking again in 5 seconds."
    sleep 5
  fi
done

  echo "Fetching Vault unseal keys..."
  keys=$(kubectl -n "$namespace" get secret vault-keys -o jsonpath='{.data.vault-keys}' | base64 -d | jq '.keys_base64 | .[]' | sed 's/"//g')

  echo "Unsealing Vault..."
  for key in $keys; do
    kubectl -n "$namespace" exec -i "$pod_name" -- vault operator unseal "$key"
  done

  echo "Waiting for Vault pod to be Ready..."
  kubectl wait pod -n "$namespace" -l "app.kubernetes.io/name=vault" \
    --for=condition=Ready --timeout=300s

  echo "Vault unseal successfully."

  ## delete platform-keycloak
  kubectl delete pod --ignore-not-found=true -n orch-platform platform-keycloak-0

}
