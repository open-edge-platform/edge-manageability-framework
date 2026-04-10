#!/bin/bash
# SPDX-FileCopyrightText: 2026 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

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
        echo "[INFO] Checking DKAM certificate availability (attempt $attempt/$max_attempts)..."

        # --timeout=30: without this, wget can hang indefinitely when the corporate
        # proxy keeps the connection open (e.g. during a backend pod restart), which
        # stalls the entire retry loop on a single attempt.
        if wget https://tinkerbell-haproxy."$CLUSTER_FQDN"/tink-stack/keys/Full_server.crt \
                --no-check-certificate --timeout=30 -q -O Full_server.crt 2>/dev/null; then
            echo "[OK] Full_server.crt downloaded successfully"

            # Try to download signed_ipxe.efi using the certificate
            if wget --ca-certificate=Full_server.crt \
                    https://tinkerbell-haproxy."$CLUSTER_FQDN"/tink-stack/signed_ipxe.efi \
                    --timeout=30 -q -O signed_ipxe.efi 2>/dev/null; then
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
        fix_stuck_kyverno_policies
        fix_immutable_jobs

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

sync_root_app_with_prune() {

    echo "[INFO] Syncing root-app with Prune enabled to clean up removed applications..."

    kubectl patch -n "$TARGET_ENV" application "$APP_NAME" --type merge --patch "$(cat <<EOF
{
    "operation": {
        "initiatedBy": {
            "username": "admin"
        },
        "sync": {
            "prune": true,
            "syncStrategy": {
                "hook": {}
            }
        }
    }
}
EOF
)"

    echo "[INFO] Sync triggered. Waiting 30 seconds..."
    sleep 30
}

# to make sure that upgrade process started 
sync_root_app_if_needed() {

    echo "[INFO] Checking sync status for $APP_NAME..."

    SYNC_STATUS=$(kubectl get application "$APP_NAME" -n "$TARGET_ENV" \
        -o jsonpath='{.status.sync.status}' 2>/dev/null || echo "Missing")

    echo "[INFO] Current sync status: $SYNC_STATUS"

    if [[ "$SYNC_STATUS" != "Synced" ]]; then
        echo "[INFO] Application is not Synced. Triggering sync..."

        kubectl patch application "$APP_NAME" -n "$TARGET_ENV" \
            --type merge \
            -p '{"operation":{"sync":{}}}'

        echo "[OK] Sync triggered for $APP_NAME"
    else
        echo "[OK] Application is already Synced"
    fi
}

sync_root_app_if_needed

# Wait for helm upgrade to take effect
echo "[INFO] Waiting 2 minutes for helm upgrade to take effect..."
sleep 120

# Disable root-app self-heal so it doesn't re-create nginx apps while we're deleting them
echo "[INFO] Disabling root-app self-heal during nginx cleanup..."
kubectl patch application "$APP_NAME" -n "$TARGET_ENV" --type merge \
    -p '{"spec":{"syncPolicy":{"automated":{"prune":true,"selfHeal":false}}}}' || true

# Remove nginx applications
echo "[INFO] Removing nginx applications..."

# Remove finalizers first so ArgoCD doesn't block on garbage-collecting nginx resources
if kubectl get application ingress-nginx -n "$TARGET_ENV" >/dev/null 2>&1; then
    kubectl patch application ingress-nginx \
        -n "$TARGET_ENV" \
        --type=json \
        -p='[{"op":"remove","path":"/metadata/finalizers"}]'
fi

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

# Clean up the nginx ValidatingWebhookConfiguration orphaned by app deletion.
# When ArgoCD apps are force-deleted (finalizer removed), the Helm-managed VWC
# is not garbage-collected and remains pointing at the deleted service in orch-boots.
# This blocks haproxy-ingress-pxe-boots from syncing (webhook call fails with
# "service not found"). Delete it unconditionally — haproxy installs its own VWC.
echo "[INFO] Removing stale ingress-nginx admission webhook if present..."
kubectl delete validatingwebhookconfiguration ingress-nginx-admission --ignore-not-found || true

# Also remove any orphaned nginx resources in orch-boots that hold node ports
# (e.g. port 31443) and would conflict with the incoming haproxy deployment.
echo "[INFO] Removing orphaned nginx resources from orch-boots..."
kubectl delete deployment ingress-nginx-controller -n orch-boots --ignore-not-found || true
kubectl delete svc ingress-nginx-controller ingress-nginx-controller-admission -n orch-boots --ignore-not-found || true

# Re-enable root-app self-heal now that nginx apps are gone
echo "[INFO] Re-enabling root-app self-heal..."
kubectl patch application "$APP_NAME" -n "$TARGET_ENV" --type merge \
    -p '{"spec":{"syncPolicy":{"automated":{"prune":true,"selfHeal":true}}}}' || true

# Sync ha-proxy-app
sudo mkdir -p /tmp/argo-cd

cat <<EOF | sudo tee /tmp/argo-cd/sync-patch.yaml >/dev/null
operation:
  sync:
    syncStrategy:
      hook: {}
EOF

# ingress-haproxy is created by the root-app helm upgrade and may not exist yet
echo "[INFO] Waiting for ingress-haproxy application to be created..."
until kubectl get application ingress-haproxy -n "$TARGET_ENV" >/dev/null 2>&1; do
    echo "[INFO] ingress-haproxy not found yet, waiting..."
    sleep 5
done
echo "[OK] ingress-haproxy found"

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
# Clear foregroundDeletion finalizers and delete the cluster policies.
# The Kyverno admission webhook blocks finalizer removal, so scale it down first,
# do all patching and deletion while it is offline, then restore it.
# NOTE: scale-up must come AFTER deletion — not before — to avoid a race where the
# admission controller restarts and re-adds the finalizer before kubectl delete fires.
_kyverno_scale_down() {
    kubectl scale deployment kyverno-admission-controller -n kyverno --replicas=0 2>/dev/null || true
    kubectl wait --for=delete pod -l app.kubernetes.io/component=admission-controller \
        -n kyverno --timeout=60s 2>/dev/null || true
}
_kyverno_scale_up() {
    kubectl scale deployment kyverno-admission-controller -n kyverno --replicas=3 2>/dev/null || true
}
# Clears foregroundDeletion finalizers from any ClusterPolicy that is stuck terminating.
# Safe to call repeatedly — does nothing if no policies are stuck.
fix_stuck_kyverno_policies() {
    local stuck
    stuck=$(kubectl get clusterpolicy -o jsonpath='{range .items[?(@.metadata.deletionTimestamp)]}{.metadata.name}{"\n"}{end}' 2>/dev/null || true)
    [[ -z "$stuck" ]] && return 0
    echo "[INFO] Stuck ClusterPolicies detected: $(echo "$stuck" | tr '\n' ' ') — clearing finalizers..."
    _kyverno_scale_down
    while IFS= read -r policy; do
        [[ -z "$policy" ]] && continue
        kubectl patch clusterpolicy "$policy" -p '{"metadata":{"finalizers":[]}}' --type=merge 2>/dev/null || true
    done <<< "$stuck"
    _kyverno_scale_up
}

# Deletes completed Jobs that ArgoCD cannot patch due to immutable spec.template.
# Reads SyncError conditions across all apps and removes each named Job so ArgoCD
# can recreate it fresh. Safe to call repeatedly — does nothing if no errors exist.
fix_immutable_jobs() {
    local job_names
    # Check both .status.conditions (SyncError type) and .status.operationState.message
    # because apps in Missing state report the error only in operationState, not conditions.
    job_names=$(kubectl get applications -n "$TARGET_ENV" -o json 2>/dev/null \
        | jq -r '
            .items[] |
            (
                (.status.conditions[]? | select(.type == "SyncError") | .message),
                (.status.operationState.message // "")
            )' \
        | grep -oP 'Job\.batch "\K[^"]+' \
        | sort -u || true)
    [[ -z "$job_names" ]] && return 0
    while IFS= read -r job_name; do
        [[ -z "$job_name" ]] && continue
        local ns
        ns=$(kubectl get job -A -o json 2>/dev/null \
            | jq -r --arg n "$job_name" \
                '.items[] | select(.metadata.name == $n) | .metadata.namespace' \
            | head -1)
        if [[ -n "$ns" ]]; then
            echo "[INFO] Deleting immutable completed Job '$job_name' in namespace '$ns'"
            kubectl delete job "$job_name" -n "$ns" --ignore-not-found 2>/dev/null || true
            # Find the owning app and trigger re-sync so ArgoCD recreates the job
            local app_name
            app_name=$(kubectl get applications -n "$TARGET_ENV" -o json 2>/dev/null \
                | jq -r --arg j "$job_name" '
                    .items[] | select(
                        (.status.conditions[]? | select(.type == "SyncError") | .message | contains($j)) or
                        (.status.operationState.message // "" | contains($j))
                    ) | .metadata.name' \
                | head -1)
            if [[ -n "$app_name" ]]; then
                echo "[INFO] Triggering re-sync of application '$app_name' after job deletion"
                kubectl patch application "$app_name" -n "$TARGET_ENV" --type merge \
                    -p '{"operation":{"initiatedBy":{"username":"admin"},"sync":{"revision":"HEAD"}}}' || true
            fi
        fi
    done <<< "$job_names"
}

# Remove cluster policies
echo "[INFO] Removing cluster policies..."

_kyverno_scale_down
kubectl patch clusterpolicy restart-mps-deployment-on-secret-change \
    -p '{"metadata":{"finalizers":[]}}' --type=merge 2>/dev/null || true
kubectl patch clusterpolicy restart-rps-deployment-on-secret-change \
    -p '{"metadata":{"finalizers":[]}}' --type=merge 2>/dev/null || true
kubectl delete clusterpolicy restart-mps-deployment-on-secret-change --ignore-not-found
kubectl delete clusterpolicy restart-rps-deployment-on-secret-change --ignore-not-found
_kyverno_scale_up

kubectl delete job tenancy-api-mapping -n orch-iam --ignore-not-found
kubectl delete job tenancy-datamodel -n orch-iam --ignore-not-found

kubectl patch application tenancy-api-mapping -n "$TARGET_ENV" --patch-file /tmp/argo-cd/sync-patch.yaml --type merge
kubectl patch application tenancy-datamodel -n "$TARGET_ENV" --patch-file /tmp/argo-cd/sync-patch.yaml --type merge 

sleep 4

kubectl delete job init-amt-vault-job -n orch-infra --ignore-not-found
kubectl patch application infra-external -n "$TARGET_ENV" --patch-file /tmp/argo-cd/sync-patch.yaml --type merge

# Delete completed Jobs in ns-label that ArgoCD cannot patch due to immutable spec.template.
# These are recreated with generate names on each install, so the old ones must be removed.
for job in $(kubectl get jobs -n ns-label -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || true); do
    echo "[INFO] Deleting completed Job '$job' in ns-label (immutable spec.template)"
    kubectl delete job "$job" -n ns-label --ignore-not-found
done
kubectl patch application namespace-label -n "$TARGET_ENV" --patch-file /tmp/argo-cd/sync-patch.yaml --type merge || true
kubectl patch application wait-istio-job -n "$TARGET_ENV" --patch-file /tmp/argo-cd/sync-patch.yaml --type merge || true

sync_root_app_with_prune

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
