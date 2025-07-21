#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Description:
#   This script is used after an upgrade to perform the following tasks:
#     - Restart the following key components:
#         • nexus-api-gw
#         • cluster-manager-template-controller
#         • app-orch-tenant-controller
#     - Delete old ClusterTemplates that do NOT contain "k3s" in their name
#
# Usage:
#   ./after_upgrade_restart.sh

# Function: delete pod and wait until it's Running and Ready
restart_and_wait_pod() {
  local namespace="$1"
  local pattern="$2"

  echo "🔍 Looking for pod matching '$pattern' in namespace '$namespace'..."

  # Find the pod name
  local pod_name
  pod_name=$(kubectl get pods -n "$namespace" | grep "$pattern" | awk '{print $1}')

  if [ -z "$pod_name" ]; then
    echo "❌ No pod found matching pattern '$pattern' in namespace '$namespace'"
    return 1
  fi

  echo "📌 Found pod: $pod_name. Deleting..."
  kubectl delete pod "$pod_name" -n "$namespace"
  kubectl wait deployment/"$pattern" -n "$namespace" --for=condition=Available --timeout=120s

}

# Function: Dlete Old Cluster Templates that do NOT contain 'k3s'
delete_old_template() {
echo "🔍 Fetching all ClusterTemplates..."
all_templates=$(kubectl get clustertemplate -A --no-headers)

echo "🚨 Deleting ClusterTemplates that do NOT contain 'k3s' in their name..."

# Loop through each line of the result
while IFS= read -r line; do
  namespace=$(echo "$line" | awk '{print $1}')
  template_name=$(echo "$line" | awk '{print $2}')

  # Check if the template name contains "k3s"
  if [[ "$template_name" != *k3s* ]]; then
    echo "❌ Deleting template '$template_name' in namespace '$namespace'"
    kubectl delete clustertemplate "$template_name" -n "$namespace"
  else
    echo "✅ Keeping template '$template_name' in namespace '$namespace' (contains 'k3s')"
  fi
done <<< "$all_templates"

echo "✅ Cleanup complete."
kubectl get clustertemplate -A
}
#restart pod after upgrade call:
restart_and_wait_pod "orch-iam" "nexus-api-gw"
restart_and_wait_pod "orch-cluster" "cluster-manager"
restart_and_wait_pod "orch-cluster" "cluster-manager-template-controller"
restart_and_wait_pod "orch-app" "app-orch-tenant-controller"
#delete old cluster template
delete_old_template
