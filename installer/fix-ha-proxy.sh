#!/bin/bash

set -o errexit
set -o pipefail
set -o nounset

source .env

APP_NAME="root-app"

check_and_download_dkam_certs() {
    echo "[INFO] Checking DKAM certificates readiness..."
    
    # Remove old certificates if they exist
    rm -rf Full_server.crt signed_ipxe.efi 2>/dev/null || true
    
    local max_attempts=20  # 20 attempts * 30 seconds = 10 minutes
    local attempt=1
    local success=false
    
    while (( attempt <= max_attempts )); do
        echo "[INFO] Checking DKAM certificate availability..."
        
        # Try to download Full_server.crt
        if wget https://tinkerbell-haproxy."$CLUSTER_FQDN"/tink-stack/keys/Full_server.crt --no-check-certificate --no-proxy -q -O Full_server.crt 2>/dev/null; then
            echo "[OK] Full_server.crt downloaded successfully"
            
            # Try to download signed_ipxe.efi using the certificate
            if wget --ca-certificate=Full_server.crt https://tinkerbell-haproxy."$CLUSTER_FQDN"/tink-stack/signed_ipxe.efi -q -O signed_ipxe.efi 2>/dev/null; then
                echo "[OK] signed_ipxe.efi downloaded successfully"
                success=true
                break
            else
                echo "[WARN] Failed to download signed_ipxe.efi, retrying..."
                rm -f Full_server.crt signed_ipxe.efi 2>/dev/null || true
            fi
        else
            echo "[WARN] Full_server.crt not available yet, waiting..."
        fi
        
        if (( attempt < max_attempts )); then
            echo "[INFO] Waiting 30 seconds before next attempt..."
            sleep 30
        fi
        ((attempt++))
    done
    
    if [[ "$success" == "true" ]]; then
        echo "[SUCCESS] DKAM certificates are ready and downloaded"
        return 0
    else
        echo "[FAIL] DKAM certificates not available after 10 minutes"
        return 1
    fi
}


# Wait until an ArgoCD application is deleted
wait_for_app_deletion() {
    local app=$1
    local ns=$2

    echo "[INFO] Waiting for application $app deletion..."

    while kubectl get application "$app" -n "$ns" >/dev/null 2>&1; do
        echo "[INFO] Application $app still exists..."
        sleep 5
    done

    echo "[OK] Application $app deleted"
}

# Wait until root-app becomes Synced and Healthy
wait_for_root_app_healthy() {

    echo "[INFO] Waiting for $APP_NAME to become Synced and Healthy..."

    while true; do
        STATUS=$(kubectl get application "$APP_NAME" -n "$TARGET_ENV" \
            -o jsonpath='{.status.sync.status} {.status.health.status}' 2>/dev/null || echo "Missing")

        echo "[INFO] Current status: $STATUS"

        if [[ "$STATUS" == "Synced Healthy" ]]; then
            echo "[OK] $APP_NAME is Synced and Healthy"
            break
        fi

        if [[ "$STATUS" == *"Degraded"* ]]; then
            echo "[ERROR] Application became Degraded"
            exit 1
        fi

        sleep 10
    done
}


# Wait for helm upgrade to take effect
echo "[INFO] Waiting 2 minutes for helm upgrade to take effect..."
sleep 120

# Remove nginx applications
echo "[INFO] Removing nginx applications..."

kubectl delete application ingress-nginx -n "$TARGET_ENV" --ignore-not-found

# Remove finalizer if nginx-ingress-pxe-boots is stuck
if kubectl get application nginx-ingress-pxe-boots -n "$TARGET_ENV" >/dev/null 2>&1; then
    kubectl patch application nginx-ingress-pxe-boots \
        -n "$TARGET_ENV" \
        --type=json \
        -p='[{"op":"remove","path":"/metadata/finalizers"}]'
fi

kubectl delete application nginx-ingress-pxe-boots -n "$TARGET_ENV" --ignore-not-found

wait_for_app_deletion ingress-nginx "$TARGET_ENV"
wait_for_app_deletion nginx-ingress-pxe-boots "$TARGET_ENV"

# Sync ha-proxy-app
sudo mkdir -p /tmp/argo-cd

cat <<EOF | sudo tee /tmp/argo-cd/sync-patch.yaml >/dev/null
operation:
  sync:
    syncStrategy:
      hook: {}
EOF

kubectl patch -n "$TARGET_ENV" application ingress-haproxy \
    --patch-file /tmp/argo-cd/sync-patch.yaml \
    --type merge

sleep 5


# Stop root-app sync
echo "[INFO] Stopping sync on root-app..."

kubectl patch application "$APP_NAME" -n "$TARGET_ENV" \
    --type merge \
    -p '{"operation":null}' || true

kubectl patch application "$APP_NAME" -n "$TARGET_ENV" \
    --type json \
    -p '[{"op": "remove", "path": "/status/operationState"}]' || true

sleep 2
# Remove cluster policies
echo "[INFO] Removing cluster policies..."

kubectl delete job init-amt-vault-job -n orch-infra --ignore-not-found

kubectl delete clusterpolicy restart-mps-deployment-on-secret-change --ignore-not-found
kubectl delete clusterpolicy restart-rps-deployment-on-secret-change --ignore-not-found

kubectl patch application tenancy-api-mapping -n "$TARGET_ENV" --patch-file /tmp/argo-cd/sync-patch.yaml --type merge
kubectl patch application tenancy-datamodel -n "$TARGET_ENV" --patch-file /tmp/argo-cd/sync-patch.yaml --type merge 


# add sync
# Sync root-app
sudo mkdir -p /tmp/argo-cd

cat <<EOF | sudo tee /tmp/argo-cd/sync-patch.yaml >/dev/null
operation:
  sync:
    syncStrategy:
      hook: {}
EOF

echo "[INFO] Syncing root-app..."

kubectl patch -n "$TARGET_ENV" application "$APP_NAME" \
    --patch-file /tmp/argo-cd/sync-patch.yaml \
    --type merge

# Wait for root-app to be healthy
wait_for_root_app_healthy

# Delete tls-boot secret
echo "[INFO] Deleting tls-boot secret..."

kubectl delete secret tls-boot --ignore-not-found

sleep 20

# Remove os-resource-manager deployment
echo "[INFO] Removing os-resource-manager deployment..."

kubectl delete deployment os-resource-manager -n orch-infra --ignore-not-found

sleep 3

# Remove DKAM pods
echo "[INFO] Removing DKAM pods..."

kubectl delete pod -n orch-infra -l app.kubernetes.io/name=dkam --ignore-not-found

sleep 10


check_and_download_dkam_certs
