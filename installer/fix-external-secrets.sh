
#!/bin/bash
# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0


restart_and_wait_pod() {
  local namespace="$1"
  local pattern="$2"

  echo "🔍 Looking for pod matching '$pattern' in namespace '$namespace'..."

  # Find the pod name
  local pod_name
  pod_name=$(kubectl get pods -n "$namespace" | grep "$pattern" | awk '{print $1}')

  if [ -z "$pod_name" ]; then
    echo "❌ No pod found matching pattern '$pattern' in namespace '$namespace'"
    return 1
  fi

  echo "📌 Found pod: $pod_name. Deleting..."
  kubectl delete pod "$pod_name" -n "$namespace"
  kubectl wait deployment/"$pattern" -n "$namespace" --for=condition=Available --timeout=120s

}

# Wait for helm upgrade to take effect
echo "Waiting for 2 minutes for the helm upgrade to take effect..."
sleep 120

# Stop sync on root app
echo "Stopping sync on root app..."
kubectl patch application root-app -n "$TARGET_ENV" --type merge -p '{"operation":null}'
kubectl patch application root-app -n "$TARGET_ENV" --type json -p '[{"op": "remove", "path": "/status/operationState"}]'

# Stop sync on external secrets
echo "Stopping sync on external secrets..."
kubectl patch application external-secrets -n "$TARGET_ENV" --type merge -p '{"operation":null}'
kubectl patch application external-secrets -n "$TARGET_ENV" --type json -p '[{"op": "remove", "path": "/status/operationState"}]'

# Fix external secrets and apply
echo "Deleting and patching external secrets..."
kubectl patch application -n $TARGET_ENV external-secrets  -p '{"metadata": {"finalizers": ["resources-finalizer.argocd.argoproj.io"]}}' --type merge
kubectl delete application -n $TARGET_ENV external-secrets --cascade=background &

kubectl patch crd clustersecretstores.external-secrets.io -p '{"metadata":{"finalizers":[]}}' --type=merge
kubectl delete crd clustersecretstores.external-secrets.io --force &

kubectl patch crd secretstores.external-secrets.io -p '{"metadata":{"finalizers":[]}}' --type=merge
kubectl delete crd secretstores.external-secrets.io --force &

kubectl patch crd externalsecrets.external-secrets.io -p '{"metadata":{"finalizers":[]}}' --type=merge
kubectl delete crd externalsecrets.external-secrets.io --force &
kubectl delete -f https://raw.githubusercontent.com/external-secrets/external-secrets/main/deploy/crds/bundle.yaml &

kubectl delete deployment -n orch-secret external-secrets &
kubectl delete deployment -n orch-secret external-secrets-cert-controller &
kubectl delete deployment -n orch-secret external-secrets-webhook &

kubectl delete service -n orch-secret  external-secrets-webhook &

# Delete the crds again first delete failes sometimes.
kubectl patch crd clustersecretstores.external-secrets.io -p '{"metadata":{"finalizers":[]}}' --type=merge
kubectl delete crd clustersecretstores.external-secrets.io --force &

kubectl patch crd secretstores.external-secrets.io -p '{"metadata":{"finalizers":[]}}' --type=merge
kubectl delete crd secretstores.external-secrets.io --force &

kubectl patch crd externalsecrets.external-secrets.io -p '{"metadata":{"finalizers":[]}}' --type=merge
kubectl delete crd externalsecrets.external-secrets.io --force &

# Delete all the crd by running: 
kubectl delete -f https://raw.githubusercontent.com/external-secrets/external-secrets/main/deploy/crds/bundle.yaml

echo "Deleted external-secrets"
echo "sleep for 100s"
sleep 100
kubectl apply --server-side=true --force-conflicts -f https://raw.githubusercontent.com/external-secrets/external-secrets/refs/tags/v0.20.4/deploy/crds/bundle.yaml || true
# Stop old sync as it will be stuck.
kubectl patch application root-app -n "$TARGET_ENV" --type merge -p '{"operation":null}'
kubectl patch application root-app -n "$TARGET_ENV" --type json -p '[{"op": "remove", "path": "/status/operationState"}]'
sleep 10
# sync root-app
sudo mkdir -p /tmp/argo-cd
cat <<EOF | sudo tee /tmp/argo-cd/sync-patch.yaml >/dev/null
operation:
  sync:
    syncStrategy:
      hook: {}
EOF
echo "Syncing root app"
kubectl patch -n "$TARGET_ENV" application root-app --patch-file /tmp/argo-cd/sync-patch.yaml --type merge

# argo has trouble replacing this seceret so manually remove it
echo "Deleting TLS Boots..."
kubectl delete secret tls-boots -n orch-boots

# force vault to reload
echo "Deleting Vault..."
kubectl delete statefulset -n orch-platform vault

# OS profiles fix
echo "Deleting and Syncing for OS Profiles"
kubectl delete application tenancy-api-mapping -n "$TARGET_ENV"
kubectl delete application tenancy-datamodel -n "$TARGET_ENV"
kubectl delete deployment -n orch-infra os-resource-manager
kubectl patch application tenancy-api-mapping -n "$TARGET_ENV" --patch-file /tmp/argo-cd/sync-patch.yaml --type merge
kubectl patch application tenancy-datamodel -n "$TARGET_ENV" --patch-file /tmp/argo-cd/sync-patch.yaml --type merge 

kubectl patch -n "$TARGET_ENV" application root-app --patch-file /tmp/argo-cd/sync-patch.yaml --type merge

# Cluster Template fix
echo "Deleting and Syncing for Cluster Templates"
restart_and_wait_pod "orch-cluster" "cluster-manager"
restart_and_wait_pod "orch-cluster" "cluster-manager-template-controller"