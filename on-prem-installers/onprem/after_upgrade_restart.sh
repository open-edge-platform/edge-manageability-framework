#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Description:
#   This script synchronizes ArgoCD applications after an upgrade, ensuring all
#   applications reach a healthy and synced state. It processes applications in
#   wave order with automatic retry and error recovery mechanisms.
#
# Main Steps:
#   1. Install ArgoCD CLI if not present
#   2. Login to ArgoCD server (LoadBalancer or NodePort)
#   3. Stop and reconfigure root-app sync
#   4. Sync all applications (excluding root-app) in wave order
#   5. Perform post-upgrade cleanup:
#      - Delete obsolete applications (tenancy-api-mapping, tenancy-datamodel)
#      - Remove legacy deployments (os-resource-manager)
#      - Clean up stale secrets (tls-boots, boots-ca-cert)
#   6. Re-sync all applications
#   7. Sync root-app
#   8. Validate final state of all applications
#
#   All logs are written to /var/log/orch-upgrade/ directory.
#
# Usage:
#   ./after_upgrade_restart.sh [NAMESPACE]
#
#   Arguments:
#     NAMESPACE    - Target namespace for applications (optional, default: onprem)
#
# Examples:
#   ./after_upgrade_restart.sh              # Uses default namespace 'onprem'
#
# Environment Variables:
#   ARGO_NS         - ArgoCD namespace (default: argocd)
#
# Exit Codes:
#   0 - All applications synced successfully
#   1 - Sync failed after all retries

set -o pipefail

# ============================================================
# Source environment variables
# ============================================================
if [[ -f "onprem.env" ]]; then
    # shellcheck disable=SC1091
    source onprem.env
fi

# ============================================================
# ============= GLOBAL CONFIGURATION VARIABLES ===============
# ============================================================

# Parse command-line arguments
NS="${1:-onprem}"  # Use first argument or default to "onprem"
ARGO_NS="argocd"

# Log file configuration
LOG_DIR="/var/log/orch-upgrade"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/argo_sync_$(date +%Y%m%d_%H%M%S).log"

# Set up file descriptor 3 for console output (same as main log)
exec 3> >(tee -a "$LOG_FILE")

# Redirect stdout to tee (log file + stdout)
exec > >(tee -a "$LOG_FILE")
exec 2>&1

echo "[INFO] Using namespace: $NS"
echo "[INFO] Using ArgoCD namespace: $ARGO_NS"
echo "[INFO] Log file: $LOG_FILE"

# Print script identity to avoid running an old copy
echo "[INFO] Script path: $0"
if command -v sha256sum >/dev/null 2>&1 && [[ -r "$0" ]]; then
    echo "[INFO] Script sha256: $(sha256sum "$0" | awk '{print $1}')"
fi

# Also output initial info to console
echo "[INFO] ArgoCD sync script started" >&3
echo "[INFO] Log file: $LOG_FILE" >&3
echo "" >&3

# Sync behaviour
GLOBAL_POLL_INTERVAL=10           # seconds
APP_MAX_WAIT=30             #  to wait for any app (Healthy+Synced)
APP_MAX_RETRIES=5                 # retry 5 times for each app (after 3 fails, restart root-app)
GLOBAL_SYNC_RETRIES=1            # Global retry for entire sync process

# Apps requiring server-side apply (space-separated list)
SERVER_SIDE_APPS="external-secrets copy-app-gitea-cred-to-fleet copy-ca-cert-boots-to-gateway copy-ca-cert-boots-to-infra copy-ca-cert-gateway-to-cattle copy-ca-cert-gateway-to-infra copy-ca-cert-gitea-to-app copy-ca-cert-gitea-to-cluster copy-cluster-gitea-cred-to-fleet copy-keycloak-admin-to-infra infra-external platform-keycloak namespace-label wait-istio-job"

# shellcheck disable=SC1091
# ============================================================
# REQUIRE COMMANDS
# ============================================================
require_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "[ERROR] Required command '$1' not found. Install it and retry."
        exit 1
    fi
}
require_cmd kubectl
require_cmd jq

# ============================================================
# ArgoCD CLI Install
# ============================================================
install_argocd_cli() {
        if ! command -v argocd >/dev/null 2>&1; then
    echo "[INFO] argocd CLI not found. Installing..."
    VERSION=$(curl -L -s https://raw.githubusercontent.com/argoproj/argo-cd/stable/VERSION)
    echo "[INFO] Latest version: $VERSION"
    curl -sSL -o argocd-linux-amd64 \
        https://github.com/argoproj/argo-cd/releases/download/v"${VERSION}"/argocd-linux-amd64
    sudo install -m 555 argocd-linux-amd64 /usr/local/bin/argocd
    rm -f argocd-linux-amd64
    echo "[INFO] argocd CLI installed successfully."
else
    echo "[INFO] argocd CLI already installed: $(argocd version --client | head -1)"
fi
}
install_argocd_cli

# ============================================================
# Fetch admin password
# ============================================================
echo "[INFO] Fetching ArgoCD admin password..."
if command -v yq >/dev/null 2>&1; then
    ADMIN_PASSWD=$(kubectl get secret -n "$ARGO_NS" argocd-initial-admin-secret -o yaml \
        | yq -r '.data.password' | base64 -d)
else
    ADMIN_PASSWD=$(kubectl get secret -n "$ARGO_NS" argocd-initial-admin-secret \
        -o jsonpath='{.data.password}' | base64 -d)
fi

# ============================================================
# Discover Argo endpoint
# ============================================================
echo "[INFO] Detecting ArgoCD Server endpoint..."
LB_IP=$(kubectl get svc argocd-server -n "$ARGO_NS" \
    -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

if [[ -n "$LB_IP" ]]; then
    ARGO_ENDPOINT="$LB_IP"
    echo "[INFO] Using LoadBalancer IP: $ARGO_ENDPOINT"
else
    NODEPORT=$(kubectl get svc argocd-server -n "$ARGO_NS" -o jsonpath='{.spec.ports[0].nodePort}')
    NODEIP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}' | awk '{print $1}')
    ARGO_ENDPOINT="${NODEIP}:${NODEPORT}"
    echo "[INFO] Using NodePort: $ARGO_ENDPOINT"
fi

# Delete Kyverno ClusterPolicies that restart MPS/RPS deployments when secrets change (if present)

for policy in restart-mps-deployment-on-secret-change restart-rps-deployment-on-secret-change; do
  if kubectl get clusterpolicy "$policy" -o name >/dev/null 2>&1; then
    echo "Deleting ClusterPolicy: $policy"
    kubectl delete clusterpolicy "$policy"
  else
    echo "ClusterPolicy $policy not found, skipping"
  fi
done

# ============================================================
# Argo Login
# ============================================================
echo "[INFO] Logging into ArgoCD..."
argocd login "$ARGO_ENDPOINT" --username admin --password "$ADMIN_PASSWD" --insecure --grpc-web
echo "[INFO] Login OK."

# ============================================================
# Pre-sync: Stop root-app sync and apply patch
# ============================================================
echo "[INFO] Stopping root-app sync..."
apps_ns="$NS"
# Stop root-app sync.
kubectl patch application root-app -n "$apps_ns" --type merge -p '{"operation":null}'
kubectl patch application root-app -n "$apps_ns" --type json -p '[{"op": "remove", "path": "/status/operationState"}]'
sleep 10
# Apply root-app sync
echo "[INFO] Applying root-app sync patch..."
kubectl patch application root-app -n "$apps_ns" --patch-file /tmp/argo-cd/sync-patch.yaml --type merge
sleep 10
echo "[INFO] Root-app sync configuration completed."

# ============================================================
# Fetch all apps by wave
# ============================================================
get_all_apps_by_wave() {
    kubectl get applications.argoproj.io -n "$NS" -o json \
    | jq -r '.items[] |
        {
            name: .metadata.name,
            wave: (.metadata.annotations["argocd.argoproj.io/sync-wave"] // "0"),
            health: .status.health.status,
            sync: .status.sync.status
        }
        | "\(.wave) \(.name) \(.health) \(.sync)"
    ' | sort -n -k1
}

# ============================================================
# Fetch NOT-GREEN apps by wave
# ============================================================
get_not_green_apps() {
    kubectl get applications.argoproj.io -n "$NS" -o json \
    | jq -r '.items[] |
        {
            name: .metadata.name,
            wave: (.metadata.annotations["argocd.argoproj.io/sync-wave"] // "0"),
            health: .status.health.status,
            sync: .status.sync.status
        }
        | select(.health != "Healthy" or .sync != "Synced")
        | "\(.wave) \(.name) \(.health) \(.sync)"
    ' | sort -n -k1
}

# Optional color helpers
bold() { tput bold 2>/dev/null; }
normal() { tput sgr0 2>/dev/null; }
green() { tput setaf 2>/dev/null 2 && tput setaf 2; }
red() { tput setaf 1 2>/dev/null; }
yellow() { tput setaf 3 2>/dev/null; }
blue() { tput setaf 4 2>/dev/null; }
reset() { tput sgr0 2>/dev/null; }

# Get timestamp
get_timestamp() {
    date '+%Y-%m-%d %H:%M:%S'
}

# ============================================================
# Patch pod finalizers for pods created by a Job
# NOTE: `kubectl patch` does NOT support `-l/--selector`, so we
# must enumerate pods first and patch them individually.
# ============================================================
patch_job_pods_remove_finalizers() {
    local job_ns="$1"
    local job_name="$2"
    local pods

    pods=$(kubectl get pods -n "$job_ns" -l "job-name=${job_name}" -o name 2>/dev/null || true)
    [[ -z "$pods" ]] && return 0

    while IFS= read -r pod; do
        [[ -z "$pod" ]] && continue
        kubectl patch "$pod" -n "$job_ns" --type=merge -p='{"metadata":{"finalizers":[]}}' 2>/dev/null || true
    done <<< "$pods"
}

# ============================================================
# Console output helpers (summary only)
# ============================================================
console_info() {
    echo "$@" >&3
}

console_success() {
    echo "$(green)$*$(reset)" >&3
}

console_warn() {
    echo "$(yellow)$*$(reset)" >&3
}

console_error() {
    echo "$(red)$*$(reset)" >&3
}

# ============================================================
# Check and fix CRD version mismatches
# ============================================================
check_and_fix_crd_version_mismatch() {
    local app_name="$1"

    # Get application status
    local status
    status=$(kubectl get applications.argoproj.io "$app_name" -n "$NS" -o json 2>/dev/null)
    if [[ -z "$status" ]]; then
        return 1
    fi

    # Check for CRD version mismatch errors in sync messages
    local version_mismatch
    version_mismatch=$(echo "$status" | jq -r '
        .status.conditions[]? |
        select(.type == "ComparisonError" or .type == "SyncError") |
        select(.message | contains("could not find version") or contains("Version") and contains("is installed")) |
        .message
    ' 2>/dev/null)

    if [[ -n "$version_mismatch" ]]; then
        echo "$(red)[CRD-VERSION-MISMATCH] Detected CRD version mismatch in $app_name:$(reset)"
        echo "$version_mismatch"

        # Extract CRD details from error message
        local crd_group
        crd_group=$(echo "$version_mismatch" | grep -oP '[a-z0-9.-]+\.[a-z]+(?=/[A-Z])' | head -1)
        local crd_kind
        crd_kind=$(echo "$version_mismatch" | grep -oP '/[A-Z][a-zA-Z]+' | sed 's|/||' | head -1)

        if [[ -n "$crd_group" && -n "$crd_kind" ]]; then
            # Try to find and list the CRD
            local crd_name="${crd_kind,,}s.${crd_group}"
            echo "$(yellow)[INFO] Looking for CRD: $crd_name$(reset)"

            # Check if CRD exists
            if kubectl get crd "$crd_name" &>/dev/null; then
                echo "$(yellow)[INFO] CRD $crd_name exists, checking versions...$(reset)"
                kubectl get crd "$crd_name" -o jsonpath='{.spec.versions[*].name}' 2>/dev/null
                echo

                # For external-secrets.io, we need to update to v1beta1
                if [[ "$crd_group" == "external-secrets.io" ]]; then
                    echo "$(yellow)[FIX] Attempting to refresh application to use correct CRD version...$(reset)"
                    argocd app get "${NS}/${app_name}" --hard-refresh --grpc-web >/dev/null 2>&1 || true
                    sleep 3
                    return 0
                fi
            else
                echo "$(red)[ERROR] CRD $crd_name not found on cluster$(reset)"
            fi
        fi

        return 0
    fi

    return 1
}

# ============================================================
# Check if application has failed sync and needs cleanup
# ============================================================
check_and_handle_failed_sync() {
    local app_name="$1"
    local full_app="${NS}/${app_name}"

    # Get application status
    local status
    status=$(kubectl get applications.argoproj.io "$app_name" -n "$NS" -o json 2>/dev/null)
    if [[ -z "$status" ]]; then
        return 1
    fi

    local sync_phase
    sync_phase=$(echo "$status" | jq -r '.status.operationState.phase // "Unknown"')
    #local sync_status
    #sync_status=$(echo "$status" | jq -r '.status.sync.status // "Unknown"')

    # Check if sync failed
    if [[ "$sync_phase" == "Failed" || "$sync_phase" == "Error" ]]; then
        echo "$(red)[FAILED-SYNC] Application $app_name has failed sync (phase=$sync_phase)$(reset)"

        # Check for failed jobs/CRDs
        local failed_resources
        failed_resources=$(echo "$status" | jq -r '
            .status.resources[]? |
            select(.kind == "Job" or .kind == "CustomResourceDefinition") |
            select(.health.status == "Degraded" or .health.status == "Missing" or .health.status == null) |
            "\(.kind) \(.namespace) \(.name)"
        ')

        if [[ -n "$failed_resources" ]]; then
            echo "$(red)[CLEANUP] Found failed jobs/CRDs in $app_name:$(reset)"
            while IFS= read -r res_line; do
                [[ -z "$res_line" ]] && continue
                read -r kind res_ns res_name <<< "$res_line"
                echo "$(red)  - Deleting $kind $res_name in $res_ns (background)$(reset)"

                if [[ "$kind" == "Job" ]]; then
                    kubectl delete pods -n "$res_ns" -l job-name="$res_name" --ignore-not-found=true 2>/dev/null &
                    kubectl delete job "$res_name" -n "$res_ns" --ignore-not-found=true 2>/dev/null &
                    kubectl patch job "$res_name" -n "$res_ns" --type=merge -p='{"metadata":{"finalizers":[]}}' || true
                elif [[ "$kind" == "CustomResourceDefinition" ]]; then
                    kubectl delete crd "$res_name" --ignore-not-found=true 2>/dev/null &
                fi
            done <<< "$failed_resources"
        fi

        # Terminate stuck operations and refresh
        echo "$(yellow)[RESTART] Restarting sync for $app_name...$(reset)"
        argocd app terminate-op "$full_app" --grpc-web 2>/dev/null || true
        sleep 2
        argocd app get "$full_app" --hard-refresh --grpc-web >/dev/null 2>&1 || true
        sleep 5

        # Trigger a new sync
        echo "$(yellow)[RESYNC] Triggering fresh sync for $app_name...$(reset)"
        argocd app sync "$full_app" --grpc-web 2>&1 || true
        sleep 5

        return 0
    fi

    return 1
}

# ============================================================
# Stop root-app sync operations
# ============================================================
root_app_stop_start() {
    echo "$(yellow)[INFO] Stopping root-app sync operations...$(reset)"
    kubectl patch application root-app -n "$NS" --type merge -p '{"operation":null}' 2>/dev/null || true
    kubectl patch application root-app -n "$NS" --type json -p '[{"op": "remove", "path": "/status/operationState"}]' 2>/dev/null || true
    sleep 2
    kubectl patch application root-app -n "$NS" --patch-file /tmp/argo-cd/sync-patch.yaml --type merge || true
    sleep 5
}

# ============================================================
# Delete an ArgoCD application
# ============================================================
delete_app() {
    local app_name="$1"
    local app_namespace="${2:-$NS}"
    
    echo "$(yellow)[DELETE] Deleting application $app_name in namespace $app_namespace...$(reset)"
    
    # Stop root-app sync
    kubectl patch application root-app -n "$app_namespace" --type merge -p '{"operation":null}' 2>/dev/null || true
    kubectl patch application root-app -n "$app_namespace" --type json -p '[{"op": "remove", "path": "/status/operationState"}]' 2>/dev/null || true
    sleep 5
    
    # Delete application with finalizer removal
    kubectl patch application "$app_name" -n "$app_namespace" --type=json -p='[{"op":"remove","path":"/metadata/finalizers"}]' 2>/dev/null || true
    kubectl delete application "$app_name" -n "$app_namespace" --force --grace-period=0 --ignore-not-found=true 2>/dev/null || true
    sleep 5
    
    # Apply root-app sync
    kubectl patch application root-app -n "$app_namespace" --patch-file /tmp/argo-cd/sync-patch.yaml --type merge 2>/dev/null || true
    sleep 10
    
    echo "$(blue)[INFO] Application $app_name deleted.$(reset)"
}

# ============================================================
# Check and download DKAM certificates
# ============================================================
check_and_download_dkam_certs() {
    echo "$(yellow)[INFO] Checking DKAM certificates readiness...$(reset)"
    console_info "[→] Downloading DKAM certificates..."
    
    # Remove old certificates if they exist
    rm -rf Full_server.crt signed_ipxe.efi 2>/dev/null || true
    
    local max_attempts=20  # 20 attempts * 30 seconds = 10 minutes
    local attempt=1
    local success=false
    
    while (( attempt <= max_attempts )); do
        echo "[$(get_timestamp)] [Attempt ${attempt}/${max_attempts}] Checking DKAM certificate availability..."
        
        # Try to download Full_server.crt
        if wget https://tinkerbell-haproxy."$CLUSTER_DOMAIN"/tink-stack/keys/Full_server.crt --no-check-certificate --no-proxy -q -O Full_server.crt 2>/dev/null; then
            echo "$(green)[OK] Full_server.crt downloaded successfully$(reset)"
            
            # Try to download signed_ipxe.efi using the certificate
            if wget --ca-certificate=Full_server.crt https://tinkerbell-haproxy."$CLUSTER_DOMAIN"/tink-stack/signed_ipxe.efi -q -O signed_ipxe.efi 2>/dev/null; then
                echo "$(green)[OK] signed_ipxe.efi downloaded successfully$(reset)"
                success=true
                break
            else
                echo "$(yellow)[WARN] Failed to download signed_ipxe.efi, retrying...$(reset)"
                rm -f Full_server.crt signed_ipxe.efi 2>/dev/null || true
            fi
        else
            echo "$(yellow)[WARN] Full_server.crt not available yet, waiting...$(reset)"
        fi
        
        if (( attempt < max_attempts )); then
            echo "[INFO] Waiting 30 seconds before next attempt..."
            sleep 30
        fi
        ((attempt++))
    done
    
    if [[ "$success" == "true" ]]; then
        echo "$(green)[SUCCESS] DKAM certificates are ready and downloaded$(reset)"
        console_success "[✓] DKAM certificates downloaded successfully"
        return 0
    else
        echo "$(red)[FAIL] DKAM certificates not available after 10 minutes$(reset)"
        console_error "[✗] DKAM certificates download failed after 10 minutes"
        return 1
    fi
}

# ============================================================
# Clean unhealthy jobs for a specific application
# ============================================================
clean_unhealthy_jobs_for_app() {
    local app_name="$1"

    # Check for unhealthy jobs in this app and clean them up
    app_resources=$(kubectl get applications.argoproj.io "$app_name" -n "$NS" -o json 2>/dev/null | jq -r '
        .status.resources[]? |
        select(.kind == "Job" and (.health.status != "Healthy" or .health.status == null)) |
        "\(.namespace) \(.name)"
    ')

    if [[ -n "$app_resources" ]]; then
        echo "$(yellow)[CLEANUP] Found unhealthy/failed jobs in $app_name:$(reset)"
        while IFS= read -r job_line; do
            [[ -z "$job_line" ]] && continue
            read -r job_ns job_name <<< "$job_line"
            echo "$(yellow)  - Deleting job $job_name in $job_ns (background)$(reset)"
            kubectl delete pods -n "$job_ns" -l job-name="$job_name" --ignore-not-found=true 2>/dev/null &
            patch_job_pods_remove_finalizers "$job_ns" "$job_name"
            kubectl delete job "$job_name" -n "$job_ns" --ignore-not-found=true 2>/dev/null &
            kubectl patch job "$job_name" -n "$job_ns" --type=merge -p='{"metadata":{"finalizers":[]}}' || true
        done <<< "$app_resources"
        echo "[INFO] Job cleanup initiated in background, proceeding..."
        return 0
    fi
    return 1
}

print_header() {
    echo
    echo "$(bold)$(blue)============================================================$(reset)"
    echo "$(bold)$(blue)== $1$(reset)"
    echo "$(bold)$(blue)============================================================$(reset)"
    # Also output to console
    console_info ""
    console_info "$(bold)$(blue)== $1$(reset)"
}

print_table_header() {
    printf "%-18s %-25s %-10s %-10s\n" "Wave" "App Name" "Health" "Sync"
    echo "------------------------------------------------------------"
}

print_table_row() {
    local wave="$1" name="$2" health="$3" sync="$4"
    local color=""
    if [[ "$health" == "Healthy" && "$sync" == "Synced" ]]; then
        color=$(green)
    elif [[ "$health" == "Healthy" || "$sync" == "Synced" ]]; then
        color=$(yellow)
    else
        color=$(red)
    fi
    printf "%s%-18s %-25s %-10s %-10s%s\n" "$color" "$wave" "$name" "$health" "$sync" "$(reset)"
}

# ============================================================
# Sync apps one-by-one in wave order (with nice reporting)
# ============================================================
sync_not_green_apps_once() {
    mapfile -t all_apps < <(get_all_apps_by_wave)
    [[ ${#all_apps[@]} -eq 0 ]] && { echo "[WARN] No applications found in namespace '$NS'."; return 0; }

    print_header "Applications (Wave-Ordered Status)"
    print_table_header
    for line in "${all_apps[@]}"; do
        read -r wave name health sync <<< "$line"
        print_table_row "$wave" "$name" "$health" "$sync"
    done
    echo

    # Print summary of NOT-GREEN apps before syncing
    echo "$(bold)[INFO] Apps NOT Healthy or NOT Synced:$(reset)"
    local not_green_count=0
    for line in "${all_apps[@]}"; do
        read -r wave name health sync <<< "$line"
        if [[ "$health" != "Healthy" || "$sync" != "Synced" ]]; then
            echo "$(red)  - $name (wave=$wave) Health=$health Sync=$sync$(reset)"
            ((not_green_count++))
        fi
    done
    if (( not_green_count > 0 )); then
        console_info "[→] Starting sync for $not_green_count applications"
    fi
    echo

    # Sync NOT-GREEN apps in wave order, skipping root-app until last
    for line in "${all_apps[@]}"; do
        read -r wave name health sync <<< "$line"
        full_app="${NS}/${name}"

        # Skip root-app for now, handle it after all other apps
        if [[ "$name" == "root-app" ]]; then
            continue
        fi

        # Console: Show which app is being processed
        console_info "[→] Syncing: $name (wave=$wave)"

        # First check and handle any failed syncs
        echo "[$(get_timestamp)] Checking for failed syncs in $name..."
        check_and_handle_failed_sync "$name"

        # Special pre-sync handling for haproxy-ingress-pxe-boots
        if [[ "$name" == "haproxy-ingress-pxe-boots" ]]; then
            echo "$(yellow)[INFO] Pre-sync: haproxy-ingress-pxe-boots detected - deleting tls-boots secret first...$(reset)"
            kubectl delete secret tls-boots -n orch-boots 2>/dev/null || true
            sleep 3
        fi

        attempt=1
        synced=false
        while (( attempt <= APP_MAX_RETRIES )); do
            status=$(kubectl get applications.argoproj.io "$name" -n "$NS" -o json 2>/dev/null)
            if [[ -z "$status" ]]; then
                echo "$(red)[FAIL] $full_app not found$(reset)"
                break
            fi
            health=$(echo "$status" | jq -r '.status.health.status')
            sync=$(echo "$status" | jq -r '.status.sync.status')
            last_sync_status=$(echo "$status" | jq -r '.status.operationState.phase // "Unknown"')
            last_sync_time=$(echo "$status" | jq -r '.status.operationState.finishedAt // "N/A"')

            echo "[$(get_timestamp)] $full_app Status: Health=$health Sync=$sync LastSync=$last_sync_status Time=$last_sync_time"

            if (( attempt == 1 )); then
                if [[ "$health" == "Healthy" && "$sync" == "Synced" ]]; then
                    echo "$(green)[OK] $full_app (wave=$wave) already Healthy+Synced$(reset)"
                    console_success "[✓] $name - Already synced"
                    synced=true
                    break
                fi

                # Check if last sync failed and clean up
                if [[ "$last_sync_status" == "Failed" || "$last_sync_status" == "Error" ]]; then
                    echo "$(red)[CLEANUP] Last sync failed for $full_app, cleaning up stuck resources...$(reset)"
                    clean_unhealthy_jobs_for_app "$name"
                    argocd app terminate-op "$full_app" --grpc-web 2>/dev/null || true
                    argocd app get "$full_app" --hard-refresh --grpc-web >/dev/null 2>&1 || true
                    sleep 5
                fi

                # Refresh app if it's degraded or not healthy
                if [[ "$health" == "Degraded" || "$health" == "Progressing" || "$health" != "Healthy" ]]; then
                    echo "$(yellow)[REFRESH] App is $health, checking for unhealthy jobs...$(reset)"

                    # Clean up any unhealthy jobs first
                    clean_unhealthy_jobs_for_app "$name"

                    if (( attempt > 1 )); then
                        # Hard refresh on retry attempts
                        argocd app get "$full_app" --hard-refresh --grpc-web >/dev/null 2>&1 || true
                    else
                        argocd app get "$full_app" --refresh --grpc-web >/dev/null 2>&1 || true
                    fi
                    sleep 5
                fi
            fi

            echo "$(bold)[SYNC] $full_app (wave=$wave) at [$(get_timestamp)]$(reset)"
            echo "$(yellow)[INFO] Attempt ${attempt}/${APP_MAX_RETRIES}, elapsed: 0s$(reset)"

            # Check if app requires server-side apply and special cleanup
            if [[ " $SERVER_SIDE_APPS " =~ \ $name\  ]]; then
                root_app_stop_start
                sleep 3
                echo "$(yellow)[INFO] Stopping any ongoing operations for $name before force sync...$(reset)"
                argocd app terminate-op "$full_app" --grpc-web 2>/dev/null || true
                sleep 2
                
                # Check for OutOfSync or error state resources (Jobs, CRDs, ExternalSecrets, etc.)
                echo "$(yellow)[CLEANUP] Checking for OutOfSync/error resources in $name...$(reset)"
                problem_resources=$(kubectl get applications.argoproj.io "$name" -n "$NS" -o json 2>/dev/null | jq -r '
                    .status.resources[]? |
                    select(.status == "OutOfSync" or .health.status == "Degraded" or .health.status == "Missing") |
                    select(.kind == "Job" or .kind == "CustomResourceDefinition" or .kind == "ExternalSecret" or .kind == "SecretStore" or .kind == "ClusterSecretStore") |
                    "\(.kind) \(.namespace) \(.name)"
                ')
                
                if [[ -n "$problem_resources" ]]; then
                    echo "$(yellow)[DELETE] Removing problem resources before sync...$(reset)"
                    while IFS= read -r res_line; do
                        [[ -z "$res_line" ]] && continue
                        read -r kind res_ns res_name <<< "$res_line"
                        echo "$(yellow)  - Deleting $kind $res_name in $res_ns (background)$(reset)"
                        
                        if [[ "$kind" == "Job" ]]; then
                            kubectl patch job "$res_name" -n "$res_ns" --type=merge -p='{"metadata":{"finalizers":[]}}' 2>/dev/null || true
                            kubectl delete pods -n "$res_ns" -l job-name="$res_name" --ignore-not-found=true --timeout=10s 2>/dev/null &
                            patch_job_pods_remove_finalizers "$res_ns" "$res_name"
                            kubectl delete job "$res_name" -n "$res_ns" --ignore-not-found=true --timeout=10s 2>/dev/null &
                            kubectl patch job "$res_name" -n "$res_ns" --type=merge -p='{"metadata":{"finalizers":[]}}' || true
                        elif [[ "$kind" == "CustomResourceDefinition" ]]; then
                            kubectl patch crd "$res_name" --type=merge -p='{"metadata":{"finalizers":[]}}' 2>/dev/null || true
                            kubectl delete crd "$res_name" --ignore-not-found=true --timeout=10s 2>/dev/null &
                            kubectl patch crd "$res_name" --type=merge -p='{"metadata":{"finalizers":[]}}' || true
                        else
                            kubectl delete "$kind" "$res_name" -n "$res_ns" --ignore-not-found=true --timeout=10s 2>/dev/null &
                            kubectl patch "$kind" "$res_name" -n "$res_ns" --type=merge -p='{"metadata":{"finalizers":[]}}' || true
                        fi
                    done <<< "$problem_resources"
                    echo "$(yellow)[INFO] Waiting for cleanup to complete...$(reset)"
                    sleep 3
                fi
                
                echo "$(yellow)[INFO] Syncing $name with --force --replace --server-side (safer for CRD upgrades)...$(reset)"
                start_ts=$(date +%s)
                LOG=$(argocd app sync "$full_app" --force --replace --server-side --grpc-web 2>&1)
                rc=$?
            else
                # Standard sync for apps not in SERVER_SIDE_APPS
                start_ts=$(date +%s)
                LOG=$(argocd app sync "$full_app" --grpc-web 2>&1)
                rc=$?
            fi

            if [[ $rc -ne 0 ]]; then
                if [[ "$LOG" =~ "deleting" ]]; then
                    echo "$(red)[SKIP] $full_app is deleting. Skipping further attempts.$(reset)"
                    break
                fi
                echo "$(red)[ERROR] Sync command failed, will retry if attempts remain.$(reset)"
                ((attempt++))
                continue
            fi

            #timed_out=false
            while true; do
                now_ts=$(date +%s)
                elapsed=$(( now_ts - start_ts ))
                if (( elapsed >= APP_MAX_WAIT )); then
                    echo "$(red)[TIMEOUT] $full_app did not become Healthy+Synced within ${APP_MAX_WAIT}s.$(reset)"
                    #timed_out=true
                    break
                fi
                status=$(kubectl get applications.argoproj.io "$name" -n "$NS" -o json 2>/dev/null)
                [[ -z "$status" ]] && { sleep "$GLOBAL_POLL_INTERVAL"; continue; }
                health=$(echo "$status" | jq -r '.status.health.status')
                sync=$(echo "$status" | jq -r '.status.sync.status')
                operation_phase=$(echo "$status" | jq -r '.status.operationState.phase // "Unknown"')

                # Check for failed jobs/CRDs during sync
                failed_jobs=$(echo "$status" | jq -r '
                    .status.resources[]? |
                    select(.kind == "Job" and .health.status == "Degraded") |
                    .name
                ' | wc -l)

                if [[ $failed_jobs -gt 0 ]]; then
                    echo "$(red)[ERROR] $full_app has $failed_jobs failed job(s), triggering cleanup and restart...$(reset)"
                    # Clean up failed jobs and restart sync
                    clean_unhealthy_jobs_for_app "$name"
                    argocd app terminate-op "$full_app" --grpc-web 2>/dev/null || true
                    argocd app get "$full_app" --hard-refresh --grpc-web >/dev/null 2>&1 || true
                    sleep 3
                    argocd app sync "$full_app" --grpc-web 2>&1 || true
                    start_ts=$(date +%s)  # Reset timer
                    sleep "$GLOBAL_POLL_INTERVAL"
                    continue
                fi

                # Check if sync operation failed
                if [[ "$operation_phase" == "Failed" || "$operation_phase" == "Error" ]]; then
                    echo "$(red)[ERROR] $full_app sync operation failed with phase=$operation_phase at [$(get_timestamp)]$(reset)"
                    #timed_out=true
                    break
                fi

                print_table_row "$wave" "$name" "$health" "$sync"
                echo "    [$(get_timestamp)] Elapsed: ${elapsed}s"
                if [[ "$health" == "Healthy" && "$sync" == "Synced" ]]; then
                    echo "$(green)[DONE] $full_app Healthy+Synced in ${elapsed}s at [$(get_timestamp)] (attempt ${attempt})$(reset)"
                    console_success "[✓] $name - Completed (${elapsed}s)"
                    synced=true
                    break
                fi
                sleep "$GLOBAL_POLL_INTERVAL"
            done
            if [[ "$synced" == "true" ]]; then
                break
            fi
            ((attempt++))
            
            # After 3 failed attempts, delete app and restart root-app before continuing
            if (( attempt == 4 )); then
                echo "$(yellow)[ACTION] After 3 failed attempts, deleting app and restarting root-app...$(reset)"
                
                # Delete the failed application (already handles root-app sync internally)
                delete_app "$name" "$NS"
                
                echo "$(blue)[INFO] Application $name deleted and root-app restarted. Continuing with retries...$(reset)"
            fi
            
            if (( attempt <= APP_MAX_RETRIES )); then
                echo "$(yellow)[RETRY] Retrying $full_app (${attempt}/${APP_MAX_RETRIES})...$(reset)"
                # On retry, clean up unhealthy jobs and clear stuck operations
                clean_unhealthy_jobs_for_app "$name"
                argocd app terminate-op "$full_app" --grpc-web 2>/dev/null || true
                argocd app get "$full_app" --hard-refresh --grpc-web >/dev/null 2>&1 || true
                sleep 5
            else
                echo "$(red)[FAIL] Max retries (${APP_MAX_RETRIES}) reached for $full_app. Proceeding to next app.$(reset)"
                console_error "[✗] $name - Failed"
            fi
        done
        echo "$(blue)[INFO] Proceeding to next app...$(reset)"
    done

    # Now handle root-app sync after all other apps
    console_info "[→] Syncing: root-app"
    status=$(kubectl get applications.argoproj.io "root-app" -n "$NS" -o json 2>/dev/null)
    if [[ -z "$status" ]]; then
        echo "$(red)[FAIL] root-app not found in namespace '$NS'.$(reset)"
        return 1
    fi
    health=$(echo "$status" | jq -r '.status.health.status')
    sync=$(echo "$status" | jq -r '.status.sync.status')
    wave=$(echo "$status" | jq -r '.metadata.annotations["argocd.argoproj.io/sync-wave"] // "0"')
    full_app="${NS}/root-app"

    attempt=1
    synced=false
    while (( attempt <= APP_MAX_RETRIES )); do
        last_sync_status=$(echo "$status" | jq -r '.status.operationState.phase // "Unknown"')
        last_sync_time=$(echo "$status" | jq -r '.status.operationState.finishedAt // "N/A"')

        echo "[$(get_timestamp)] root-app Status: Health=$health Sync=$sync LastSync=$last_sync_status Time=$last_sync_time"

        if [[ "$health" == "Healthy" && "$sync" == "Synced" ]]; then
            echo "$(green)[OK] $full_app (wave=$wave) already Healthy+Synced$(reset)"
            synced=true
            break
        fi

        # Check if last sync failed and clean up
        if [[ "$last_sync_status" == "Failed" || "$last_sync_status" == "Error" ]]; then
            echo "$(red)[CLEANUP] Last sync failed for root-app, cleaning up stuck resources...$(reset)"
            clean_unhealthy_jobs_for_app "root-app"
            argocd app terminate-op "$full_app" --grpc-web 2>/dev/null || true
            argocd app get "$full_app" --hard-refresh --grpc-web >/dev/null 2>&1 || true
            sleep 5
        fi

        # Refresh root-app if it's degraded or not healthy
        if [[ "$health" == "Degraded" || "$health" == "Progressing" || "$health" != "Healthy" ]]; then
            echo "$(yellow)[REFRESH] root-app is $health, refreshing before sync...$(reset)"
            if (( attempt > 1 )); then
                argocd app get "$full_app" --hard-refresh --grpc-web >/dev/null 2>&1 || true
            else
                argocd app get "$full_app" --refresh --grpc-web >/dev/null 2>&1 || true
            fi
            sleep 5
        fi

        echo "$(bold)[SYNC] $full_app (wave=$wave) at [$(get_timestamp)]$(reset)"
        echo "$(yellow)[INFO] Attempt ${attempt}/${APP_MAX_RETRIES}, elapsed: 0s$(reset)"

        # Stop any ongoing operations and refresh before sync
        echo "[INFO] Stopping ongoing operations and refreshing before sync..."
        argocd app terminate-op "$full_app" --grpc-web 2>/dev/null || true
        sleep 2
        argocd app get "$full_app" --refresh --grpc-web >/dev/null 2>&1 || true
        sleep 3

        start_ts=$(date +%s)
        LOG=$(argocd app sync "$full_app" --grpc-web 2>&1)
        rc=$?

        if [[ $rc -ne 0 ]]; then
            if [[ "$LOG" =~ "deleting" ]]; then
                echo "$(red)[SKIP] $full_app is deleting. Skipping further attempts.$(reset)"
                break
            fi
            echo "$(red)[ERROR] Sync command failed, will retry if attempts remain.$(reset)"
            ((attempt++))
            continue
        fi

        #timed_out=false
        while true; do
            now_ts=$(date +%s)
            elapsed=$(( now_ts - start_ts ))
            if (( elapsed >= APP_MAX_WAIT )); then
                echo "$(red)[TIMEOUT] $full_app did not become Healthy+Synced within ${APP_MAX_WAIT}s.$(reset)"
                #timed_out=true
                break
            fi
            status=$(kubectl get applications.argoproj.io "root-app" -n "$NS" -o json 2>/dev/null)
            [[ -z "$status" ]] && { sleep "$GLOBAL_POLL_INTERVAL"; continue; }
            health=$(echo "$status" | jq -r '.status.health.status')
            sync=$(echo "$status" | jq -r '.status.sync.status')
            print_table_row "$wave" "root-app" "$health" "$sync"
            echo "    Elapsed: ${elapsed}s"
            if [[ "$health" == "Healthy" && "$sync" == "Synced" ]]; then
                echo "$(green)[DONE] $full_app Healthy+Synced in ${elapsed}s (attempt ${attempt})$(reset)"
                console_success "[✓] root-app - Completed (${elapsed}s)"
                synced=true
                break
            fi
            sleep "$GLOBAL_POLL_INTERVAL"
        done
        if [[ "$synced" == "true" ]]; then
            break
        fi
        ((attempt++))
        if (( attempt <= APP_MAX_RETRIES )); then
            echo "$(yellow)[RETRY] Retrying $full_app (${attempt}/${APP_MAX_RETRIES})...$(reset)"
        else
            echo "$(red)[FAIL] Max retries reached for $full_app.$(reset)"
            console_error "[✗] root-app - Failed"
        fi
    done
    echo "$(blue)[INFO] Finished root-app sync attempt(s).$(reset)"
}

# ============================================================
# Sync all apps except root-app (wave order, nice reporting)
# ============================================================
sync_all_apps_exclude_root() {
    mapfile -t all_apps < <(get_all_apps_by_wave)
    [[ ${#all_apps[@]} -eq 0 ]] && { echo "[WARN] No applications found in namespace '$NS'."; return 0; }

    print_header "Applications (Wave-Ordered Status, excluding root-app)"
    print_table_header
    for line in "${all_apps[@]}"; do
        read -r wave name health sync <<< "$line"
        if [[ "$name" != "root-app" ]]; then
            print_table_row "$wave" "$name" "$health" "$sync"
        fi
    done
    echo

    # Print summary of NOT-GREEN apps before syncing
    echo "$(bold)[INFO] Apps NOT Healthy or NOT Synced (excluding root-app):$(reset)"
    for line in "${all_apps[@]}"; do
        read -r wave name health sync <<< "$line"
        if [[ "$name" != "root-app" && ( "$health" != "Healthy" || "$sync" != "Synced" ) ]]; then
            echo "$(red)  - $name (wave=$wave) Health=$health Sync=$sync$(reset)"
        fi
    done
    echo

    # Sync NOT-GREEN apps in wave order, skipping root-app
    for line in "${all_apps[@]}"; do
        read -r wave name health sync <<< "$line"
        full_app="${NS}/${name}"

        if [[ "$name" == "root-app" ]]; then
            continue
        fi

        # Console: Show which app is being processed
        console_info "[→] Syncing: $name (wave=$wave)"

        # First check and handle any failed syncs
        echo "[$(get_timestamp)] Checking for failed syncs in $name..."
        check_and_handle_failed_sync "$name"

        # Check for CRD version mismatches
        echo "[$(get_timestamp)] Checking for CRD version mismatches in $name..."
        check_and_fix_crd_version_mismatch "$name"

        # Special pre-sync handling for haproxy-ingress-pxe-boots
        if [[ "$name" == "haproxy-ingress-pxe-boots" ]]; then
            echo "$(yellow)[INFO] Pre-sync: haproxy-ingress-pxe-boots detected - deleting tls-boots secret first...$(reset)"
            kubectl delete secret tls-boots -n orch-boots 2>/dev/null || true
            sleep 3
        fi

        attempt=1
        synced=false
        while (( attempt <= APP_MAX_RETRIES )); do
            status=$(kubectl get applications.argoproj.io "$name" -n "$NS" -o json 2>/dev/null)
            if [[ -n "$status" ]]; then
                health=$(echo "$status" | jq -r '.status.health.status')
                sync=$(echo "$status" | jq -r '.status.sync.status')
                last_sync_status=$(echo "$status" | jq -r '.status.operationState.phase // "Unknown"')
                last_sync_time=$(echo "$status" | jq -r '.status.operationState.finishedAt // "N/A"')

                echo "[$(get_timestamp)] $full_app Status: Health=$health Sync=$sync LastSync=$last_sync_status Time=$last_sync_time"

                if [[ "$health" == "Healthy" && "$sync" == "Synced" ]]; then
                    echo "$(green)[OK] $full_app (wave=$wave) already Healthy+Synced$(reset)"
                    synced=true
                    break
                fi

                # Check if last sync failed and clean up
                if [[ "$last_sync_status" == "Failed" || "$last_sync_status" == "Error" ]]; then
                    echo "$(red)[CLEANUP] Last sync failed for $full_app, cleaning up stuck resources...$(reset)"
                    clean_unhealthy_jobs_for_app "$name"
                    argocd app terminate-op "$full_app" --grpc-web 2>/dev/null || true
                    argocd app get "$full_app" --hard-refresh --grpc-web >/dev/null 2>&1 || true
                    sleep 5
                fi

                # Refresh app if it's degraded or not healthy
                if [[ "$health" == "Degraded" || "$health" == "Progressing" || "$health" != "Healthy" ]]; then
                    echo "$(yellow)[REFRESH] App is $health, checking for unhealthy jobs...$(reset)"

                    # Clean up any unhealthy jobs first
                    clean_unhealthy_jobs_for_app "$name"

                    if (( attempt > 1 )); then
                        # Hard refresh on retry attempts
                        argocd app get "$full_app" --hard-refresh --grpc-web >/dev/null 2>&1 || true
                    else
                        argocd app get "$full_app" --refresh --grpc-web >/dev/null 2>&1 || true
                    fi
                    sleep 5
                fi
            fi

            echo "$(bold)[SYNC] $full_app (wave=$wave) at [$(get_timestamp)]$(reset)"
            echo "$(yellow)[INFO] Attempt ${attempt}/${APP_MAX_RETRIES}, elapsed: 0s$(reset)"

            # Check if app requires server-side apply and special cleanup
            if [[ " $SERVER_SIDE_APPS " =~ \ $name\  ]]; then
                root_app_stop_start
                sleep 3
                echo "$(yellow)[INFO] Stopping any ongoing operations for $name before force sync...$(reset)"
                argocd app terminate-op "$full_app" --grpc-web 2>/dev/null || true
                sleep 2
                
                # Check for OutOfSync or error state resources (Jobs, CRDs, ExternalSecrets, etc.)
                echo "$(yellow)[CLEANUP] Checking for OutOfSync/error resources in $name...$(reset)"
                problem_resources=$(kubectl get applications.argoproj.io "$name" -n "$NS" -o json 2>/dev/null | jq -r '
                    .status.resources[]? |
                    select(.status == "OutOfSync" or .health.status == "Degraded" or .health.status == "Missing") |
                    select(.kind == "Job" or .kind == "CustomResourceDefinition" or .kind == "ExternalSecret" or .kind == "SecretStore" or .kind == "ClusterSecretStore") |
                    "\(.kind) \(.namespace) \(.name)"
                ')
                
                if [[ -n "$problem_resources" ]]; then
                    echo "$(yellow)[DELETE] Removing problem resources before sync...$(reset)"
                    while IFS= read -r res_line; do
                        [[ -z "$res_line" ]] && continue
                        read -r kind res_ns res_name <<< "$res_line"
                        echo "$(yellow)  - Deleting $kind $res_name in $res_ns (background)$(reset)"
                        
                        if [[ "$kind" == "Job" ]]; then
                            kubectl patch job "$res_name" -n "$res_ns" --type=merge -p='{"metadata":{"finalizers":[]}}' 2>/dev/null || true
                            kubectl delete pods -n "$res_ns" -l job-name="$res_name" --ignore-not-found=true --timeout=10s 2>/dev/null &
                            patch_job_pods_remove_finalizers "$res_ns" "$res_name"
                            kubectl delete job "$res_name" -n "$res_ns" --ignore-not-found=true --timeout=10s 2>/dev/null &
                            kubectl patch job "$res_name" -n "$res_ns" --type=merge -p='{"metadata":{"finalizers":[]}}' || true
                        elif [[ "$kind" == "CustomResourceDefinition" ]]; then
                            kubectl patch crd "$res_name" --type=merge -p='{"metadata":{"finalizers":[]}}' 2>/dev/null || true
                            kubectl delete crd "$res_name" --ignore-not-found=true --timeout=10s 2>/dev/null &
                            kubectl patch crd "$res_name" --type=merge -p='{"metadata":{"finalizers":[]}}' || true
                        else
                            kubectl delete "$kind" "$res_name" -n "$res_ns" --ignore-not-found=true --timeout=10s 2>/dev/null &
                            kubectl patch "$kind" "$res_name" -n "$res_ns" --type=merge -p='{"metadata":{"finalizers":[]}}' || true
                        fi
                    done <<< "$problem_resources"
                    echo "$(yellow)[INFO] Waiting for cleanup to complete...$(reset)"
                    sleep 3
                    
                    # Verify resources are deleted, if still present, force finalizer removal
                    echo "$(yellow)[VERIFY] Checking if resources were successfully deleted...$(reset)"
                    while IFS= read -r res_line; do
                        [[ -z "$res_line" ]] && continue
                        read -r kind res_ns res_name <<< "$res_line"
                        
                        if [[ "$kind" == "Job" ]]; then
                            if kubectl get job "$res_name" -n "$res_ns" &>/dev/null; then
                                echo "$(red)[STUCK] Job $res_name still exists, forcing finalizer removal...$(reset)"
                                kubectl patch job "$res_name" -n "$res_ns" --type=json -p='[{"op":"remove","path":"/metadata/finalizers"}]' 2>/dev/null || true
                                kubectl delete job "$res_name" -n "$res_ns" --force --grace-period=0 2>/dev/null || true &
                            fi
                        elif [[ "$kind" == "CustomResourceDefinition" ]]; then
                            if kubectl get crd "$res_name" &>/dev/null; then
                                echo "$(red)[STUCK] CRD $res_name still exists, forcing finalizer removal...$(reset)"
                                kubectl patch crd "$res_name" --type=json -p='[{"op":"remove","path":"/metadata/finalizers"}]' 2>/dev/null || true
                                kubectl delete crd "$res_name" --force --grace-period=0 2>/dev/null || true &
                            fi
                        elif [[ "$kind" == "ExternalSecret" || "$kind" == "SecretStore" || "$kind" == "ClusterSecretStore" ]]; then
                            if kubectl get "$kind" "$res_name" -n "$res_ns" &>/dev/null; then
                                echo "$(red)[STUCK] $kind $res_name still exists, forcing finalizer removal...$(reset)"
                                kubectl patch "$kind" "$res_name" -n "$res_ns" --type=json -p='[{"op":"remove","path":"/metadata/finalizers"}]' 2>/dev/null || true
                                kubectl delete "$kind" "$res_name" -n "$res_ns" --force --grace-period=0 2>/dev/null || true &
                            fi
                        fi
                    done <<< "$problem_resources"
                    sleep 2
                fi
                
                echo "$(yellow)[INFO] Syncing $name with --force --replace --server-side (safer for CRD upgrades)...$(reset)"
                start_ts=$(date +%s)
                LOG=$(argocd app sync "$full_app" --force --replace --server-side --grpc-web 2>&1)
                rc=$?
            else
                # Standard sync for apps not in SERVER_SIDE_APPS
                start_ts=$(date +%s)
                LOG=$(argocd app sync "$full_app" --grpc-web 2>&1)
                rc=$?
            fi

            if [[ $rc -ne 0 ]]; then
                if [[ "$LOG" =~ "deleting" ]]; then
                    echo "$(red)[SKIP] $full_app is deleting. Skipping further attempts.$(reset)"
                    break
                fi
                echo "$(red)[ERROR] Sync command failed, will retry if attempts remain.$(reset)"
                ((attempt++))
                continue
            fi

            #timed_out=false
            while true; do
                now_ts=$(date +%s)
                elapsed=$(( now_ts - start_ts ))
                if (( elapsed >= APP_MAX_WAIT )); then
                    echo "$(red)[TIMEOUT] $full_app did not become Healthy+Synced within ${APP_MAX_WAIT}s.$(reset)"
                    #timed_out=true
                    break
                fi
                status=$(kubectl get applications.argoproj.io "$name" -n "$NS" -o json 2>/dev/null)
                [[ -z "$status" ]] && { sleep "$GLOBAL_POLL_INTERVAL"; continue; }
                health=$(echo "$status" | jq -r '.status.health.status')
                sync=$(echo "$status" | jq -r '.status.sync.status')
                operation_phase=$(echo "$status" | jq -r '.status.operationState.phase // "Unknown"')

                # Check for failed jobs/CRDs during sync
                failed_jobs=$(echo "$status" | jq -r '
                    .status.resources[]? |
                    select(.kind == "Job" and .health.status == "Degraded") |
                    .name
                ' | wc -l)

                if [[ $failed_jobs -gt 0 ]]; then
                    echo "$(red)[ERROR] $full_app has $failed_jobs failed job(s), triggering cleanup and restart...$(reset)"
                    # Clean up failed jobs and restart sync
                    clean_unhealthy_jobs_for_app "$name"
                    argocd app terminate-op "$full_app" --grpc-web 2>/dev/null || true
                    argocd app get "$full_app" --hard-refresh --grpc-web >/dev/null 2>&1 || true
                    sleep 3
                    argocd app sync "$full_app" --grpc-web 2>&1 || true
                    start_ts=$(date +%s)  # Reset timer
                    sleep "$GLOBAL_POLL_INTERVAL"
                    continue
                fi

                # Check if sync operation failed
                if [[ "$operation_phase" == "Failed" || "$operation_phase" == "Error" ]]; then
                    echo "$(red)[ERROR] $full_app sync operation failed with phase=$operation_phase$(reset)"
                    #timed_out=true
                    break
                fi

                print_table_row "$wave" "$name" "$health" "$sync"
                echo "    Elapsed: ${elapsed}s"
                if [[ "$health" == "Healthy" && "$sync" == "Synced" ]]; then
                    echo "$(green)[DONE] $full_app Healthy+Synced in ${elapsed}s (attempt ${attempt})$(reset)"
                    console_success "[✓] $name - Completed (${elapsed}s)"
                    synced=true
                    break
                fi
                sleep "$GLOBAL_POLL_INTERVAL"
            done
            if [[ "$synced" == "true" ]]; then
                break
            fi
            ((attempt++))
            if (( attempt <= APP_MAX_RETRIES )); then
                echo "$(yellow)[RETRY] Retrying $full_app (${attempt}/${APP_MAX_RETRIES})...$(reset)"
                # On retry, clean up unhealthy jobs and clear stuck operations
                clean_unhealthy_jobs_for_app "$name"
                argocd app terminate-op "$full_app" --grpc-web 2>/dev/null || true
                argocd app get "$full_app" --hard-refresh --grpc-web >/dev/null 2>&1 || true
                sleep 5
            else
                echo "$(red)[FAIL] Max retries reached for $full_app. Proceeding to next app.$(reset)"
                console_error "[✗] $name - Failed"
            fi
        done
        echo "$(blue)[INFO] Proceeding to next app...$(reset)"
    done
}

# ============================================================
# Sync root-app only (with nice reporting)
# ============================================================
sync_root_app_only() {
    console_info "[→] Syncing: root-app"
    status=$(kubectl get applications.argoproj.io "root-app" -n "$NS" -o json 2>/dev/null)
    if [[ -z "$status" ]]; then
        echo "$(red)[FAIL] root-app not found in namespace '$NS'.$(reset)"
        return 1
    fi
    health=$(echo "$status" | jq -r '.status.health.status')
    sync=$(echo "$status" | jq -r '.status.sync.status')
    wave=$(echo "$status" | jq -r '.metadata.annotations["argocd.argoproj.io/sync-wave"] // "0"')
    full_app="${NS}/root-app"

    print_header "root-app Status"
    print_table_header
    print_table_row "$wave" "root-app" "$health" "$sync"
    echo

    # First check and handle any failed syncs
    echo "[$(get_timestamp)] Checking for failed syncs in root-app..."
    check_and_handle_failed_sync "root-app"

    # Check for CRD version mismatches
    echo "[$(get_timestamp)] Checking for CRD version mismatches in root-app..."
    check_and_fix_crd_version_mismatch "root-app"

    last_sync_status=$(echo "$status" | jq -r '.status.operationState.phase // "Unknown"')
    last_sync_time=$(echo "$status" | jq -r '.status.operationState.finishedAt // "N/A"')

    echo "[$(get_timestamp)] root-app Status: Health=$health Sync=$sync LastSync=$last_sync_status Time=$last_sync_time"

    if [[ "$health" == "Healthy" && "$sync" == "Synced" ]]; then
        echo "$(green)[OK] $full_app (wave=$wave) already Healthy+Synced$(reset)"
        console_success "[✓] root-app - Already synced"
        return 0
    fi

    # Check if last sync failed and clean up
    if [[ "$last_sync_status" == "Failed" || "$last_sync_status" == "Error" ]]; then
        echo "$(red)[CLEANUP] Last sync failed for root-app, cleaning up stuck resources...$(reset)"
        clean_unhealthy_jobs_for_app "root-app"
        argocd app terminate-op "$full_app" --grpc-web 2>/dev/null || true
        argocd app get "$full_app" --hard-refresh --grpc-web >/dev/null 2>&1 || true
        sleep 5
    fi

    echo "$(bold)[SYNC] $full_app (wave=$wave) at [$(get_timestamp)]$(reset)"
    attempt=1
    synced=false
    while (( attempt <= APP_MAX_RETRIES )); do
        # Refresh root-app if it's degraded or not healthy
        if [[ "$health" == "Degraded" || "$health" == "Progressing" || "$health" != "Healthy" ]]; then
            echo "$(yellow)[REFRESH] root-app is $health, refreshing before sync...$(reset)"
            if (( attempt > 1 )); then
                argocd app get "$full_app" --hard-refresh --grpc-web >/dev/null 2>&1 || true
            else
                argocd app get "$full_app" --refresh --grpc-web >/dev/null 2>&1 || true
            fi
            sleep 5
        fi

        echo "$(yellow)[INFO] Attempt ${attempt}/${APP_MAX_RETRIES}, elapsed: 0s$(reset)"

        # Stop any ongoing operations and refresh before sync
        echo "[INFO] Stopping ongoing operations and refreshing before sync..."
        argocd app terminate-op "$full_app" --grpc-web 2>/dev/null || true
        sleep 2
        argocd app get "$full_app" --refresh --grpc-web >/dev/null 2>&1 || true
        sleep 3

        start_ts=$(date +%s)
        LOG=$(argocd app sync "$full_app" --grpc-web 2>&1)
        rc=$?

        if [[ $rc -ne 0 ]]; then
            if [[ "$LOG" =~ "deleting" ]]; then
                echo "$(red)[SKIP] $full_app is deleting. Skipping further attempts.$(reset)"
                break
            fi
            echo "$(red)[ERROR] Sync command failed, will retry if attempts remain.$(reset)"
            ((attempt++))
            continue
        fi

        #timed_out=false
        while true; do
            now_ts=$(date +%s)
            elapsed=$(( now_ts - start_ts ))
            if (( elapsed >= APP_MAX_WAIT )); then
                echo "$(red)[TIMEOUT] $full_app did not become Healthy+Synced within ${APP_MAX_WAIT}s.$(reset)"
                #timed_out=true
                break
            fi
            status=$(kubectl get applications.argoproj.io "root-app" -n "$NS" -o json 2>/dev/null)
            [[ -z "$status" ]] && { sleep "$GLOBAL_POLL_INTERVAL"; continue; }
            health=$(echo "$status" | jq -r '.status.health.status')
            sync=$(echo "$status" | jq -r '.status.sync.status')
            operation_phase=$(echo "$status" | jq -r '.status.operationState.phase // "Unknown"')

            # Check if sync operation failed
            if [[ "$operation_phase" == "Failed" || "$operation_phase" == "Error" ]]; then
                echo "$(red)[ERROR] $full_app sync operation failed with phase=$operation_phase$(reset)"
                #timed_out=true
                break
            fi

            print_table_row "$wave" "root-app" "$health" "$sync"
            echo "    Elapsed: ${elapsed}s"
            if [[ "$health" == "Healthy" && "$sync" == "Synced" ]]; then
                echo "$(green)[DONE] $full_app Healthy+Synced in ${elapsed}s (attempt ${attempt})$(reset)"
                console_success "[✓] root-app - Completed (${elapsed}s)"
                synced=true
                break
            fi
            sleep "$GLOBAL_POLL_INTERVAL"
        done
        if [[ "$synced" == "true" ]]; then
            break
        fi
        ((attempt++))
        if (( attempt <= APP_MAX_RETRIES )); then
            echo "$(yellow)[RETRY] Retrying $full_app (${attempt}/${APP_MAX_RETRIES})...$(reset)"
            # On retry, clean up unhealthy jobs and clear stuck operations
            clean_unhealthy_jobs_for_app "root-app"
            argocd app terminate-op "$full_app" --grpc-web 2>/dev/null || true
            argocd app get "$full_app" --hard-refresh --grpc-web >/dev/null 2>&1 || true
            sleep 5
        else
            echo "$(red)[FAIL] Max retries reached for $full_app.$(reset)"
            console_error "[✗] root-app - Failed"
        fi

        # Re-fetch status for next iteration
        status=$(kubectl get applications.argoproj.io "root-app" -n "$NS" -o json 2>/dev/null)
        if [[ -n "$status" ]]; then
            health=$(echo "$status" | jq -r '.status.health.status')
            sync=$(echo "$status" | jq -r '.status.sync.status')
        fi
    done
    echo "$(blue)[INFO] Finished root-app sync attempt(s).$(reset)"
}

# ============================================================
# Wait until NS is all green (excluding root-app)
# ============================================================
namespace_all_green_exclude_root() {
    kubectl get applications.argoproj.io -n "$NS" -o json \
    | jq -r '
        .items[] |
        select(.metadata.name != "root-app") |
        {
            health: .status.health.status,
            sync: .status.sync.status
        }
        | select(.health != "Healthy" or .sync != "Synced")
    ' | grep -q .
    return $?
}

sync_until_green_ns_exclude_root() {
    local retry_count=0
    local max_retries=2
    
    console_info "[→] Starting sync for all applications (excluding root-app)..."
    
    while (( retry_count < max_retries )); do
        if ! namespace_all_green_exclude_root; then
            print_header "All non-root-app applications are Healthy+Synced in namespace '$NS'."
            console_success "[✓] All non-root applications are synced"
            break
        fi

        print_header "NOT-GREEN apps (Wave-Ordered, excluding root-app) - Attempt $((retry_count + 1))/${max_retries}"
        console_info "[→] Global retry attempt $((retry_count + 1))/${max_retries}"
        print_table_header
        mapfile -t not_green < <(kubectl get applications.argoproj.io -n "$NS" -o json \
            | jq -r '.items[] | select(.metadata.name != "root-app") | {
                name: .metadata.name,
                wave: (.metadata.annotations["argocd.argoproj.io/sync-wave"] // "0"),
                health: .status.health.status,
                sync: .status.sync.status
            } | "\(.wave) \(.name) \(.health) \(.sync)"' | sort -n -k1)
        for line in "${not_green[@]}"; do
            read -r wave name health sync <<< "$line"
            print_table_row "$wave" "$name" "$health" "$sync"
        done
        echo

        sync_all_apps_exclude_root

        ((retry_count++))
    done
    
    if (( retry_count >= max_retries )) && namespace_all_green_exclude_root; then
        echo "$(yellow)[WARN] Maximum retries (${max_retries}) reached but some apps are still not green$(reset)"
        console_warn "[!] Maximum retries reached - some apps still not synced"
    fi
}


# ============================================================
# Check and delete stuck/out-of-sync dependent CRD jobs
# ============================================================
check_and_delete_stuck_crd_jobs() {
    print_header "Checking for stuck/out-of-sync dependent CRD jobs"

    # Check for stuck jobs in all namespaces
    echo "[INFO] Looking for stuck or failed jobs..."

    # Get jobs that are not completed or have failed
    stuck_jobs=$(kubectl get jobs --all-namespaces -o json | jq -r '
        .items[] |
        select(.status.succeeded != 1 and (.status.failed > 0 or .status.active > 0)) |
        "\(.metadata.namespace) \(.metadata.name)"
    ')

    if [[ -n "$stuck_jobs" ]]; then
        echo "$(yellow)[WARN] Found stuck/failed jobs:$(reset)"
        echo "$stuck_jobs"

        # Delete stuck jobs and their pods
        while IFS= read -r line; do
            [[ -z "$line" ]] && continue
            read -r job_ns job_name <<< "$line"
            echo "$(yellow)[CLEANUP] Deleting stuck job $job_name in namespace $job_ns (background)$(reset)"

            # Remove finalizers first to allow deletion
            patch_job_pods_remove_finalizers "$job_ns" "$job_name"
            kubectl patch job "$job_name" -n "$job_ns" --type=merge -p='{"metadata":{"finalizers":[]}}' 2>/dev/null || true

            # Delete associated pods first
            kubectl delete pods -n "$job_ns" -l job-name="$job_name" --ignore-not-found=true 2>/dev/null &

            # Delete the job
            kubectl delete job "$job_name" -n "$job_ns" --ignore-not-found=true &
        done <<< "$stuck_jobs"

        echo "[INFO] Job cleanup initiated in background, proceeding..."
    else
        echo "$(green)[OK] No stuck jobs found$(reset)"
    fi

    # Check for applications that are OutOfSync
    echo "[INFO] Looking for OutOfSync applications..."
    out_of_sync_apps=$(kubectl get applications.argoproj.io -n "$NS" -o json | jq -r '
        .items[] |
        select(.status.sync.status == "OutOfSync") |
        .metadata.name
    ')

    if [[ -n "$out_of_sync_apps" ]]; then
        echo "$(yellow)[WARN] Found OutOfSync applications:$(reset)"
        echo "$out_of_sync_apps"

        # Stop and restart sync for OutOfSync apps
        while IFS= read -r app_name; do
            [[ -z "$app_name" ]] && continue
            echo "$(yellow)[CLEANUP] Stopping sync for $app_name$(reset)"
            argocd app terminate-op "${NS}/${app_name}" --grpc-web 2>/dev/null || true
            sleep 2
        done <<< "$out_of_sync_apps"
    else
        echo "$(green)[OK] No OutOfSync applications found$(reset)"
    fi

    # Check for applications with sync failures
    echo "[INFO] Looking for applications with sync failures..."
    sync_failed_apps=$(kubectl get applications.argoproj.io -n "$NS" -o json | jq -r '
        .items[] |
        select(.status.operationState.phase == "Failed" or .status.operationState.phase == "Error") |
        "\(.metadata.name) \(.status.operationState.phase)"
    ')

    if [[ -n "$sync_failed_apps" ]]; then
        echo "$(red)[WARN] Found applications with sync failures:$(reset)"
        echo "$sync_failed_apps"

        # Clean up failed apps
        while IFS= read -r line; do
            [[ -z "$line" ]] && continue
            read -r app_name phase <<< "$line"
            echo "$(red)[CLEANUP] App $app_name has phase=$phase, cleaning up...$(reset)"

            # Clean up unhealthy jobs for this app
            clean_unhealthy_jobs_for_app "$app_name"

            # Terminate any stuck operations
            argocd app terminate-op "${NS}/${app_name}" --grpc-web 2>/dev/null || true

            # Hard refresh to clear the error state
            argocd app get "${NS}/${app_name}" --hard-refresh --grpc-web >/dev/null 2>&1 || true

            sleep 2
        done <<< "$sync_failed_apps"
    else
        echo "$(green)[OK] No sync failed applications found$(reset)"
    fi

    echo "[INFO] Stuck CRD jobs check and cleanup completed."
}

# ============================================================
# List all pods in unhealthy state
# ============================================================
list_unhealthy_pods() {
    print_header "Listing Pods in Unhealthy State"
    console_info "[→] Checking for unhealthy pods..."

    echo "[INFO] Looking for pods in unhealthy states across all namespaces..."

    # Get all pods in unhealthy states (CrashLoopBackOff, Error, ImagePullBackOff, etc.)
    unhealthy_pods=$(kubectl get pods --all-namespaces -o json | jq -r '
        .items[] |
        select(
            .status.phase != "Running" and .status.phase != "Succeeded" or
            (.status.containerStatuses[]? | 
                select(
                    .state.waiting?.reason == "CrashLoopBackOff" or
                    .state.waiting?.reason == "ImagePullBackOff" or
                    .state.waiting?.reason == "ErrImagePull" or
                    .state.waiting?.reason == "Error" or
                    .state.terminated?.reason == "Error" or
                    .ready == false
                )
            )
        ) |
        "\(.metadata.namespace)|\(.metadata.name)|\(.status.phase)|\(
            if .status.containerStatuses then
                (.status.containerStatuses[] | 
                    if .state.waiting then .state.waiting.reason
                    elif .state.terminated then .state.terminated.reason
                    elif .ready == false then "NotReady"
                    else "Unknown"
                    end
                )
            else "Unknown"
            end
        )"
    ')

    if [[ -n "$unhealthy_pods" ]]; then
        echo ""
        echo "$(bold)$(red)List of Unhealthy Pods:$(reset)"
        echo "--------------------------------------------------------------------------------"
        printf "%-30s %-35s %-15s %-20s\n" "Namespace" "Pod Name" "Phase" "Status/Reason"
        echo "--------------------------------------------------------------------------------"
        
        local pod_count=0
        
        while IFS='|' read -r pod_ns pod_name pod_phase pod_status; do
            [[ -z "$pod_ns" ]] && continue
            ((pod_count++))
            printf "%-30s %-35s %-15s %-20s\n" "$pod_ns" "$pod_name" "$pod_phase" "$pod_status"
        done <<< "$unhealthy_pods"
        
        echo "--------------------------------------------------------------------------------"
        echo ""
        echo "$(red)[SUMMARY] Found $pod_count unhealthy pod(s)$(reset)"
        console_warn "[!] Found $pod_count unhealthy pod(s)"
        
        return 0
    else
        echo "$(green)[OK] No unhealthy pods found$(reset)"
        console_success "[✓] All pods are healthy"
        return 1
    fi
}

# ============================================================
# Delete all pods in unhealthy state
# ============================================================
delete_unhealthy_pods() {
    print_header "Deleting Pods in Unhealthy State"
    console_info "[→] Checking and deleting unhealthy pods..."

    echo "[INFO] Looking for pods in unhealthy states across all namespaces..."

    # Get all pods in unhealthy states (CrashLoopBackOff, Error, ImagePullBackOff, etc.)
    unhealthy_pods=$(kubectl get pods --all-namespaces -o json | jq -r '
        .items[] |
        select(
            .status.phase != "Running" and .status.phase != "Succeeded" or
            (.status.containerStatuses[]? | 
                select(
                    .state.waiting?.reason == "CrashLoopBackOff" or
                    .state.waiting?.reason == "ImagePullBackOff" or
                    .state.waiting?.reason == "ErrImagePull" or
                    .state.waiting?.reason == "Error" or
                    .state.terminated?.reason == "Error" or
                    .ready == false
                )
            )
        ) |
        "\(.metadata.namespace)|\(.metadata.name)|\(.status.phase)|\(
            if .status.containerStatuses then
                (.status.containerStatuses[] | 
                    if .state.waiting then .state.waiting.reason
                    elif .state.terminated then .state.terminated.reason
                    elif .ready == false then "NotReady"
                    else "Unknown"
                    end
                )
            else "Unknown"
            end
        )"
    ')

    if [[ -n "$unhealthy_pods" ]]; then
        echo ""
        echo "$(bold)$(red)List of Unhealthy Pods to be Deleted:$(reset)"
        echo "--------------------------------------------------------------------------------"
        printf "%-30s %-35s %-15s %-20s\n" "Namespace" "Pod Name" "Phase" "Status/Reason"
        echo "--------------------------------------------------------------------------------"
        
        local pod_count=0
        local deleted_pods=()
        
        while IFS='|' read -r pod_ns pod_name pod_phase pod_status; do
            [[ -z "$pod_ns" ]] && continue
            ((pod_count++))
            printf "%-30s %-35s %-15s %-20s\n" "$pod_ns" "$pod_name" "$pod_phase" "$pod_status"
            deleted_pods+=("$pod_ns|$pod_name")
        done <<< "$unhealthy_pods"
        
        echo "--------------------------------------------------------------------------------"
        echo ""
        
        # Now delete the pods
        for pod_entry in "${deleted_pods[@]}"; do
            IFS='|' read -r pod_ns pod_name <<< "$pod_entry"
            echo "$(yellow)[CLEANUP] Deleting unhealthy pod $pod_name in namespace $pod_ns...$(reset)"
            
            # Remove finalizers first to prevent hanging
            kubectl patch pod "$pod_name" -n "$pod_ns" --type=merge -p='{"metadata":{"finalizers":[]}}' 2>/dev/null || true
            
            # Delete the pod
            kubectl delete pod "$pod_name" -n "$pod_ns" --ignore-not-found=true --grace-period=0 --force 2>/dev/null || true
        done
        
        echo ""
        echo "$(green)[OK] Deleted $pod_count unhealthy pod(s)$(reset)"
        console_success "[✓] Deleted $pod_count unhealthy pod(s)"
        
        # Wait a moment for pods to be recreated
        echo "[INFO] Waiting 10 seconds for pods to be recreated..."
        sleep 10
        
        return 0
    else
        echo "$(green)[OK] No unhealthy pods found to delete$(reset)"
        console_success "[✓] All pods are healthy"
        return 1
    fi
}

# ============================================================
# Post-upgrade cleanup function
# ============================================================
post_upgrade_cleanup() {
    print_header "Post-upgrade Cleanup (Manual Fixes)"
    console_info "[→] Running post-upgrade cleanup..."

    echo "[INFO] Deleting applications tenancy-api-mapping and tenancy-datamodel in namespace onprem..."
    delete_app "tenancy-api-mapping" "onprem"
    delete_app "tenancy-datamodel" "onprem"

    echo "[INFO] Deleting deployment os-resource-manager in namespace orch-infra..."
    kubectl delete deployment -n orch-infra os-resource-manager || true
    kubectl delete pod -n orch-infra -l app.kubernetes.io/name=dkam 2>/dev/null 
    check_and_download_dkam_certs
    #echo "[INFO] Post-upgrade cleanup completed."
    console_success "[✓] Post-upgrade cleanup completed"
}

# ============================================================
# Main sync function with retry logic
# ============================================================
execute_full_sync() {
    sync_until_green_ns_exclude_root
    print_header "Syncing root-app after all other apps are green"
    sync_root_app_only
    post_upgrade_cleanup
    sync_root_app_only
    check_and_download_dkam_certs
    #list_unhealthy_pods
    #delete_unhealthy_pods
}

# ============================================================
# Check if sync was successful
# ============================================================
check_sync_success() {
    # Check root-app status
    status=$(kubectl get applications.argoproj.io "root-app" -n "$NS" -o json 2>/dev/null)
    if [[ -z "$status" ]]; then
        echo "$(red)[FAIL] root-app not found in namespace '$NS'.$(reset)"
        return 1
    fi
    health=$(echo "$status" | jq -r '.status.health.status')
    sync=$(echo "$status" | jq -r '.status.sync.status')

    if [[ "$health" != "Healthy" || "$sync" != "Synced" ]]; then
        echo "$(red)[FAIL] root-app is NOT Healthy+Synced (Health: $health, Sync: $sync)$(reset)"
        return 1
    fi

    # Check for any non-healthy apps
    not_healthy=$(kubectl get applications.argoproj.io -n "$NS" -o json | jq -r '
        .items[] |
        select(.status.health.status != "Healthy" or .status.sync.status != "Synced") |
        .metadata.name
    ' | wc -l)

    if [[ $not_healthy -gt 0 ]]; then
        echo "$(red)[FAIL] $not_healthy applications are not Healthy+Synced$(reset)"
        console_error "[✗] Sync Failed: $not_healthy applications are not Healthy+Synced"
        return 1
    fi
    kubectl get applications -A
    echo "$(green)[OK] All applications are Healthy+Synced$(reset)"
    console_success "[✓] All applications are Healthy+Synced"
    
    # Display all applications status
    echo
    echo "$(bold)$(green)Final Application Status:$(reset)"
    
    
    return 0
}

# ============================================================
# GLOBAL TIMEOUT WATCHDOG
# ============================================================
#SCRIPT_START_TS=$(date +%s)

# Global retry loop
global_retry=1
#sync_success=false

while (( global_retry <= GLOBAL_SYNC_RETRIES )); do
    print_header "GLOBAL SYNC ATTEMPT ${global_retry}/${GLOBAL_SYNC_RETRIES}"
    console_info "[→] Global sync attempt ${global_retry}/${GLOBAL_SYNC_RETRIES}"

    execute_full_sync

    if check_sync_success; then
        #sync_success=true
        print_header "Sync Script Completed Successfully"
        console_success "[✓] ===== SYNC COMPLETED SUCCESSFULLY ====="
        exit 0
    fi

    if (( global_retry < GLOBAL_SYNC_RETRIES )); then
        echo "$(yellow)[RETRY] Sync attempt ${global_retry} failed. Checking for stuck resources...$(reset)"
        console_warn "[!] Sync attempt ${global_retry} failed - checking for stuck resources"

        # Check and cleanup stuck resources before next retry
        check_and_delete_stuck_crd_jobs

        # Stop all ongoing sync operations
        echo "[INFO] Stopping all ongoing sync operations..."
        mapfile -t all_apps < <(kubectl get applications.argoproj.io -n "$NS" -o jsonpath='{.items[*].metadata.name}')
        for app in "${all_apps[@]}"; do
            [[ -z "$app" ]] && continue
            argocd app terminate-op "${NS}/${app}" --grpc-web 2>/dev/null || true
        done

        echo "$(yellow)[INFO] Waiting 30 seconds before retry ${global_retry}...$(reset)"
        sleep 30

        ((global_retry++))
    else
        echo "$(red)[FAIL] Maximum global retries (${GLOBAL_SYNC_RETRIES}) reached. Sync failed.$(reset)"
        console_error "[✗] ===== SYNC FAILED ===== Maximum retries (${GLOBAL_SYNC_RETRIES}) reached"
        exit 1
    fi
done

# This should not be reached, but just in case
echo "$(red)[FAIL] Sync did not complete successfully after ${GLOBAL_SYNC_RETRIES} attempts.$(reset)"
console_error "[✗] ===== SYNC FAILED ===== Did not complete after ${GLOBAL_SYNC_RETRIES} attempts"
exit 1
