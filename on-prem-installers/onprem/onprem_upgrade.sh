#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Script Name: onprem_upgrade.sh
# Description: This script:
#               If requested - does a backup of PVs and cluster's ETCD
#               Downloads debian packages and repo artifacts,
#               Upgrades packages to v3.1.0:
#                 - OS config,
#                 - RKE2 and basic cluster components,
#                 - ArgoCD,
#                 - Gitea,
#                 - Edge Orchestrator Applications

# Usage: ./onprem_upgrade
#    -o:             Override production values with dev values
#    -b:             enable backup of Orchestrator PVs before upgrade (optional)
#    -h:             help (optional)

set -e
set -o pipefail

# Setup logging - capture all output to log file while still displaying on console
LOG_FILE="onprem_upgrade_$(date +'%Y%m%d_%H%M%S').log"
LOG_DIR="/var/log/orch-upgrade"

# Create log directory if it doesn't exist
sudo mkdir -p "$LOG_DIR"
sudo chown "$(whoami):$(whoami)" "$LOG_DIR"

# Full path to log file
FULL_LOG_PATH="$LOG_DIR/$LOG_FILE"

# Function to log messages with timestamp
log_message() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*" | tee -a "$FULL_LOG_PATH"
}

# Function to log info messages
log_info() {
    log_message "INFO: $*"
}

# Function to log warning messages
log_warn() {
    log_message "WARN: $*"
}

# Function to log error messages
log_error() {
    log_message "ERROR: $*"
}

# Redirect all output to both console and log file
exec > >(tee -a "$FULL_LOG_PATH")
exec 2> >(tee -a "$FULL_LOG_PATH" >&2)

log_info "Starting OnPrem Edge Orchestrator upgrade script"
log_info "Log file: $FULL_LOG_PATH"

# Import shared functions
# shellcheck disable=SC1091
source "$(dirname "${0}")/functions.sh"
# shellcheck disable=SC1091
source "$(dirname "${0}")/upgrade_postgres.sh"
# shellcheck disable=SC1091
source "$(dirname "${0}")/vault_unseal.sh"
# shellcheck disable=SC1091
source "$(dirname "$0")/onprem.env"

### Constants
RELEASE_SERVICE_URL="${RELEASE_SERVICE_URL:-registry-rs.edgeorchestration.intel.com}"
ORCH_INSTALLER_PROFILE="${ORCH_INSTALLER_PROFILE:-onprem}"
DEPLOY_VERSION="${DEPLOY_VERSION:-v3.1.0}"  # Updated to v3.1.0
GITEA_IMAGE_REGISTRY="${GITEA_IMAGE_REGISTRY:-docker.io}"
USE_LOCAL_PACKAGES="${USE_LOCAL_PACKAGES:-false}"  # New flag for local packages
UPGRADE_3_1_X="${UPGRADE_3_1_X:-true}"

### Variables
cwd=$(pwd)

deb_dir_name="installers"
git_arch_name="repo_archives"
archives_rs_path="edge-orch/common/files/orchestrator"
installer_rs_path="edge-orch/common/files"
si_config_repo="edge-manageability-framework"
apps_ns=onprem
argo_cd_ns=argocd
gitea_ns=gitea
# shellcheck disable=SC2034
root_app=root-app


# Variables that depend on the above and might require updating later, are placed in here
set_artifacts_version() {
  installer_list=(
    "onprem-config-installer:${DEPLOY_VERSION}"
    "onprem-ke-installer:${DEPLOY_VERSION}"
    "onprem-argocd-installer:${DEPLOY_VERSION}"
    "onprem-gitea-installer:${DEPLOY_VERSION}"
    "onprem-orch-installer:${DEPLOY_VERSION}"
  )

  git_archive_list=(
    "onpremfull:${DEPLOY_VERSION}"
  )
}

export GIT_REPOS=$cwd/$git_arch_name
export ONPREM_UPGRADE_SYNC="${ONPREM_UPGRADE_SYNC:-false}"
retrieve_and_apply_config() {
    local config_file="$cwd/onprem.env"
    tmp_dir="$cwd/$git_arch_name/tmp"
    rm -rf "$tmp_dir"
    mkdir -p "$tmp_dir"

    ## Untar edge-manageability-framework repo
    repo_file=$(find "$cwd/$git_arch_name" -name "*$si_config_repo*.tgz" -type f -printf "%f\n")
    tar -xf "$cwd/$git_arch_name/$repo_file" -C "$tmp_dir"

    # Get the external IP address of the LoadBalancer services
    ARGO_IP=$(kubectl get svc argocd-server -n argocd -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
    TRAEFIK_IP=$(kubectl get svc traefik -n orch-gateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
    NGINX_IP=$(kubectl get svc ingress-nginx-controller -n orch-boots -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

    update_config_variable "$config_file" "ARGO_IP" "${ARGO_IP}"
    update_config_variable "$config_file" "TRAEFIK_IP" "${TRAEFIK_IP}"
    update_config_variable "$config_file" "NGINX_IP" "${NGINX_IP}"

    sre_tls=$(kubectl get applications -n "$apps_ns" sre-exporter -o jsonpath='{.spec.sources[*].helm.valuesObject.otelCollector.tls.enabled}')
    if [[ $sre_tls = 'true' ]]; then
        update_config_variable "$config_file" "SRE_TLS_ENABLED" "${sre_tls}"
        sre_dest_ca_cert=$(kubectl get applications -n "$apps_ns" sre-exporter -o jsonpath='{.spec.sources[*].helm.valuesObject.otelCollector.tls.caSecret.enabled}')
        if [[ "${sre_dest_ca_cert}" == "true" ]]; then
            update_config_variable "$config_file" "SRE_DEST_CA_CERT" "${sre_dest_ca_cert}"
        fi
    else
        update_config_variable "$config_file" "SRE_TLS_ENABLED" "false"
    fi

    VALUE_FILES=$(kubectl get application root-app -n "$apps_ns" -o jsonpath='{.spec.sources[0].helm.valueFiles[*]}')

    if [[ -z "$VALUE_FILES" ]]; then
        echo "⚠️  Warning: No value files found in root-app spec"
        exit 1
    fi

    echo "Found value files:"
    echo "$VALUE_FILES" | tr ' ' '\n' | sed 's/^/  - /'
    echo

    # Initialize all profiles as disabled (true means profile is disabled)
    DISABLE_CO_PROFILE="false"
    DISABLE_AO_PROFILE="false"
    DISABLE_O11Y_PROFILE="false"
    SINGLE_TENANCY_PROFILE="false"

    # Check for enabled profiles (inverse logic)
    # If we find enable-cluster-orch.yaml, then CO is enabled, so DISABLE_CO=false
    if echo "$VALUE_FILES" | grep -q "enable-cluster-orch.yaml"; then
        echo "✅ Cluster Orchestrator (CO) profile is ENABLED"
    else
        DISABLE_CO_PROFILE="true"
        echo "⛔ Cluster Orchestrator (CO) profile is DISABLED"
    fi

    if echo "$VALUE_FILES" | grep -q "enable-app-orch.yaml"; then
        echo "✅ Application Orchestrator (AO) profile is ENABLED"
    else
        DISABLE_AO_PROFILE="true"
        echo "⛔ Application Orchestrator (AO) profile is DISABLED"
    fi

    if echo "$VALUE_FILES" | grep -qE "(enable-o11y\.yaml|o11y-onprem-1k\.yaml)"; then
        echo "✅ Observability (O11y) profile is ENABLED"
    else
        DISABLE_O11Y_PROFILE="true"
        echo "⛔ Observability (O11y) profile is DISABLED"
    fi

    if echo "$VALUE_FILES" | grep -q "enable-singleTenancy.yaml"; then
        SINGLE_TENANCY_PROFILE="true"
        echo "✅ Single Tenancy is ENABLED"
    else
        echo "⛔ Single Tenancy is DISABLED"
    fi

    update_config_variable "$config_file" "DISABLE_CO_PROFILE" "$DISABLE_CO_PROFILE"
    update_config_variable "$config_file" "DISABLE_AO_PROFILE" "$DISABLE_AO_PROFILE"
    update_config_variable "$config_file" "DISABLE_O11Y_PROFILE" "$DISABLE_O11Y_PROFILE"
    update_config_variable "$config_file" "SINGLE_TENANCY_PROFILE" "$SINGLE_TENANCY_PROFILE"

    # Get SMTP_SKIP_VERIFY from alerting-monitor application
    SMTP_SKIP_VERIFY=$(kubectl get application alerting-monitor -n "$apps_ns" -o jsonpath='{.spec.sources[*].helm.valuesObject.alertingMonitor.smtp.insecureSkipVerify}' 2>/dev/null || echo "false")
    if [[ -n "$SMTP_SKIP_VERIFY" ]]; then
        update_config_variable "$config_file" "SMTP_SKIP_VERIFY" "$SMTP_SKIP_VERIFY"
    fi

    #cleanup old file
    rm -rf "$ORCH_INSTALLER_PROFILE".yaml

    # Generate Cluster Config
    ./generate_cluster_yaml.sh onprem

    # cp changes to tmp repo
    tmp_dir="$cwd/$git_arch_name/tmp"
    cp "$ORCH_INSTALLER_PROFILE".yaml "$tmp_dir"/$si_config_repo/orch-configs/clusters/"$ORCH_INSTALLER_PROFILE".yaml

    while true; do
        if [[ -n ${PROCEED} ]]; then
            break
        fi
        read -rp "Edit config values.yaml files with custom configurations if necessary!!!
    The files are located at:
    $tmp_dir/$si_config_repo/orch-configs/profiles/<profile>.yaml
    $tmp_dir/$si_config_repo/orch-configs/clusters/$ORCH_INSTALLER_PROFILE.yaml
    Enter 'yes' to confirm that configuration is done in order to progress with installation
    ('no' will exit the script) !!!

    Ready to proceed with installation? " yn
        case $yn in
            [Yy]* ) break;;
            [Nn]* ) exit 1;;
            * ) echo "Please answer yes or no.";;
        esac
    done

    ## Tar back the repo
    cd "$tmp_dir"
    tar -zcvf "$repo_file" ./edge-manageability-framework
    mv -f "$repo_file" "$cwd/$git_arch_name/$repo_file"
    cd "$cwd"
    rm -rf "$tmp_dir"
}

resync_all_apps() {
    # Re-create the patch file for ArgoCD sync operation if it doesn't exist
    if [[ ! -f /tmp/argo-cd/sync-patch.yaml ]]; then
        sudo mkdir -p /tmp/argo-cd
        cat <<EOF | sudo tee /tmp/argo-cd/sync-patch.yaml >/dev/null
operation:
  sync:
    syncStrategy:
      hook: {}
EOF
fi
    kubectl patch -n "$apps_ns" application postgresql-secrets --patch-file /tmp/argo-cd/sync-patch.yaml --type merge
    kubectl patch -n "$apps_ns" application root-app --patch-file /tmp/argo-cd/sync-patch.yaml --type merge
}

terminate_existing_sync() {
    local app_name=$1
    local namespace=$2

    local current_phase
    current_phase=$(kubectl get application "$app_name" -n "$namespace" -o jsonpath='{.status.operationState.phase}' 2>/dev/null)

    if [[ "$current_phase" == "Running" ]]; then
        echo "🛑 Terminating existing sync operation..."
        kubectl patch application "$app_name" -n "$namespace" --type='merge' -p='{"operation": null}'

        # Wait for termination
        timeout 30 bash -c "while [[ \"\$(kubectl get application $app_name -n $namespace -o jsonpath='{.status.operationState.phase}' 2>/dev/null)\" == \"Running\" ]]; do sleep 2; done"
        echo "✅ Existing operation terminated"
    else
        echo "ℹ️  No running operation to terminate"
    fi
}

force_sync_outofsync_app() {
    local app_name=$1
    local namespace=$2
    local server_side_apply=${3:-false}  # Default to false if not specified

    set +e
    terminate_existing_sync "$app_name" "$namespace"
    echo "Force syncing $app_name (ServerSideApply=$server_side_apply)..."

    kubectl patch -n "$namespace" application "$app_name" --type merge --patch "$(cat <<EOF
{
    "operation": {
        "initiatedBy": {
            "username": "admin"
        },
        "sync": {
            "syncOptions": [
                "Replace=true",
                "Force=true",
                "ServerSideApply=$server_side_apply"
            ]
        }
    }
}
EOF
)"
    set -e
}

# Function to check and force sync application if not healthy
check_and_force_sync_app() {
    local app_name=$1
    local namespace=$2
    local server_side_apply=${3:-false}  # Default to false if not specified
    local max_retries=2

    for ((i=1; i<=max_retries; i++)); do
        app_status=$(kubectl get application "$app_name" -n "$namespace" -o jsonpath='{.status.sync.status} {.status.health.status}' 2>/dev/null || echo "NotFound NotFound")

        if [[ "$app_status" == "Synced Healthy" ]]; then
            echo "✅ $app_name is Synced and Healthy"
            return 0
        fi

        echo "⚠️  $app_name is not Synced and Healthy (status: $app_status). Force-syncing... (attempt $i/$max_retries)"
        force_sync_outofsync_app "$app_name" "$namespace" "$server_side_apply"
        echo "✅ $app_name sync triggered"

        # Check status every 5s for 90s
        local check_timeout=90
        local check_interval=3
        local elapsed=0

        while (( elapsed < check_timeout )); do
            app_status=$(kubectl get application "$app_name" -n "$namespace" -o jsonpath='{.status.sync.status} {.status.health.status}' 2>/dev/null || echo "NotFound NotFound")

            if [[ "$app_status" == "Synced Healthy" ]]; then
                echo "✅ $app_name became Synced and Healthy"
                return 0
            else
                echo "Current status: $app_status (elapsed: ${elapsed}s)"
            fi

            sleep $check_interval
            elapsed=$((elapsed + check_interval))
        done

        echo "⏳ $app_name did not become healthy within ${check_timeout}s"
    done

    echo "⚠️  $app_name may still require attention after $max_retries attempts"
}

check_and_patch_sync_app() {
    local app_name=$1
    local namespace=$2
    local max_retries=2

    for ((i=1; i<=max_retries; i++)); do
        app_status=$(kubectl get application "$app_name" -n "$namespace" -o jsonpath='{.status.sync.status} {.status.health.status}' 2>/dev/null || echo "NotFound NotFound")

        if [[ "$app_status" == "Synced Healthy" ]]; then
            echo "✅ $app_name is Synced and Healthy"
            return 0
        fi

        echo "⚠️  $app_name is not Synced and Healthy (status: $app_status). Force-syncing... (attempt $i/$max_retries)"

        set +e
        terminate_existing_sync "$app_name" "$namespace"
        kubectl patch -n "$namespace" application "$app_name" --patch-file /tmp/argo-cd/sync-patch.yaml --type merge
        set -e

        # Check status every 5s for 90s
        local check_timeout=90
        local check_interval=3
        local elapsed=0

        while (( elapsed < check_timeout )); do
            app_status=$(kubectl get application "$app_name" -n "$namespace" -o jsonpath='{.status.sync.status} {.status.health.status}' 2>/dev/null || echo "NotFound NotFound")

            if [[ "$app_status" == "Synced Healthy" ]]; then
                echo "✅ $app_name became Synced and Healthy"
                return 0
            else
                echo "Current status: $app_status (elapsed: ${elapsed}s)"
            fi

            sleep $check_interval
            elapsed=$((elapsed + check_interval))
        done

        echo "⏳ $app_name did not become healthy within ${check_timeout}s"
    done

    echo "⚠️  $app_name may still require attention after $max_retries attempts"
}

# Function to wait for application to be Synced and Healthy with timeout
wait_for_app_synced_healthy() {
    local app_name=$1
    local namespace=$2
    local timeout=${3:-120}  # Default 120 seconds if not specified

    local start_time
    start_time=$(date +%s)
    set +e
    while true; do
        echo "Checking $app_name application status..."
        local app_status
        app_status=$(kubectl get application "$app_name" -n "$namespace" -o jsonpath='{.status.sync.status} {.status.health.status}' 2>/dev/null || echo "NotFound NotFound")
        if [[ "$app_status" == "Synced Healthy" ]]; then
            echo "✅ $app_name application is Synced and Healthy."
            set -e
            return 0
        fi
        local current_time
        current_time=$(date +%s)
        local elapsed=$((current_time - start_time))
        if (( elapsed > timeout )); then
            echo "⚠️ Timeout waiting for $app_name to be Synced and Healthy after ${timeout}s (status: $app_status)"
            set -e
            return 0
        fi
        echo "Waiting for $app_name to be Synced and Healthy... (status: $app_status, ${elapsed}s/${timeout}s elapsed)"
        sleep 3
    done
}

# Function to restart a StatefulSet by scaling to 0 and back
restart_statefulset() {
    local name=$1
    local namespace=$2

    echo "Restarting StatefulSet $name in namespace $namespace..."

    # Get current replica count
    REPLICAS=$(kubectl get statefulset "$name" -n "$namespace" -o jsonpath='{.spec.replicas}')
    echo "Current replicas: $REPLICAS"

    # Scale to 0
    kubectl scale statefulset "$name" -n "$namespace" --replicas=0

    # Wait for pods to terminate
    kubectl wait --for=delete pod -l app="$name" -n "$namespace" --timeout=300s

    # Scale back to original replica count
    kubectl scale statefulset "$name" -n "$namespace" --replicas="$REPLICAS"

    echo "✅ $name restarted"
}


# Function to check app status and clean up job if needed
check_and_cleanup_job() {
    local app_name=$1
    local namespace=$2
    local job_label=${3:-job-name}

    app_status=$(kubectl get application "$app_name" -n "$apps_ns" -o jsonpath='{.status.sync.status} {.status.health.status}' 2>/dev/null || echo "NotFound NotFound")
    if [[ "$app_status" != "Synced Healthy" ]]; then
        if kubectl get job -n "$namespace" -l "$job_label" 2>/dev/null | grep "$app_name"; then
            echo "Deleting $app_name job..."
            job_name=$(kubectl get job -n "$namespace" -l "$job_label" | grep "$app_name" | awk '{print $1}')
            kubectl delete job "$job_name" -n "$namespace" --force --grace-period=0 --ignore-not-found
            echo "✅ $app_name job deleted"
            kubectl patch -n "$apps_ns" application "$app_name" --patch-file /tmp/argo-cd/sync-patch.yaml --type merge
        else
            echo "ℹ️  No $app_name job found to delete"
        fi
    fi
}

# Checks if orchestrator is currently installed on the node
# check_orch_install <array[@] of package names>
check_orch_install() {
    package_list=("$@")
    for package in "${package_list[@]}"; do
        package_name="${package%%:*}"
        if ! dpkg -l "$package_name" >/dev/null 2>&1; then
            echo "Package: $package_name is not installed on the node, OnPrem Edge Orchestrator is not installed or installation is broken"
            exit 1
        fi

        # shellcheck disable=SC2034
        installed_ver=$(dpkg-query -W -f='${Version}' "$package_name")
        incoming_ver="${package#*:}"
        incoming_ver="${incoming_ver#v}"
        # if [[ $installed_ver = "$incoming_ver" ]]; then
        #     echo "Package: $package_name is already at version: $incoming_ver"
        #     exit 1
        # fi
    done
}

# Get LV size and format it to be ready for lvcreate command
# get_lv_size <lv_path> returns <formatted size>
get_lv_size() {
    lv_path=$1

    size_output=$(sudo lvdisplay "$lv_path" | grep "LV Size" | awk '{print $3, $4}')
    size=$(echo "$size_output" | awk '{print $1}')
    unit=$(echo "$size_output" | awk '{print $2}')

    case $unit in
    GiB)
        formatted_size="${size}G"
        ;;
    MiB)
        formatted_size="${size}M"
        ;;
    TiB)
        formatted_size="${size}T"
        ;;
    *)
        echo "Error: Unsupported unit $unit."
        exit 1
        ;;
    esac

    echo "$formatted_size"
}

check_space_for_backup() {
    vg_info=$(sudo vgs --noheadings --units g --nosuffix -o vg_size,vg_free 2>/dev/null)
    vsize=$(echo "$vg_info" | awk '{print $1}')
    vfree=$(echo "$vg_info" | awk '{print $2}')
    vused=$(echo "$vsize - $vfree" | bc)

    margin=$(echo "$vused * 0.05" | bc)
    enough_space=$(echo "$vfree > ($vused + $margin)" | bc)

    echo "$enough_space"
}

# Backup all PVs to LVs in the same VG. They won't get deleted during orchestrator removal from node.
backup_pvs() {
    space_check_result=$(check_space_for_backup)
    if [[ $space_check_result -eq 0 ]]; then
        echo "Error: there is not enough space for PVs backup in VG"
        return 1
    fi

    vg_name=lvmvg
    vol_snap_class_name=openebs-lvm-vsc
    backup_date=$(date +'%Y-%m-%d-%H_%M')

    pvs_to_backup=$(kubectl get pvc --all-namespaces -o jsonpath='{range .items[?(@.status.phase=="Bound")]}{.metadata.name}{" "}{.metadata.namespace}{" "}{.spec.volumeName}{"\n"}{end}')
    echo "$pvs_to_backup" | while IFS= read -r line; do
        read -r pvc_name pvc_namespace lv_name <<<"$line"

        # dkam-pvc doesn't need backup as its data will get re-populated anyway
        if [[ $pvc_name == 'dkam-pvc' ]]; then
            continue
        fi

        # Create VolumeSnapshot and use it in backup for data consistency
        kubectl apply -f - <<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: $pvc_name-snap
  namespace: $pvc_namespace
spec:
  volumeSnapshotClassName: $vol_snap_class_name
  source:
    persistentVolumeClaimName: $pvc_name
EOF

        attempts=0
        max_attempts=40
        while [ "$(kubectl get volumesnapshot -n "$pvc_namespace" "$pvc_name-snap" -o jsonpath='{.status.readyToUse}')" != "true" ]; do
            echo "Waiting for VolumeSnaphot $pvc_name-snap in $pvc_namespace to be readyToUse..."
            sleep 5
            attempts=$((attempts + 1))

            if [ $attempts -ge $max_attempts ]; then
            echo "Reached maximum number of attempts ($max_attempts), stopping upgrade operation"
            exit 1
            fi
        done

        # Create backup LV on VG
        formatted_size=$(get_lv_size "/dev/$vg_name/$lv_name")
        bak_lv_name="${pvc_name}-${pvc_namespace}-bak-${backup_date}"

        sudo lvcreate -n "$bak_lv_name" -L "$formatted_size" $vg_name -y
        sudo mkfs.ext4 "/dev/$vg_name/$bak_lv_name"

        # Create mounts for copying files from original LV to backup
        sudo mkdir -p /mnt/original-lv
        sudo mkdir -p /mnt/backup-lv

        # Copy data from original LV snapshot filesystem to backup LV filesystem
        snap_name=$(sudo lvs --options lv_name,origin --noheadings | grep "$lv_name" | awk -v lv_name="$lv_name" '$1 != lv_name {print $1}')
        sudo mount "/dev/$vg_name/$snap_name" /mnt/original-lv
        sudo mount "/dev/$vg_name/$bak_lv_name" /mnt/backup-lv
        sudo cp -a /mnt/original-lv/. /mnt/backup-lv/
        sudo umount "/dev/$vg_name/$snap_name"
        sudo umount "/dev/$vg_name/$bak_lv_name"

        sudo rm -fr /mnt/original-lv
        sudo rm -fr /mnt/backup-lv

        # Delete VolumeSnapshot as data there is backed up already
        kubectl delete volumesnapshot -n "$pvc_namespace" "$pvc_name-snap"
    done
}

# Function to search and delete specific secrets in the 'gitea' namespace
cleanup_gitea_secrets() {
  echo "Checking for secrets in namespace: gitea"

  local secrets=(
    "gitea-apporch-token"
    "gitea-argocd-token"
    "gitea-clusterorch-token"
  )

  for secret in "${secrets[@]}"; do
    if kubectl get secret "$secret" -n gitea >/dev/null 2>&1; then
      echo "Secret found: $secret - Deleting..."
      kubectl delete secret "$secret" -n gitea
    else
      echo "Secret not found: $secret"
    fi
  done

  echo "Secret cleanup completed."
}

usage() {
    cat >&2 <<EOF
Purpose:
Upgrade OnPrem Edge Orchestrator to v3.1.0.

Usage:
$(basename "$0") [option...] [argument]

ex:
./onprem_upgrade.sh -b
./onprem_upgrade.sh -bl  # Use local packages with backup

Options:
    -b:             enable backup of Orchestrator PVs before upgrade (optional)
    -l:             use local packages instead of downloading (optional)
    -o:             override production values with dev values (optional)
    -h:             help (optional)

EOF
}

################################
##### UPGRADE SCRIPT START #####
################################

# shellcheck disable=SC2034
while getopts 'v:hbol' flag; do
    case "${flag}" in
    h) HELP='true' ;;
    b) BACKUP='true' ;;
    o) OVERRIDE='true' ;;
    l) USE_LOCAL_PACKAGES='true' ;;  # New local packages flag
    *) HELP='true' ;;
    esac
done

if [[ $HELP ]]; then
    usage
    exit 1
fi


# Check if postgres is running and if it is safe to backup
check_postgres
if ! check_postgres; then
    echo "PostgreSQL is not running or backup file already exists. Exiting..."
    exit 1
fi

# Perform PostgreSQL secret backup if not done already
if [[ ! -f postgres_secret.yaml ]]; then
    if [[ "$UPGRADE_3_1_X" == "true" ]]; then
        kubectl get secret -n orch-database postgresql -o yaml > postgres_secret.yaml
    else
        kubectl get secret -n orch-database passwords -o yaml > postgres_secret.yaml
    fi
fi


# Delete gitea secrets before backup
cleanup_gitea_secrets

# Backup PostgreSQL databases
backup_postgres


echo "Running On Premise Edge Orchestrator upgrade to $DEPLOY_VERSION"

# Refresh variables after checking user args
set_artifacts_version

# Check if orchestrator is currently installed
check_orch_install "${installer_list[@]}"

# Check & install script dependencies
check_oras
install_yq

### Backup

if [[ $BACKUP ]]; then
    echo "Backing up PVs..."
    backup_pvs
    if [[ $? -eq 1 ]]; then
        exit 1
    fi
    echo "PVs backed up successfully"

    # Take RKE2 backup (etcd) -> https://docs.rke2.io/backup_restore
    echo "Taking rke2 snapshot..."
    sudo rke2 etcd-snapshot save --name "pre-upgrade-snapshot-$(dpkg-query -W -f='${Version}' onprem-ke-installer)"
    sudo mkdir -p /var/orch-backups/
    sudo find /var/lib/rancher/rke2/server/db/snapshots/ -name "pre-upgrade-snapshot-*" -exec mv {} /var/orch-backups/ \;
    echo "Snapshot saved to /var/orch-backups/"
fi

# Skip artifact download if using local packages
if [[ $USE_LOCAL_PACKAGES != "true" ]]; then
    # Cleanup and download .deb packages
    sudo rm -rf "${cwd:?}/${deb_dir_name:?}/"
    download_artifacts "$cwd" "$deb_dir_name" "$RELEASE_SERVICE_URL" "$installer_rs_path" "${installer_list[@]}"
    sudo chown -R _apt:root "$deb_dir_name"

    # Cleanup and download .git packages
    sudo rm -rf "${cwd:?}/${git_arch_name:?}/"
    download_artifacts "$cwd" "$git_arch_name" "$RELEASE_SERVICE_URL" "$archives_rs_path" "${git_archive_list[@]}"
else
    echo "Using local packages..."

    # Ensure local directories exist with required files
    if [[ ! -d "$deb_dir_name" ]]; then
        echo "Error: Local $deb_dir_name directory not found!"
        echo "Please place your .deb files in: $cwd/$deb_dir_name/"
        exit 1
    fi

    if [[ ! -d "$git_arch_name" ]]; then
        echo "Error: Local $git_arch_name directory not found!"
        echo "Please place your onpremFull_edge-manageability-framework_3.1.0-dev.tgz in: $cwd/$git_arch_name/"
        exit 1
    fi

    # Verify required .deb files exist
    for package in "${installer_list[@]}"; do
        package_name="${package%%:*}"
        if ! ls "$cwd/$deb_dir_name/${package_name}"_*_amd64.deb 1> /dev/null 2>&1; then
            echo "Error: ${package_name} .deb file not found in $cwd/$deb_dir_name/"
            exit 1
        fi
    done

    # Verify .tgz file exists
    if ! ls "$cwd/$git_arch_name/"*edge-manageability-framework*.tgz 1> /dev/null 2>&1; then
        echo "Error: edge-manageability-framework .tgz file not found in $cwd/$git_arch_name/"
        exit 1
    fi

    sudo chown -R _apt:root "$deb_dir_name"
fi

# Retrieve config that was set during onprem installation and apply it to orch-configs
# Modify orch-configs settings for upgrade procedure
retrieve_and_apply_config

# Check if kyverno-clean-reports job exists before attempting cleanup
if kubectl get job kyverno-clean-reports -n kyverno >/dev/null 2>&1; then
    echo "Cleaning up kyverno-clean-reports job..."
    kubectl delete job kyverno-clean-reports -n kyverno &
    kubectl delete pods -l job-name="kyverno-clean-reports" -n kyverno &
    kubectl patch job kyverno-clean-reports -n kyverno --type=merge -p='{"metadata":{"finalizers":[]}}'
else
    echo "kyverno-clean-reports job not found in kyverno namespace, skipping cleanup"
fi

### Upgrade

# Run OS Configuration upgrade
echo "Upgrading the OS level configuration..."
eval "sudo DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=l apt-get install --only-upgrade --allow-downgrades -y $cwd/$deb_dir_name/onprem-config-installer_*_amd64.deb"
echo "OS level configuration upgraded to $(dpkg-query -W -f='${Version}' onprem-config-installer)"

# Run RKE2 upgrade
echo "Upgrading RKE2..."
eval "sudo DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=l apt-get install --only-upgrade --allow-downgrades -y $cwd/$deb_dir_name/onprem-ke-installer_*_amd64.deb"
echo "RKE2 upgraded to $(dpkg-query -W -f='${Version}' onprem-ke-installer)"

# Run Gitea upgrade
echo "Upgrading Gitea..."
eval "sudo IMAGE_REGISTRY=${GITEA_IMAGE_REGISTRY} DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=l apt-get install --only-upgrade --allow-downgrades -y $cwd/$deb_dir_name/onprem-gitea-installer_*_amd64.deb"
wait_for_pods_running $gitea_ns
echo "Gitea upgraded to $(dpkg-query -W -f='${Version}' onprem-gitea-installer)"

# Run ArgoCD upgrade
echo "Upgrading ArgoCD..."
eval "sudo DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=l apt-get install --only-upgrade --allow-downgrades -y $cwd/$deb_dir_name/onprem-argocd-installer_*_amd64.deb"
wait_for_pods_running $argo_cd_ns
echo "ArgoCD upgraded to $(dpkg-query -W -f='${Version}' onprem-argocd-installer)"

# Run Orchestrator upgrade
echo "Upgrading Edge Orchestrator Packages..."

# Skip saving passwords if postgres-secrets-password.txt exists and is not empty
if [[ ! -s postgres-secrets-password.txt ]]; then
    ALERTING=$(kubectl get secret alerting-local-postgresql -n orch-infra -o jsonpath='{.data.PGPASSWORD}')
    CATALOG_SERVICE=$(kubectl get secret app-orch-catalog-local-postgresql -n orch-app -o jsonpath='{.data.PGPASSWORD}')
    INVENTORY=$(kubectl get secret inventory-local-postgresql -n orch-infra -o jsonpath='{.data.PGPASSWORD}')
    IAM_TENANCY=$(kubectl get secret iam-tenancy-local-postgresql -n orch-iam -o jsonpath='{.data.PGPASSWORD}')
    PLATFORM_KEYCLOAK=$(kubectl get secret platform-keycloak-local-postgresql -n orch-platform -o jsonpath='{.data.PGPASSWORD}')
    VAULT=$(kubectl get secret vault-local-postgresql -n orch-platform -o jsonpath='{.data.PGPASSWORD}')
    if [[ "$UPGRADE_3_1_X" == "true" ]]; then
        POSTGRESQL=$(kubectl get secret postgresql -n orch-database -o jsonpath='{.data.postgres-password}')
    else
        POSTGRESQL=$(kubectl get secret orch-database-postgresql -n orch-database -o jsonpath='{.data.password}')
    fi
    MPS=$(kubectl get secret mps-local-postgresql -n orch-infra -o jsonpath='{.data.PGPASSWORD}')
    RPS=$(kubectl get secret rps-local-postgresql -n orch-infra -o jsonpath='{.data.PGPASSWORD}')
    {
        echo "Alerting: $ALERTING"
        echo "CatalogService: $CATALOG_SERVICE"
        echo "Inventory: $INVENTORY"
        echo "IAMTenancy: $IAM_TENANCY"
        echo "PlatformKeycloak: $PLATFORM_KEYCLOAK"
        echo "Vault: $VAULT"
        echo "PostgreSQL: $POSTGRESQL"
        echo "Mps: $MPS"
        echo "Rps: $RPS"
    } > postgres-secrets-password.txt
else
    echo "postgres-secrets-password.txt exists and is not empty, skipping password save."
fi


# Delete secrets for mps and rps if they exist, so that they can be recreated later
if kubectl get secret mps -n orch-infra >/dev/null 2>&1; then
    kubectl get secret mps -n orch-infra -o yaml > mps_secret.yaml
    kubectl delete secret mps -n orch-infra
fi

if kubectl get secret rps -n orch-infra >/dev/null 2>&1; then
    kubectl get secret rps -n orch-infra -o yaml > rps_secret.yaml
    kubectl delete secret rps -n orch-infra
fi


# Idea is the same as in postrm_patch but for orch-installer whole new script is required
sudo tee /var/lib/dpkg/info/onprem-orch-installer.postrm >/dev/null <<'EOF'
#!/usr/bin/env bash

set -o errexit

export KUBECONFIG=/home/$USER/.kube/config
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/games:/usr/local/games:/snap/bin

kubectl delete job -n gitea -l managed-by=edge-manageability-framework || true
kubectl delete sts -n orch-database postgresql || true
kubectl delete job -n orch-infra credentials || true
kubectl delete job -n orch-infra loca-credentials || true

if [ "${1}" = "upgrade" ]; then
    exit 0
fi

# Secrets for postgresql are generated on each installation, so we have to clean them up to avoid issues during reinstallation
kubectl delete secret -l managed-by=edge-manageability-framework -A || true

EOF

eval "sudo DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=l ORCH_INSTALLER_PROFILE=$ORCH_INSTALLER_PROFILE GIT_REPOS=$GIT_REPOS apt-get install --only-upgrade --allow-downgrades -y $cwd/$deb_dir_name/onprem-orch-installer_*_amd64.deb"
echo "Edge Orchestrator getting upgraded to version $(dpkg-query -W -f='${Version}' onprem-orch-installer), wait for SW to deploy... "

# Allow adjustments as some PVCs sizes might have changed
#kubectl patch storageclass openebs-lvmpv -p '{"allowVolumeExpansion": true}'

# Delete rke2-metrics-server chart. If it fails ignore
helm delete -n kube-system rke2-metrics-server || true

resync_all_apps

# Restore PostgreSQL passwords after they have been overwritten
set +e
while true; do
    if kubectl get secret -n orch-app app-orch-catalog-reader-local-postgresql; then
        echo "Proceeding with passwords restoration..."
        sleep 10
        break
    else
        echo "Passwords not yet overwritten, waiting..."
        sleep 10
    fi
done
set -e

patch_secrets() {

    # Patch secrets with passwords from postgres-secrets-password.txt
    # If the file is not empty, read the passwords and patch the secrets accordingly
    if [[ -s postgres-secrets-password.txt ]]; then
        echo "Patching secrets with passwords from postgres-secrets-password.txt"
        while IFS=': ' read -r key value; do
            case "$key" in
                Alerting) ALERTING="$value" ;;
                CatalogService) CATALOG_SERVICE="$value" ;;
                Inventory) INVENTORY="$value" ;;
                IAMTenancy) IAM_TENANCY="$value" ;;
                PlatformKeycloak) PLATFORM_KEYCLOAK="$value" ;;
                Vault) VAULT="$value" ;;
                PostgreSQL) POSTGRESQL="$value" ;;
                Mps) MPS="$value" ;;
                Rps) RPS="$value" ;;
            esac
        done < postgres-secrets-password.txt
    fi

    wait_for_app_synced_healthy postgresql-secrets "$apps_ns"

    check_and_patch_sync_app postgresql-secrets "$apps_ns"

    wait_for_app_synced_healthy postgresql-secrets "$apps_ns"

    # Check if postgresql-secrets is Synced and Healthy
    app_status=$(kubectl get application postgresql-secrets -n "$apps_ns" -o jsonpath='{.status.sync.status} {.status.health.status}' 2>/dev/null || echo "NotFound NotFound")
    if [[ "$app_status" != "Synced Healthy" ]]; then
        check_and_patch_sync_app root-app "$apps_ns"
    fi

    # Check for secret every 5 sec for 10 times
    for i in $(seq 1 40); do

        if kubectl get secret app-orch-catalog-local-postgresql -n orch-app >/dev/null 2>&1; then
            echo "✅ Secret found!"
            break
        fi

        if [ "$i" -lt 40 ]; then
            echo "❌ Secret not found. Waiting 5s..."
            sleep 5
        fi
    done

    # Wait for all required secrets to exist before patching
    local secrets_to_check=(
        "orch-app:app-orch-catalog-local-postgresql"
        "orch-app:app-orch-catalog-reader-local-postgresql"
        "orch-iam:iam-tenancy-local-postgresql"
        "orch-iam:iam-tenancy-reader-local-postgresql"
        "orch-infra:alerting-local-postgresql"
        "orch-infra:alerting-reader-local-postgresql"
        "orch-infra:inventory-local-postgresql"
        "orch-infra:inventory-reader-local-postgresql"
        "orch-platform:platform-keycloak-local-postgresql"
        "orch-platform:platform-keycloak-reader-local-postgresql"
        "orch-platform:vault-local-postgresql"
        "orch-platform:vault-reader-local-postgresql"
        "orch-infra:mps-local-postgresql"
        "orch-infra:mps-reader-local-postgresql"
        "orch-infra:rps-local-postgresql"
        "orch-infra:rps-reader-local-postgresql"
    )

    local max_wait=600  # 10 minutes timeout
    local check_interval=5
    local elapsed=0

    echo "Waiting for all required secrets to exist..."
    for secret_entry in "${secrets_to_check[@]}"; do
        local namespace="${secret_entry%%:*}"
        local secret_name="${secret_entry##*:}"
        elapsed=0

        while ! kubectl get secret "$secret_name" -n "$namespace" >/dev/null 2>&1; do
            if [ $elapsed -ge $max_wait ]; then
                echo "❌ Timeout waiting for secret $secret_name in namespace $namespace after ${max_wait}s"
                exit 1
            fi
            echo "⏳ Waiting for secret $secret_name in namespace $namespace... (${elapsed}s/${max_wait}s)"
            sleep $check_interval
            elapsed=$((elapsed + check_interval))
        done
        echo "✅ Secret $secret_name found in namespace $namespace"
    done

    echo "✅ All required secrets exist, proceeding with patching..."


    kubectl patch secret -n orch-app app-orch-catalog-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$CATALOG_SERVICE\"}}" --type=merge
    kubectl patch secret -n orch-app app-orch-catalog-reader-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$CATALOG_SERVICE\"}}" --type=merge
    kubectl patch secret -n orch-iam iam-tenancy-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$IAM_TENANCY\"}}" --type=merge
    kubectl patch secret -n orch-iam iam-tenancy-reader-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$IAM_TENANCY\"}}" --type=merge
    kubectl patch secret -n orch-infra alerting-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$ALERTING\"}}" --type=merge
    kubectl patch secret -n orch-infra alerting-reader-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$ALERTING\"}}" --type=merge
    kubectl patch secret -n orch-infra inventory-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$INVENTORY\"}}" --type=merge
    kubectl patch secret -n orch-infra inventory-reader-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$INVENTORY\"}}" --type=merge
    kubectl patch secret -n orch-platform platform-keycloak-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$PLATFORM_KEYCLOAK\"}}" --type=merge
    kubectl patch secret -n orch-platform platform-keycloak-reader-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$PLATFORM_KEYCLOAK\"}}" --type=merge
    kubectl patch secret -n orch-platform vault-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$VAULT\"}}" --type=merge
    kubectl patch secret -n orch-platform vault-reader-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$VAULT\"}}" --type=merge
    kubectl patch secret -n orch-infra mps-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$MPS\"}}" --type=merge
    kubectl patch secret -n orch-infra mps-reader-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$MPS\"}}" --type=merge
    kubectl patch secret -n orch-infra rps-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$RPS\"}}" --type=merge
    kubectl patch secret -n orch-infra rps-reader-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$RPS\"}}" --type=merge

    # New secrets needed for postgresql chart migration to cloudnative-pg
    if kubectl get secret orch-app-app-orch-catalog -n orch-database >/dev/null 2>&1; then
      kubectl patch secret -n orch-database orch-app-app-orch-catalog -p "{\"data\": {\"password\": \"$CATALOG_SERVICE\"}}" --type=merge
      kubectl patch secret -n orch-database orch-iam-iam-tenancy -p "{\"data\": {\"password\": \"$IAM_TENANCY\"}}" --type=merge
      kubectl patch secret -n orch-database orch-infra-alerting -p "{\"data\": {\"password\": \"$ALERTING\"}}" --type=merge
      kubectl patch secret -n orch-database orch-infra-inventory -p "{\"data\": {\"password\": \"$INVENTORY\"}}" --type=merge
      kubectl patch secret -n orch-database orch-platform-platform-keycloak -p "{\"data\": {\"password\": \"$PLATFORM_KEYCLOAK\"}}" --type=merge
      kubectl patch secret -n orch-database orch-platform-vault -p "{\"data\": {\"password\": \"$VAULT\"}}" --type=merge
      kubectl patch secret -n orch-database orch-infra-mps -p "{\"data\": {\"password\": \"$MPS\"}}" --type=merge
      kubectl patch secret -n orch-database orch-infra-rps -p "{\"data\": {\"password\": \"$RPS\"}}" --type=merge
    fi
}

# Stop sync operation for root-app, so it won't be synced with the old version of the application.
kubectl patch application root-app -n "$apps_ns" --type merge -p '{"operation":null}'
kubectl patch application root-app -n "$apps_ns" --type json -p '[{"op": "remove", "path": "/status/operationState"}]'

# Force postgresql application to sync with the new version of the application.
echo "
operation:
  sync:
    syncStrategy:
      hook: {}
" | sudo tee /tmp/sync-postgresql-patch.yaml

#kubectl patch -n "$apps_ns" application postgresql-secrets --patch-file /tmp/sync-postgresql-patch.yaml --type merge
kubectl patch -n "$apps_ns" application root-app --patch-file /tmp/sync-postgresql-patch.yaml --type merge

#kubectl patch -n "$apps_ns" application postgresql --patch-file /tmp/sync-postgresql-patch.yaml --type merge

start_time=$(date +%s)
timeout=3600 # 1 hour in seconds
set +e
while true; do
    echo "Checking postgresql-secrets application status..."
    app_status=$(kubectl get application postgresql-secrets -n "$apps_ns" -o jsonpath='{.status.sync.status} {.status.health.status}')
    if [[ "$app_status" == "Synced Healthy" ]]; then
        echo "postgresql-secrets application is Synced and Healthy."
        break
    fi
    current_time=$(date +%s)
    elapsed=$((current_time - start_time))
    if (( elapsed > timeout )); then
        echo "Timeout waiting for postgresql-secrets to be Synced and Healthy."
        exit 1
    fi
    echo "Waiting for postgresql-secrets to be Synced and Healthy... (status: $app_status)"
    sleep 5
done
set -e


delete_postgres

# Stop sync operation for root-app, so it won't be synced with the old version of the application.
kubectl patch application root-app -n "$apps_ns" --type merge -p '{"operation":null}'
kubectl patch application root-app -n "$apps_ns" --type json -p '[{"op": "remove", "path": "/status/operationState"}]'
sleep 30
kubectl patch -n "$apps_ns" application root-app --patch-file /tmp/sync-postgresql-patch.yaml --type merge
sleep 30
patch_secrets
sleep 10

# Restore secret after app delete but before postgress restored
if [[ "$UPGRADE_3_1_X" == "true" ]]; then
        yq e 'del(.metadata.labels, .metadata.annotations, .metadata.uid, .metadata.creationTimestamp)' postgres_secret.yaml | kubectl apply -f -
else
        yq e '
          del(.metadata.labels) |
          del(.metadata.annotations) |
          del(.metadata.ownerReferences) |
          del(.metadata.finalizers) |
          del(.metadata.managedFields) |
          del(.metadata.resourceVersion) |
          del(.metadata.uid) |
          del(.metadata.creationTimestamp)
        ' postgres_secret.yaml | kubectl apply -f -
fi
sleep 30
# Wait until PostgreSQL pod is running (Re-sync)
start_time=$(date +%s)
timeout=300  # 5 minutes in seconds
set +e
while true; do
    echo "Checking PostgreSQL pod status..."
    # CloudNativePG uses cnpg.io/cluster label instead of app.kubernetes.io/name
    pod_status=$(kubectl get pods -n orch-database -l cnpg.io/cluster=postgresql-cluster,cnpg.io/instanceRole=primary -o jsonpath='{.items[0].status.phase}')
    if [[ "$pod_status" == "Running" ]]; then
        echo "PostgreSQL pod is Running."
        sleep 30
        break
    fi
    current_time=$(date +%s)
    elapsed=$((current_time - start_time))
    if (( elapsed > timeout )); then
        echo "Timeout waiting for PostgreSQL pod to be Running."
        exit 1
    fi
    echo "Waiting for PostgreSQL pod to be Running... (status: $pod_status)"
    sleep 5
done
set -e

# Now that PostgreSQL is running, we can restore the secret
restore_postgres

# Update ALL database user passwords in PostgreSQL after restore
echo "Updating all database user passwords in PostgreSQL..."

echo "✅ All database user passwords updated successfully"

vault_unseal

# Re-create the secrets for mps and rps if they were deleted
if [[ -s mps_secret.yaml ]]; then
    kubectl apply -f mps_secret.yaml
fi

if [[ -s rps_secret.yaml ]]; then
    kubectl apply -f rps_secret.yaml
fi


# TODO may need to move the vault unseal before this step
kubectl patch application root-app -n "$apps_ns" --patch-file /tmp/argo-cd/sync-patch.yaml --type merge

# Restore Gitea credentials to Vault
password=$(kubectl get secret app-gitea-credential -n orch-platform -o jsonpath="{.data.password}" | base64 -d)
username=$(kubectl get secret app-gitea-credential -n orch-platform -o jsonpath="{.data.username}" | base64 -d)

# Store Gitea credentials in Vault
kubectl exec -it vault-0 -n orch-platform -c vault -- vault kv put secret/ma_git_service username="$username" password="$password"

# Delete all secrets with name containing 'fleet-gitrepo-cred'
kubectl get secret --all-namespaces --no-headers | awk '/fleet-gitrepo-cred/ {print $1, $2}' | \
while IFS=' ' read -r ns secret; do
    echo "Deleting secret $secret in namespace $ns"
    kubectl delete secret "$secret" -n "$ns"
done

# Fix MPS and RPS connection strings for CNPG migration
echo "Updating MPS and RPS connection strings for CloudNativePG..."

# Get the current passwords from the secrets
MPS_PASSWORD=$(kubectl get secret mps-local-postgresql -n orch-infra -o jsonpath='{.data.PGPASSWORD}' | base64 -d)
RPS_PASSWORD=$(kubectl get secret rps-local-postgresql -n orch-infra -o jsonpath='{.data.PGPASSWORD}' | base64 -d)

# Update MPS connection string to use CNPG service name
MPS_CONN_STRING="postgresql://orch-infra-mps_user:${MPS_PASSWORD}@postgresql-cluster-rw.orch-database/orch-infra-mps?search_path=public&sslmode=disable"
MPS_CONN_BASE64=$(echo -n "$MPS_CONN_STRING" | base64 -w 0)
kubectl patch secret mps -n orch-infra -p "{\"data\":{\"connectionString\":\"$MPS_CONN_BASE64\"}}" --type=merge

# Update RPS connection string to use CNPG service name
RPS_CONN_STRING="postgresql://orch-infra-rps_user:${RPS_PASSWORD}@postgresql-cluster-rw.orch-database/orch-infra-rps?search_path=public&sslmode=disable"
RPS_CONN_BASE64=$(echo -n "$RPS_CONN_STRING" | base64 -w 0)
kubectl patch secret rps -n orch-infra -p "{\"data\":{\"connectionString\":\"$RPS_CONN_BASE64\"}}" --type=merge

echo "✅ Updated MPS and RPS connection strings to use postgresql-cluster-rw.orch-database"

echo "Restart MPS and RPS to pick up new connection strings"
kubectl rollout restart deployment rps -n orch-infra
kubectl rollout restart deployment mps -n orch-infra

echo "✅ MPS and RPS deployments restarted"

echo "Restart inventory to refresh database connection to CNPG service"
kubectl rollout restart deployment inventory -n orch-infra

echo "✅ inventory deployment restarted"

echo "Restart onboarding-manager to connect to refreshed inventory service"
kubectl rollout restart deployment onboarding-manager -n orch-infra

echo "✅ onboarding-manager deployment restarted"

echo "Restart dkam to refresh connection"
kubectl rollout restart deployment dkam -n orch-infra

echo "✅ dkam deployment restarted"

echo "Restart keycloak-tenant-controller to resolve vault authentication issues"

echo "Restart keycloak-tenant-controller to refresh connection"
restart_statefulset keycloak-tenant-controller-set orch-platform

echo "Restart harbor-oci-database to refresh connection"
restart_statefulset harbor-oci-database orch-harbor

# Restart harbor-oci-core to refresh connection
echo "Restart harbor-oci-core to refresh connection"
kubectl rollout restart deployment harbor-oci-core -n orch-harbor

echo "✅ harbor-oci-core restarted"


echo "Cleaning up external-secrets installation..."

if kubectl get crd clustersecretstores.external-secrets.io >/dev/null 2>&1; then
    kubectl delete crd clustersecretstores.external-secrets.io &
    kubectl patch crd/clustersecretstores.external-secrets.io -p '{"metadata":{"finalizers":[]}}' --type=merge
fi
if kubectl get crd secretstores.external-secrets.io >/dev/null 2>&1; then
    kubectl delete crd secretstores.external-secrets.io &
    kubectl patch crd/secretstores.external-secrets.io -p '{"metadata":{"finalizers":[]}}' --type=merge
fi
if kubectl get crd externalsecrets.external-secrets.io >/dev/null 2>&1; then
    kubectl delete crd externalsecrets.external-secrets.io &
    kubectl patch crd/externalsecrets.external-secrets.io -p '{"metadata":{"finalizers":[]}}' --type=merge
fi

# Apply External Secrets CRDs with server-side apply
echo "Applying external-secrets CRDs with server-side apply..."
kubectl apply --server-side=true --force-conflicts -f https://raw.githubusercontent.com/external-secrets/external-secrets/refs/tags/v0.20.4/deploy/crds/bundle.yaml || true

# Unseal vault after external-secrets is ready
echo "Unsealing vault..."
vault_unseal
echo "✅ Vault unsealed successfully"
# Stop root-app old sync as it will be stuck.
kubectl patch application root-app -n  "$apps_ns"  --type merge -p '{"operation":null}'
kubectl patch application root-app -n  "$apps_ns"  --type json -p '[{"op": "remove", "path": "/status/operationState"}]'
# Apply root-app Patch
kubectl patch application root-app -n  "$apps_ns"  --patch-file /tmp/argo-cd/sync-patch.yaml --type merge
sleep 10
#restart os-resource-manager
kubectl delete application tenancy-api-mapping -n onprem || true
kubectl delete application tenancy-datamodel -n onprem || true
kubectl delete deployment -n orch-infra os-resource-manager || true
#restart tls-boot secrets
kubectl delete secret tls-boots -n orch-boots || true
kubectl delete secret boots-ca-cert -n orch-gateway || true
kubectl delete secret boots-ca-cert -n orch-infra || true
sleep 10
kubectl delete pod -n orch-infra -l app.kubernetes.io/name=dkam 2>/dev/null || true

echo "Wait ~10–15 minutes for ArgoCD to sync and deploy all application"
echo "   👉 Run the script to to futher sync and post run"
echo "          ./after_upgrade_restart.sh"
echo ""
echo "Upgrade completed! Wait for ArgoCD applications to be in 'Synced' and 'Healthy' state"
