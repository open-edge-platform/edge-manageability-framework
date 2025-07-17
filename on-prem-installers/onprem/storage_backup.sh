#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Script Name: storage_backup.sh
# Description: This script:
#   1. Downloads and installs Velero for Kubernetes backup and restore.
#   2. Creates a credentials file for Velero.
#   3. Backs up specified namespaces in the Kubernetes cluster.
#   4. Restores the backup to the specified namespaces.
#   5. Enables or disables sync policies for specified applications.

# Usage: ./storage_backup.sh [install|backup|restore|enable-sync|disable-sync|cleanup]
#    install:               Download and install Velero
#    backup:                Backup specified namespaces
#    restore:               Restore backup to specified namespaces
#    enable-sync:           Enable sync policies for specified applications
#    disable-sync:          Disable sync policies for specified applications
#    cleanup:               Cleanup namespaces

VELERO_BIN="/usr/local/bin/velero"
CREDENTIALS_FILE="./credentials-velero"
MINIO_BUCKET="velero-backups"

# List of namespaces to backup
# Uncomment the namespaces you want to include in the backup
# If this list is empty, the script will attempt to populate it from existing PVCs
backup_namespace_list=(  
  #"orch-boots"
  "orch-database"
  #"gitea"
  "orch-platform"
  #"orch-app"
  #"orch-cluster"
  "orch-infra"
  #"orch-sre"
  #"orch-ui"
  #"orch-secret"
  #"orch-gateway"
  "orch-harbor"
  #"cattle-system"
)

# Function to get the namespace of the PVC that claims a given PV
get_pvc_namespace() {
  pv_name=$1
  pvc_info=$(kubectl get pv "$pv_name" -o json | jq -r '.spec.claimRef | select(. != null) | .namespace')
  
  if [ -n "$pvc_info" ]; then
    echo "$pvc_info"
  fi
}

get_backup_namespaces_from_pvs() {
    echo "Checking PVC claims for all PVs..."

    # Get all PV names
    pv_names=$(kubectl get pv -o json | jq -r '.items[].metadata.name')

    # Initialize an empty list to store namespaces
    namespace_list=""

    # Loop through each PV and find the namespace of the claiming PVC
    for pv_name in $pv_names; do
        namespace=$(get_pvc_namespace "$pv_name")
        if [ -n "$namespace" ]; then
            namespace_list+="$namespace"$'\n'
        fi
    done

    # Sort and remove duplicates, print as space-separated list
    # Convert the sorted, unique namespace list into an array
    readarray -t backup_namespace_list < <(echo "$namespace_list" | sort | uniq)
    echo "${backup_namespace_list[@]}"
}

disable_sync() {
    # Ensure the backup_namespace_list is populated
    if [ ${#backup_namespace_list[@]} -eq 0 ]; then
        echo "Backup namespace list is empty. Attempting to populate from PVCs."
        get_backup_namespaces_from_pvs
    fi

    # root-app is the root application that manages all other applications
    # This is disabled first to prevent sync issues during backup and restore
    kubectl patch application "root-app" -n onprem --type merge -p '{"spec":{"syncPolicy":null}}'

    for ns in "${backup_namespace_list[@]}"; do
        kubectl get applications -A -o yaml | yq ".items[] | select(.spec.destination.namespace == \"$ns\") | .metadata.name" | while read -r app; do
            echo "Disable sync on $app"
            kubectl patch application "$app" -n onprem --type merge -p '{"spec":{"syncPolicy":null}}'
        done
    done
}

enable_sync() {
    if [ ${#backup_namespace_list[@]} -eq 0 ]; then
        echo "Backup namespace list is empty. Attempting to populate from PVCs."
        get_backup_namespaces_from_pvs
    fi

    for ns in "${backup_namespace_list[@]}"; do
        kubectl get applications -A -o yaml | yq ".items[] | select(.spec.destination.namespace == \"$ns\") | .metadata.name" | while read -r app; do
            echo "Enabling sync on $app"
            kubectl patch application "$app" -n onprem --type merge -p '{"spec":{"syncPolicy":{"automated":{"prune":true,"selfHeal":true}}}}'
        done
    done

    # root-app is the root application that manages all other applications
    # This is enabled last to ensure all other applications are synced first
    kubectl patch application "$app" -n onprem --type merge -p '{"spec":{"syncPolicy":{"automated":{"prune":true,"selfHeal":true}}}}'
}

cleanup() {
    if [ ${#backup_namespace_list[@]} -eq 0 ]; then
        echo "Backup namespace list is empty. Attempting to populate from PVCs."
        get_backup_namespaces_from_pvs
    fi

    namespaces=$(IFS=, ; echo "${backup_namespace_list[*]}")
    echo "You are about to delete all resources in the following namespaces: $namespaces"
    read -p "Are you sure you want to proceed? (yes/no): " confirm
    if [[ "$confirm" != "yes" ]]; then
        echo "Cleanup aborted."
        return
    fi

    for ns in "${backup_namespace_list[@]}"; do
        kubectl delete all --all -n "$ns"
    done
}

create_credentials_file() {
    if [[ -f "$CREDENTIALS_FILE" ]]; then
        echo "Credentials file already exists. Skipping creation."
        return
    fi

    cat <<EOF > "$CREDENTIALS_FILE"
[default]
aws_access_key_id=admin
aws_secret_access_key=password
EOF
}

download_velero() {
    if [[ -d "$VELERO_DIR" ]]; then
        echo "Velero directory already exists. Skipping download."
        return
    fi

    if $VELERO_BIN --version &>/dev/null; then
        echo "Velero is already installed. Skipping download."
        return
    fi

    wget "https://github.com/vmware-tanzu/velero/releases/download/v1.16.1/velero-v1.16.1-linux-amd64.tar.gz"
    tar xvf velero-v1.16.1-linux-amd64.tar.gz
    sudo cp velero-v1.16.1-linux-amd64/velero "$VELERO_BIN"
}

install_velero() {
    if [[ -z ${MINIO_URL} ]]; then
        echo "MINIO_URL is not set. Please set the MINIO_URL environment variable."
        exit 1
    fi
    
    kubeconfig="${KUBECONFIG:-"/home/$USER/.kube/config"}"

    if [[ ! -f "$kubeconfig" ]]; then
        echo "Kubeconfig file not found at $kubeconfig. Please provide a valid kubeconfig."
        exit 1
    fi

    "$VELERO_BIN" install \
        --kubeconfig $kubeconfig \
        --provider aws \
        --plugins velero/velero-plugin-for-aws:v1.1.0 \
        --bucket "$MINIO_BUCKET" \
        --secret-file "$CREDENTIALS_FILE" \
        --use-volume-snapshots=false \
        --use-node-agent \
        --default-volumes-to-fs-backup \
        --backup-location-config region=minio,s3ForcePathStyle="true",s3Url="$MINIO_URL"
}

backup() {
    if [ ${#backup_namespace_list[@]} -eq 0 ]; then
        echo "Backup namespace list is empty. Attempting to populate from PVCs."
        get_backup_namespaces_from_pvs
    fi

    namespaces=$(IFS=, ; echo "${backup_namespace_list[*]}")
    "$VELERO_BIN" backup create "orch-backup" --include-namespaces "$namespaces" --wait
    "$VELERO_BIN" backup get
}

restore() {
    if [ ${#backup_namespace_list[@]} -eq 0 ]; then
        echo "Backup namespace list is empty. Attempting to populate from PVCs."
        get_backup_namespaces_from_pvs
    fi

    namespaces=$(IFS=, ; echo "${backup_namespace_list[*]}")
    "$VELERO_BIN" restore create --include-namespaces "$namespaces" --from-backup "orch-backup" --restore-volumes --wait
    "$VELERO_BIN" restore get
}

case "$1" in
    disable-sync)
        disable_sync
        ;;
    enable-sync)
        enable_sync
        ;;
    install)
        download_velero
        create_credentials_file
        install_velero
        ;;
    backup)
        backup
        ;;
    restore)
        restore
        ;;
    cleanup)
        cleanup
        ;;
    *)
        echo "Usage: ./storage_backup.sh {install|backup|restore|enable-sync|disable-sync|cleanup}"
        echo "Example: ./storage_backup.sh install"
        exit 1
        ;;
esac
