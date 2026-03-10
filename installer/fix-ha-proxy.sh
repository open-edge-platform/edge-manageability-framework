#!/bin/bash

set -o errexit
set -o pipefail
set -o nounset

APP_NAME="root-app"
ARGOCD_NS="argocd"

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
        STATUS=$(kubectl get application "$APP_NAME" -n "$ARGOCD_NS" \
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

# Wait until pods with a specific label are deleted
wait_for_pod_deletion() {
    local ns=$1
    local label=$2

    echo "[INFO] Waiting for pods with label $label to terminate..."

    while kubectl get pod -n "$ns" -l "$label" --no-headers 2>/dev/null | grep -q .; do
        sleep 5
    done

    echo "[OK] Pods deleted"
}

# Wait for helm upgrade to take effect
echo "[INFO] Waiting 3 minutes for helm upgrade to take effect..."
sleep 180

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

# Remove cluster policies
echo "[INFO] Removing cluster policies..."

kubectl delete clusterpolicy restart-mps-deployment-on-secret-change --ignore-not-found
kubectl delete clusterpolicy restart-rps-deployment-on-secret-change --ignore-not-found

# Wait for root-app to be healthy
wait_for_root_app_healthy

# Stop root-app sync
echo "[INFO] Stopping sync on root-app..."

kubectl patch application "$APP_NAME" -n "$ARGOCD_NS" \
    --type merge \
    -p '{"operation":null}' || true

kubectl patch application "$APP_NAME" -n "$ARGOCD_NS" \
    --type json \
    -p '[{"op": "remove", "path": "/status/operationState"}]' || true

sleep 2

# Delete tls-boot secret
echo "[INFO] Deleting tls-boot secret..."

kubectl delete secret tls-boot --ignore-not-found

sleep 3

# Remove os-resource-manager deployment
echo "[INFO] Removing os-resource-manager deployment..."

kubectl delete deployment os-resource-manager -n orch-infra --ignore-not-found

sleep 3

# Remove DKAM pods
echo "[INFO] Removing DKAM pods..."

kubectl delete pod -n orch-infra -l app.kubernetes.io/name=dkam --ignore-not-found

wait_for_pod_deletion orch-infra "app.kubernetes.io/name=dkam"

sleep 5

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

# wait for root app to be healthy
wait_for_root_app_healthy

# Download DKAM certificates
check_and_download_dkam_certs