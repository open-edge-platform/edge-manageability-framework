#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

#####################################################################
### DO NOT INCLUDE IN 24.08 RELEASE - not fully verified & tested ###
#####################################################################

# Script Name: onprem_backup_restore.sh
# Description: Script responsible for restoring PVs backup back to cluster.
#              This script:
#               Takes path provided with -b option and restore ETCD from it,
#               Disables autoSync for all ArgoCD applications,
#               Scales deploy, sts and rs to 0 so backup can be safely restored,
#               Moves data from backup LVs to live PVs,
#               Re-enables autoSync for all ArgoCD applications so they can come back to desired state.
#

# Usage: ./onprem_backup_restore
#    -b:         rancher ETCD snapshot from which it should be restored (mandatory)
#    -d:         date of the backup to restore (mandatory)
#    -h:         help (optional)

set -e
set -o pipefail

usage() {
  cat >&2 <<EOF
Purpose:
Restore OnPrem Edge Orchestrator backup.

Usage:
$(basename "$0") [option...] [argument]

ex:
./onprem_backup_restore.sh -b /var/lib/rancher/rke2/server/db/snapshots/<snap-name> -d 2024-08-06-11_08
./onprem_backup_restore.sh -b /var/lib/rancher/rke2/server/db/snapshots/<snap-name>

Options:
    -b:          rancher ETCD snapshot from which it should be restored (mandatory)
    -d:          date of the backup to restore (mandatory), format: %Y-%m-%d-%H-%M e.g. 2024-08-06-11_08
    -h:          help (optional)
EOF
}

while getopts 'b:d:h' flag; do
  case "${flag}" in
  b) rke2_etcd_snap="$OPTARG" ;;
  d) lv_backup_date="$OPTARG" ;;
  h) HELP='true' ;;
  *) HELP='true' ;;
  esac
done

if [[ $HELP ]]; then
  usage
  exit 1
fi

if [[ -z $rke2_etcd_snap ]]; then
  echo "RKE2 ETCD backup location not provided, exiting"
  exit 1
fi

echo "Restoring Orchestrator from backup..."

# Restore rke2 ETCD backup
sudo systemctl stop rke2-server
sudo rke2 server --cluster-reset --cluster-reset-restore-path="$rke2_etcd_snap"
sudo systemctl start rke2-server

stop_sync_patch='{"spec":{"syncPolicy":null}}'
vg_name=lvmvg

# Restore PVs for each namespace one by one
namespaces=$(kubectl get namespaces -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}')
for ns in $namespaces; do
  # Find what applications are deployed in current namespace
  apps_in_ns=$(kubectl get applications.argoproj.io -n onprem --no-headers -o custom-columns=NAME:.metadata.name,NAMESPACE:.spec.destination.namespace | awk -v ns="$ns" '$2 == ns {print $1}')

  # Disable autoSync for ArgoCD applications so pods can be scaled down to zero for the time of backup restore
  declare -A sync_policy_map
  for app in $apps_in_ns; do
    echo "Disabling ArgoCD auto sync for application: $app"
    sync_policy_map[$app]=$(kubectl get application "$app" -n onprem -ojsonpath='{.spec.syncPolicy}' | sed 's/^/{"spec":{"syncPolicy":/' | sed 's/$/}}/')
    kubectl -n onprem patch application "$app" --type=merge -p "$stop_sync_patch"
  done

  echo "Scaling down all resources in namespace: $ns to 0 replicas..."

  deployments=$(kubectl get deployments -n "$ns" -o name)
  for deployment in $deployments; do
    kubectl scale --replicas=0 "$deployment" -n "$ns"
  done

  statefulsets=$(kubectl get statefulsets -n "$ns" -o name)
  for statefulset in $statefulsets; do
    kubectl scale --replicas=0 "$statefulset" -n "$ns"
  done

  replicasets=$(kubectl get replicasets -n "$ns" -o name)
  for replicaset in $replicasets; do
    kubectl scale --replicas=0 "$replicaset" -n "$ns"
  done

  echo "All resources scaled down to 0 replicas in namespace: $ns"

  # Restore PVs from backup
  pvs_to_restore=$(kubectl get pvc -n "$ns" -o jsonpath='{range .items[?(@.status.phase=="Bound")]}{.metadata.name}{" "}{.metadata.namespace}{" "}{.spec.volumeName}{"\n"}{end}')
  echo "$pvs_to_restore" | while IFS= read -r line; do
    read -r pvc_name pvc_namespace lv_name <<<"$line"
    if [[ $pvc_name == 'dkam-tink-shared-pvc' || $pvc_name == 'dkam-pvc' ]]; then continue; fi
    backup_name="${pvc_name}-{$pvc_namespace}-bak-${lv_backup_date}"

    # Create mounts for copying files from backup to live PV
    sudo mkdir -p /mnt/live-lv
    sudo mkdir -p /mnt/backup-lv

    # Copy data from original LV snapshot filesystem to backup LV filesystem
    sudo mount "/dev/$vg_name/$lv_name" /mnt/live-lv
    sudo mount "/dev/$vg_name/$backup_name" /mnt/backup-lv
    sudo cp -a /mnt/backup-lv/. /mnt/live-lv/
    sudo umount /mnt/live-lv
    sudo umount /mnt/backup-lv

    sudo rm -fr /mnt/live-lv
    sudo rm -fr /mnt/backup-lv
  done

  # Enable autoSync again so applications can scale back up to desired number of replicas
  for app in $apps_in_ns; do
    echo "${sync_policy_map[$app]}"
    kubectl -n onprem patch application "$app" --type=merge -p "${sync_policy_map[$app]}"
  done
done

echo "Orchestrator restored from backup..."
