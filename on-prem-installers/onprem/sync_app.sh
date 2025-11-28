#!/bin/bash

set -e


NS="onprem"

# -----------------------------
# Check & Install argoCD CLI
# -----------------------------
if ! command -v argocd >/dev/null 2>&1; then
    echo "[INFO] argocd CLI not found. Installing..."

    VERSION=$(curl -L -s https://raw.githubusercontent.com/argoproj/argo-cd/stable/VERSION)
    echo "[INFO] Latest version: $VERSION"

    curl -sSL -o argocd-linux-amd64 \
        https://github.com/argoproj/argo-cd/releases/download/v${VERSION}/argocd-linux-amd64

    sudo install -m 555 argocd-linux-amd64 /usr/local/bin/argocd
    rm -f argocd-linux-amd64

    echo "[INFO] argocd CLI installed successfully."
else
    echo "[INFO] argocd CLI already installed: $(argocd version --client | head -1)"
fi

# -----------------------------
# ADMIN PASSWORD
# -----------------------------
echo "[INFO] Fetching ArgoCD admin password..."

if command -v yq >/dev/null 2>&1; then
    ADMIN_PASSWD=$(kubectl get secret -n argocd argocd-initial-admin-secret -o yaml | yq '.data.password' | base64 -d)
else
    ADMIN_PASSWD=$(kubectl get secret -n argocd argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d)
fi

# -----------------------------
# ArgoCD Server IP (LB or NodePort)
# -----------------------------
echo "[INFO] Detecting ArgoCD server IP..."

ARGO_IP=$(kubectl get svc argocd-server -n argocd -o jsonpath="{.status.loadBalancer.ingress[0].ip}")

if [[ -z "$ARGO_IP" ]]; then
    # Fallback: NodePort mode
    NODEPORT=$(kubectl get svc argocd-server -n argocd -o jsonpath='{.spec.ports[0].nodePort}')
    NODEIP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[0].address}')
    ARGO_IP="${NODEIP}:${NODEPORT}"
    echo "[INFO] LoadBalancer IP not found, using NodePort: ${ARGO_IP}"
else
    echo "[INFO] LoadBalancer IP: ${ARGO_IP}"
fi

# -----------------------------
# Login
# -----------------------------
echo "[INFO] Logging in to ArgoCD..."
argocd login "${ARGO_IP}" --username admin --password "${ADMIN_PASSWD}" --insecure
echo "[INFO] ArgoCD login successful."




# ------------------------------------------------------------
# Return NOT GREEN apps (health != Healthy OR sync != Synced)
# ------------------------------------------------------------
get_not_green_apps() {
    kubectl get applications.argoproj.io -n "$NS" -o json \
    | jq -r '
        .items[] | {
            name: .metadata.name,
            wave: (.metadata.annotations["argocd.argoproj.io/sync-wave"] // "0"),
            health: .status.health.status,
            sync: .status.sync.status
        }
        | select(.health != "Healthy" or .sync != "Synced")
        | "\(.wave) \(.name) \(.health) \(.sync)"
    '
}

# ------------------------------------------------------------
# Main sync logic: Sync apps not green in wave order
# ------------------------------------------------------------
sync_not_green_apps_once() {

    mapfile -t apps < <(get_not_green_apps | sort -n -k1)

    if [[ ${#apps[@]} -eq 0 ]]; then
        echo "🎉 All apps are GREEN. Nothing to sync."
        return 0
    fi

    echo "---------------------------------------------------------"
    echo "Syncing ${#apps[@]} NOT-GREEN apps..."
    echo "---------------------------------------------------------"

    for entry in "${apps[@]}"; do

        wave=$(echo "$entry" | awk '{print $1}')
        name=$(echo "$entry" | awk '{print $2}')
        health=$(echo "$entry" | awk '{print $3}')
        sync=$(echo "$entry" | awk '{print $4}')

        full_app="${NS}/${name}"

        echo "---------------------------------------------------------"
        echo "App: $full_app"
        echo "Wave: $wave"
        echo "Current Health: $health"
        echo "Current Sync:   $sync"
        echo "Syncing...."
        echo

        argocd app sync "$full_app" --grpc-web

        if [[ $? -ne 0 ]]; then
            echo "❌ Sync FAILED for $full_app"
        else
            echo "✔ Sync OK for $full_app"
        fi

        echo
    done

    return 1
}

# ------------------------------------------------------------
# LOOP UNTIL ALL APPS ARE GREEN
# ------------------------------------------------------------
sync_until_green() {
    echo "========================================================="
    echo "Starting continuous sync loop until ALL apps are GREEN"
    echo "Namespace: $NS"
    echo "========================================================="

    while true; do
        echo
        echo "Checking app statuses..."

        # If all are green → exit
        if [[ -z "$(get_not_green_apps)" ]]; then
            echo
            echo "🎉🎉🎉 ALL APPLICATIONS ARE GREEN (Healthy + Synced) 🎉🎉🎉"
            break
        fi

        # Otherwise sync again
        sync_not_green_apps_once

        echo "Waiting 10 seconds before next check..."
        sleep 10
    done
}

# ------------------------------------------------------------
# MAIN
# ------------------------------------------------------------
sync_until_green


