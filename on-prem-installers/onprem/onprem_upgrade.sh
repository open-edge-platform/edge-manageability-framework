#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Script Name: onprem_upgrade.sh
# Description: This script:
#               If requested - does a backup of PVs and cluster's ETCD
#               Reads AZURE AD refresh_token credential from user input,
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

# Import shared functions
# shellcheck disable=SC1091
source "$(dirname "${0}")/functions.sh"
source "$(dirname "${0}")/upgrade_postgres.sh"

### Constants
RELEASE_SERVICE_URL="${RELEASE_SERVICE_URL:-registry-rs.edgeorchestration.intel.com}"
ORCH_INSTALLER_PROFILE="${ORCH_INSTALLER_PROFILE:-onprem}"
DEPLOY_VERSION="${DEPLOY_VERSION:-v3.1.0}"  # Updated to v3.1.0
GITEA_IMAGE_REGISTRY="${GITEA_IMAGE_REGISTRY:-docker.io}"
USE_LOCAL_PACKAGES="${USE_LOCAL_PACKAGES:-false}"  # New flag for local packages

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

### Functions

retrieve_and_apply_config() {
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

    while true; do
        if [[ -z ${ARGO_IP} ]]; then
            echo "Enter Argo IP:"
            read -r ARGO_IP
            export ARGO_IP
        fi

        if [[ -z ${TRAEFIK_IP} ]]; then
            echo "Enter Traefik IP:"
            read -r TRAEFIK_IP
            export TRAEFIK_IP
        fi

        if [[ -z ${NGINX_IP} ]]; then
            echo "Enter Nginx IP:"
            read -r NGINX_IP
            export NGINX_IP
        fi

        if [[ $ARGO_IP =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ && $TRAEFIK_IP =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ && $NGINX_IP =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            yq -i '.postCustomTemplateOverwrite.metallb-config.ArgoIP|=strenv(ARGO_IP)' "$tmp_dir"/$si_config_repo/orch-configs/clusters/"$ORCH_INSTALLER_PROFILE".yaml
            yq -i '.postCustomTemplateOverwrite.metallb-config.TraefikIP|=strenv(TRAEFIK_IP)' "$tmp_dir"/$si_config_repo/orch-configs/clusters/"$ORCH_INSTALLER_PROFILE".yaml
            yq -i '.postCustomTemplateOverwrite.metallb-config.NginxIP|=strenv(NGINX_IP)' "$tmp_dir"/$si_config_repo/orch-configs/clusters/"$ORCH_INSTALLER_PROFILE".yaml
            break
        else
            echo "Inputted values are not valid IPs. Please input correct IPs without any masks."
            ARGO_IP=""
            TRAEFIK_IP=""
            NGINX_IP=""
        fi
    done

    sre_tls=$(kubectl get applications -n "$apps_ns" sre-exporter -o jsonpath='{.spec.sources[*].helm.valuesObject.otelCollector.tls.enabled}')
    if [[ $sre_tls = 'true' ]]; then
        yq -i '.argo.o11y.sre.tls.enabled|=true' "$tmp_dir"/$si_config_repo/orch-configs/clusters/"$ORCH_INSTALLER_PROFILE".yaml
        sre_dest_ca_cert=$(kubectl get applications -n "$apps_ns" sre-exporter -o jsonpath='{.spec.sources[*].helm.valuesObject.otelCollector.tls.caSecret.enabled}')
        if [[ "${sre_dest_ca_cert}" == "true" ]]; then
            yq -i '.argo.o11y.sre.tls.caSecretEnabled|=true' "$tmp_dir"/$si_config_repo/orch-configs/clusters/"$ORCH_INSTALLER_PROFILE".yaml
        fi
    else
        yq -i '.argo.o11y.sre.tls.enabled|=false' "$tmp_dir"/$si_config_repo/orch-configs/clusters/"$ORCH_INSTALLER_PROFILE".yaml
    fi

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
if [[ $? -ne 0 ]]; then
    echo "PostgreSQL is not running or backup file already exists. Exiting..."
    exit 1
fi

# Perform postgreSQL backup
kubectl get secret -n $postgres_namespace postgresql -o yaml > postgres_secret.yaml

# Backup PostgreSQL databases
backup_postgres


### Upgrade existing Orch installation
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

# Save PostgreSQL passwords as they will get overwritten during orch-installer package upgrade
# ALERTING=$(kubectl get secret alerting-local-postgresql -n orch-infra -o jsonpath='{.data.PGPASSWORD}')
# CATALOG_SERVICE=$(kubectl get secret app-orch-catalog-local-postgresql -n orch-app -o jsonpath='{.data.PGPASSWORD}')
# INVENTORY=$(kubectl get secret inventory-local-postgresql -n orch-infra -o jsonpath='{.data.PGPASSWORD}')
# IAM_TENANCY=$(kubectl get secret iam-tenancy-local-postgresql -n orch-iam -o jsonpath='{.data.PGPASSWORD}')
# PLATFORM_KEYCLOAK=$(kubectl get secret platform-keycloak-local-postgresql -n orch-platform -o jsonpath='{.data.PGPASSWORD}')
# VAULT=$(kubectl get secret vault-local-postgresql -n orch-platform -o jsonpath='{.data.PGPASSWORD}')
# POSTGRESQL=$(kubectl get secret postgresql -n orch-database -o jsonpath='{.data.postgres-password}')

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

# Force sync all applications on the cluster. We need to ensure that new version of
# ArgoCD properly picked Applications definitions that were governed by older version.
echo "
operation:
  sync:
    syncStrategy:
      hook: {}
" | sudo tee /tmp/argo-cd/sync-patch.yaml

apps=$(kubectl get applications -n "$apps_ns" --no-headers -o custom-columns=":metadata.name")
for app in $apps; do
    echo "Syncing ArgoCD application: $app"
    kubectl patch -n "$apps_ns" applications "$app" --patch-file /tmp/argo-cd/sync-patch.yaml --type merge >/dev/null 2>&1
done

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

# kubectl patch secret -n orch-app app-orch-catalog-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$CATALOG_SERVICE\"}}" --type=merge
# kubectl patch secret -n orch-app app-orch-catalog-reader-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$CATALOG_SERVICE\"}}" --type=merge
# kubectl patch secret -n orch-iam iam-tenancy-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$IAM_TENANCY\"}}" --type=merge
# kubectl patch secret -n orch-iam iam-tenancy-reader-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$IAM_TENANCY\"}}" --type=merge
# kubectl patch secret -n orch-infra alerting-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$ALERTING\"}}" --type=merge
# kubectl patch secret -n orch-infra alerting-reader-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$ALERTING\"}}" --type=merge
# kubectl patch secret -n orch-infra inventory-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$INVENTORY\"}}" --type=merge
# kubectl patch secret -n orch-infra inventory-reader-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$INVENTORY\"}}" --type=merge
# kubectl patch secret -n orch-platform platform-keycloak-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$PLATFORM_KEYCLOAK\"}}" --type=merge
# kubectl patch secret -n orch-platform platform-keycloak-reader-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$PLATFORM_KEYCLOAK\"}}" --type=merge
# kubectl patch secret -n orch-platform vault-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$VAULT\"}}" --type=merge
# kubectl patch secret -n orch-platform vault-reader-local-postgresql -p "{\"data\": {\"PGPASSWORD\": \"$VAULT\"}}" --type=merge
# kubectl patch secret -n orch-database passwords -p "$(cat <<EOF
# {
#   "data": {
#     "alerting": "$ALERTING",
#     "app-orch-catalog": "$CATALOG_SERVICE",
#     "iam-tenancy": "$IAM_TENANCY",
#     "inventory": "$INVENTORY",
#     "platform-keycloak": "$PLATFORM_KEYCLOAK",
#     "vault": "$VAULT"
#   }
# }
# EOF
# )" --type=merge

# Patch postgresql secret
#kubectl patch secret -n orch-database postgresql -p "{\"data\": {\"postgres-password\": \"$POSTGRESQL\"}}" --type=merge

# delete_postgres


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


# kubectl patch -n "$apps_ns" application postgresql --patch-file /tmp/sync-postgresql-patch.yaml --type merge

start_time=$(date +%s)
timeout=300  # 5 minutes in seconds
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
sleep 10
kubectl patch -n "$apps_ns" application root-app --patch-file /tmp/sync-postgresql-patch.yaml --type merge

# Restore secret after app delete but before postgress restored
yq e 'del(.metadata.labels, .metadata.annotations, .metadata.uid, .metadata.creationTimestamp)' postgres_secret.yaml | kubectl apply -f -

# Wait until PostgreSQL pod is running (Re-sync)
start_time=$(date +%s)
timeout=300  # 5 minutes in seconds
set +e
while true; do
    echo "Checking PostgreSQL pod status..."
    podname=$(kubectl get pods -n orch-database -l app.kubernetes.io/name=postgresql -o jsonpath='{.items[0].metadata.name}')
    pod_status=$(kubectl get pods -n orch-database $podname -o jsonpath='{.status.phase}')
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

echo "Upgrade completed! Wait for ArgoCD applications to be in 'Healthy' state"
