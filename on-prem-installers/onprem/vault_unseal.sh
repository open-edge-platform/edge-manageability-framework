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

  max_wait=600
  waited=0
  interval=5

  # Wait for pod to reappear and become Running
  while true; do
    pod_exists=$(kubectl get pod "$pod_name" -n "$namespace" --no-headers 2>/dev/null || true)
    if [[ -z "$pod_exists" ]]; then
      echo "Pod '$pod_name' not found yet. Waiting $interval seconds..."
      sleep $interval
      waited=$((waited + interval))
      if (( waited >= max_wait )); then
        echo "Timed out waiting for pod '$pod_name' to appear after $max_wait seconds."
        return 1
      fi
      continue
    fi

    status=$(kubectl get pod "$pod_name" -n "$namespace" -o jsonpath='{.status.phase}' 2>/dev/null || true)
    if [[ "$status" == "Running" ]]; then
      echo "Pod '$pod_name' is Running."
      break
    else
      echo "Pod '$pod_name' status: $status. Waiting $interval seconds..."
      sleep $interval
      waited=$((waited + interval))
      if (( waited >= max_wait )); then
        echo "Timed out waiting for pod '$pod_name' to be Running after $max_wait seconds."
        return 1
      fi
    fi
  done

  echo "Fetching Vault unseal keys..."
  secret_json=$(kubectl -n "$namespace" get secret vault-keys -o jsonpath='{.data.vault-keys}' | base64 -d 2>/dev/null)
  if ! echo "$secret_json" | jq empty 2>/dev/null; then
    echo "Error: Decoded secret is not valid JSON. Secret 'vault-keys' may be missing or malformed."
    return 1
  fi

  keys=$(echo "$secret_json" | jq -r '.keys_base64[]?' 2>/dev/null)
  if [[ -z "$keys" ]]; then
    echo "Error: Failed to retrieve Vault unseal keys. 'keys_base64' field may be missing or malformed in secret 'vault-keys'."
    return 1
  fi

  echo "Unsealing Vault..."
  for key in $keys; do
    if ! kubectl -n "$namespace" exec -i "$pod_name" -- vault operator unseal "$key"; then
      echo "Error: Failed to unseal Vault with key: $key"
      return 1
    fi
  done

  echo "Waiting for Vault pod to be Ready..."
  if ! kubectl wait pod -n "$namespace" -l "app.kubernetes.io/name=vault" \
    --for=condition=Ready --timeout=300s; then
    echo "Timed out waiting for Vault pod to be Ready."
    return 1
  fi

  echo "Vault unsealed successfully."

  kubectl delete pod --ignore-not-found=true -n orch-platform platform-keycloak-0
}
