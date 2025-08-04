#!/bin/bash
# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Description:
#   This script identifies all ArgoCD Applications in the specified namespace
#   that are NOT in Synced or Healthy state. It applies a sync patch to each,
#   applying 'root-app' last. Then it waits until all applications reach
#   Synced and Healthy state.
#
# Usage:
#   source ./sync-app-patch.sh
#
# Notes:
#   - Namespace is set via the NAMESPACE variable in the script.
#   - Patch is applied using a hard refresh annotation

# Set the ArgoCD application namespace
NAMESPACE="onprem"

# Patch file in current directory
PATCH_FILE="./sync-patch.yaml"

# Create the patch file
cat <<EOF > "$PATCH_FILE"
metadata:
  annotations:
    argocd.argoproj.io/refresh: "hard"
EOF

echo "📄 Patch file created: $PATCH_FILE"
echo "🔍 Searching for ArgoCD Applications in namespace '$NAMESPACE' that are NOT Synced and NOT Healthy..."

# Arrays to hold apps (normal and root-app separately)
declare -a normal_apps
declare -a root_app

# Collect apps into appropriate arrays
while IFS=$'\t' read -r appname; do
  if [[ "$appname" == "root-app" ]]; then
    root_app+=("$appname")
  else
    normal_apps+=("$appname")
  fi
done < <(kubectl get applications.argoproj.io -n "$NAMESPACE" -o json | jq -r '
  .items[] |
  select(.status.sync.status != "Synced" or .status.health.status != "Healthy") |
  .metadata.name
')

# Function to patch an app
patch_app() {
  app="$1"
  echo "⚙️  Applying patch to: $app (Namespace: $NAMESPACE)"
  if kubectl patch application "$app" -n "$NAMESPACE" --patch-file "$PATCH_FILE" --type merge >/dev/null 2>&1; then
    echo "✅ Patch applied to: $app"
  else
    echo "❌ Failed to patch: $app"
  fi
}

# Patch normal apps first
for app in "${normal_apps[@]}"; do
  patch_app "$app"
done

# Patch root-app last
for app in "${root_app[@]}"; do
  patch_app "$app"
done

# Wait loop for all apps to be Synced and Healthy
echo -e "\n⏳ Waiting for all applications in namespace '$NAMESPACE' to become Synced and Healthy..."
while true; do
  output=$(kubectl get applications.argoproj.io -n "$NAMESPACE" -o json | jq -r '
    .items[] |
    select(.status.sync.status != "Synced" or .status.health.status != "Healthy") |
    [.metadata.name, .status.sync.status, .status.health.status] |
    @tsv
  ')

  if [[ -z "$output" ]]; then
    echo "✅ All applications in namespace '$NAMESPACE' are now Synced and Healthy!"
    break
  else
    echo "⏳ The following applications are still not Synced and/or not Healthy:"
    echo -e "APPLICATION\tSYNC\tHEALTH"
    echo "$output" | column -t -s $'\t'
    echo "🔄 Waiting 10 seconds..."
    sleep 10
  fi
done

echo -e "\n All done — all applications are in Synced and Healthy state."
