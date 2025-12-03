#!/bin/bash
set -o pipefail

# ============================================================
# ============= GLOBAL CONFIGURATION VARIABLES ===============
# ============================================================

# Namespace settings
NS="onprem"
ARGO_NS="argocd"

# Sync behaviour
GLOBAL_POLL_INTERVAL=10           # seconds
APP_MAX_WAIT=90               # 5 minutes to wait for any app (Healthy+Synced)
APP_MAX_RETRIES=3                 # retry X times for each app

# Root app final wait
ROOT_APP_MAX_WAIT=300             # 5 minutes

# Global script timeout
SCRIPT_MAX_TIMEOUT=1200           # 20 minutes

# Installer behaviour
CURL_TIMEOUT=20

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
    if command -v argocd >/dev/null 2>&1; then
        echo "[INFO] argocd CLI present: $(argocd version --client | head -1)"
        return
    fi

    echo "[INFO] Installing argocd CLI..."
    VERSION=$(curl -sL --max-time "$CURL_TIMEOUT" \
        https://raw.githubusercontent.com/argoproj/argo-cd/stable/VERSION)

    curl -sSL --max-time "$CURL_TIMEOUT" -o argocd-linux-amd64 \
        "https://github.com/argoproj/argo-cd/releases/download/v${VERSION}/argocd-linux-amd64"

    sudo install -m 555 argocd-linux-amd64 /usr/local/bin/argocd
    rm -f argocd-linux-amd64

    echo "[INFO] argocd CLI installed."
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

# ============================================================
# Argo Login
# ============================================================
echo "[INFO] Logging into ArgoCD..."
argocd login "$ARGO_ENDPOINT" --username admin --password "$ADMIN_PASSWD" --insecure --grpc-web
echo "[INFO] Login OK."

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

print_header() {
    echo
    echo "$(bold)$(blue)============================================================$(reset)"
    echo "$(bold)$(blue)== $1$(reset)"
    echo "$(bold)$(blue)============================================================$(reset)"
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
    for line in "${all_apps[@]}"; do
        read -r wave name health sync <<< "$line"
        if [[ "$health" != "Healthy" || "$sync" != "Synced" ]]; then
            echo "$(red)  - $name (wave=$wave) Health=$health Sync=$sync$(reset)"
        fi
    done
    echo

    # Sync NOT-GREEN apps in wave order, skipping root-app until last
    for line in "${all_apps[@]}"; do
        read -r wave name health sync <<< "$line"
        full_app="${NS}/${name}"

        # Skip root-app for now, handle it after all other apps
        if [[ "$name" == "root-app" ]]; then
            continue
        fi

        attempt=1
        synced=false
        while (( attempt <= APP_MAX_RETRIES )); do
            status=$(kubectl get applications.argoproj.io "$name" -n "$NS" -o json 2>/dev/null)
            if [[ -n "$status" ]]; then
                health=$(echo "$status" | jq -r '.status.health.status')
                sync=$(echo "$status" | jq -r '.status.sync.status')
                if [[ "$health" == "Healthy" && "$sync" == "Synced" ]]; then
                    echo "$(green)[OK] $full_app (wave=$wave) already Healthy+Synced$(reset)"
                    synced=true
                    break
                fi
            fi

            echo "$(bold)[SYNC] $full_app (wave=$wave)$(reset)"
            echo "$(yellow)[INFO] Attempt ${attempt}/${APP_MAX_RETRIES}$(reset)"

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

            timed_out=false
            while true; do
                now_ts=$(date +%s)
                elapsed=$(( now_ts - start_ts ))
                if (( elapsed >= APP_MAX_WAIT )); then
                    echo "$(red)[TIMEOUT] $full_app did not become Healthy+Synced within ${APP_MAX_WAIT}s.$(reset)"
                    timed_out=true
                    break
                fi
                status=$(kubectl get applications.argoproj.io "$name" -n "$NS" -o json 2>/dev/null)
                [[ -z "$status" ]] && { sleep "$GLOBAL_POLL_INTERVAL"; continue; }
                health=$(echo "$status" | jq -r '.status.health.status')
                sync=$(echo "$status" | jq -r '.status.sync.status')
                print_table_row "$wave" "$name" "$health" "$sync"
                echo "    Elapsed: ${elapsed}s"
                if [[ "$health" == "Healthy" && "$sync" == "Synced" ]]; then
                    echo "$(green)[DONE] $full_app Healthy+Synced in ${elapsed}s (attempt ${attempt})$(reset)"
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
                echo "$(red)[FAIL] Max retries reached for $full_app. Proceeding to next app.$(reset)"
            fi
        done
        echo "$(blue)[INFO] Proceeding to next app...$(reset)"
    done

    # Now handle root-app sync after all other apps
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
        if [[ "$health" == "Healthy" && "$sync" == "Synced" ]]; then
            echo "$(green)[OK] $full_app (wave=$wave) already Healthy+Synced$(reset)"
            synced=true
            break
        fi

        echo "$(bold)[SYNC] $full_app (wave=$wave)$(reset)"
        echo "$(yellow)[INFO] Attempt ${attempt}/${APP_MAX_RETRIES}$(reset)"

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

        timed_out=false
        while true; do
            now_ts=$(date +%s)
            elapsed=$(( now_ts - start_ts ))
            if (( elapsed >= APP_MAX_WAIT )); then
                echo "$(red)[TIMEOUT] $full_app did not become Healthy+Synced within ${APP_MAX_WAIT}s.$(reset)"
                timed_out=true
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

        attempt=1
        synced=false
        while (( attempt <= APP_MAX_RETRIES )); do
            status=$(kubectl get applications.argoproj.io "$name" -n "$NS" -o json 2>/dev/null)
            if [[ -n "$status" ]]; then
                health=$(echo "$status" | jq -r '.status.health.status')
                sync=$(echo "$status" | jq -r '.status.sync.status')
                if [[ "$health" == "Healthy" && "$sync" == "Synced" ]]; then
                    echo "$(green)[OK] $full_app (wave=$wave) already Healthy+Synced$(reset)"
                    synced=true
                    break
                fi
            fi

            echo "$(bold)[SYNC] $full_app (wave=$wave)$(reset)"
            echo "$(yellow)[INFO] Attempt ${attempt}/${APP_MAX_RETRIES}$(reset)"

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

            timed_out=false
            while true; do
                now_ts=$(date +%s)
                elapsed=$(( now_ts - start_ts ))
                if (( elapsed >= APP_MAX_WAIT )); then
                    echo "$(red)[TIMEOUT] $full_app did not become Healthy+Synced within ${APP_MAX_WAIT}s.$(reset)"
                    timed_out=true
                    break
                fi
                status=$(kubectl get applications.argoproj.io "$name" -n "$NS" -o json 2>/dev/null)
                [[ -z "$status" ]] && { sleep "$GLOBAL_POLL_INTERVAL"; continue; }
                health=$(echo "$status" | jq -r '.status.health.status')
                sync=$(echo "$status" | jq -r '.status.sync.status')
                print_table_row "$wave" "$name" "$health" "$sync"
                echo "    Elapsed: ${elapsed}s"
                if [[ "$health" == "Healthy" && "$sync" == "Synced" ]]; then
                    echo "$(green)[DONE] $full_app Healthy+Synced in ${elapsed}s (attempt ${attempt})$(reset)"
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
                echo "$(red)[FAIL] Max retries reached for $full_app. Proceeding to next app.$(reset)"
            fi
        done
        echo "$(blue)[INFO] Proceeding to next app...$(reset)"
    done
}

# ============================================================
# Sync root-app only (with nice reporting)
# ============================================================
sync_root_app_only() {
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

    if [[ "$health" == "Healthy" && "$sync" == "Synced" ]]; then
        echo "$(green)[OK] $full_app (wave=$wave) already Healthy+Synced$(reset)"
        return 0
    fi

    echo "$(bold)[SYNC] $full_app (wave=$wave)$(reset)"
    attempt=1
    synced=false
    while (( attempt <= APP_MAX_RETRIES )); do
        echo "$(yellow)[INFO] Attempt ${attempt}/${APP_MAX_RETRIES}$(reset)"
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

        timed_out=false
        while true; do
            now_ts=$(date +%s)
            elapsed=$(( now_ts - start_ts ))
            if (( elapsed >= APP_MAX_WAIT )); then
                echo "$(red)[TIMEOUT] $full_app did not become Healthy+Synced within ${APP_MAX_WAIT}s.$(reset)"
                timed_out=true
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
    while true; do
        if ! namespace_all_green_exclude_root; then
            print_header "All non-root-app applications are Healthy+Synced in namespace '$NS'."
            break
        fi

        print_header "NOT-GREEN apps (Wave-Ordered, excluding root-app)"
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

        sleep "10"
    done
}


# ============================================================
# Post-upgrade cleanup function
# ============================================================
post_upgrade_cleanup() {
    print_header "Post-upgrade Cleanup (Manual Fixes)"

    echo "[INFO] Deleting applications tenancy-api-mapping and tenancy-datamodel in namespace onprem..."
    kubectl delete application tenancy-api-mapping -n onprem || true
    kubectl delete application tenancy-datamodel -n onprem || true

    echo "[INFO] Deleting deployment os-resource-manager in namespace orch-infra..."
    kubectl delete deployment -n orch-infra os-resource-manager || true

    echo "[INFO] Deleting onboarding secrets..."
    kubectl delete secret tls-boots -n orch-boots || true
    kubectl delete secret boots-ca-cert -n orch-gateway || true
    kubectl delete secret boots-ca-cert -n orch-infra || true

    echo "[INFO] Deleting dkam pods in namespace orch-infra..."
    kubectl delete pod -n orch-infra -l app.kubernetes.io/name=dkam 2>/dev/null || true

    echo "[INFO] Post-upgrade cleanup completed."
}

# ============================================================
# GLOBAL TIMEOUT WATCHDOG
# ============================================================
SCRIPT_START_TS=$(date +%s)

sync_until_green_ns_exclude_root
print_header "Syncing root-app after all other apps are green"
sync_root_app_only

post_upgrade_cleanup

sleep 60
print_header "Post-upgrade: Syncing all apps (excluding root-app) again"
sync_all_apps_exclude_root
print_header "Post-upgrade: Syncing root-app again"
sync_root_app_only

# Check root-app status after post-upgrade sync, exit 1 if not Healthy+Synced
status=$(kubectl get applications.argoproj.io "root-app" -n "$NS" -o json 2>/dev/null)
if [[ -z "$status" ]]; then
    echo "$(red)[FAIL] root-app not found in namespace '$NS' after post-upgrade.$(reset)"
    exit 1
fi
health=$(echo "$status" | jq -r '.status.health.status')
sync=$(echo "$status" | jq -r '.status.sync.status')
if [[ "$health" != "Healthy" || "$sync" != "Synced" ]]; then
    echo "$(red)[FAIL] root-app is NOT Healthy+Synced after post-upgrade.$(reset)"
    exit 1
fi

print_header "Sync Script Completed"
exit 0
