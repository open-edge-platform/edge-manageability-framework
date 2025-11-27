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

# Delete all the crd by running: 
kubectl delete -f https://raw.githubusercontent.com/external-secrets/external-secrets/main/deploy/crds/bundle.yaml

echo "Deleted extern-secrets"
echo "sleep for 60s"
sleep 60
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
kubectl patch -n "$TARGET_ENV" application root-app --patch-file /tmp/argo-cd/sync-patch.yaml --type merge

# argo has trouble replacing this seceret so manually remove it
kubectl delete secret tls-boots -n orch-boots

# force vault to reload
kubectl delete statefulset -n orch-platform vault
