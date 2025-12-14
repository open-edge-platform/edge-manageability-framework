# Troubleshooting Guide - Post-Upgrade Application Recovery

This guide provides steps to manually recover applications that are OutOfSync or not Healthy after an upgrade.

## Prerequisites

- Access to the cluster with `kubectl` CLI installed
- Access to ArgoCD UI or `argocd` CLI
- Admin credentials for ArgoCD
- Access to the `onprem` namespace (or your target namespace)

## Quick Diagnosis

### Check Application Status

```bash
# List all applications and their status
kubectl get applications -n onprem

# Get detailed status of a specific application
kubectl get application <app-name> -n onprem -o yaml
```

### Identify Problem Applications

```bash
# List OutOfSync applications
kubectl get applications -n onprem -o json | jq -r '.items[] | select(.status.sync.status != "Synced") | .metadata.name'

# List Unhealthy applications
kubectl get applications -n onprem -o json | jq -r '.items[] | select(.status.health.status != "Healthy") | .metadata.name'
```

## Recovery Procedures

### 1. Force Delete Stuck Applications

For applications known to have issues (external-secrets, copy-* apps, platform-keycloak, etc.), manually delete them:

```bash
APP_NAME="external-secrets"
NS="onprem"

# Remove finalizers
kubectl patch application "$APP_NAME" -n "$NS" --type=json -p='[{"op":"remove","path":"/metadata/finalizers"}]'

# Force delete
kubectl delete application "$APP_NAME" -n "$NS" --force --grace-period=0
```

### 2. Sync Root-App to Recreate Applications

After deleting stuck applications, sync root-app to recreate them:

**Using ArgoCD UI:**
1. Navigate to ArgoCD UI in your browser
2. Find and click on the `root-app` application
3. If the app shows "Operation in progress", click the three-dot menu → **Terminate**
4. Click the **SYNC** button
5. Select sync options if needed, then click **SYNCHRONIZE**

**Using ArgoCD CLI:**

```bash
# Login to ArgoCD
argocd login <argocd-endpoint> --username admin --password <password> --insecure --grpc-web

# Sync root-app
argocd app sync onprem/root-app --grpc-web
```

**Using kubectl:**

```bash
# If root-app is in progress, stop it first
kubectl patch application root-app -n onprem --type=json -p='[{"op":"remove","path":"/status/operationState"}]'

# Trigger root-app sync
kubectl patch application root-app -n onprem --type=merge -p='{"operation":{"sync":{"revision":"HEAD"}}}'
```

### 3. Clean Up Failed Jobs

Failed jobs can block application sync:

```bash
# List failed jobs
kubectl get jobs --all-namespaces | grep -v "1/1"

# Delete failed jobs in a specific namespace
NAMESPACE="orch-infra"
kubectl delete jobs -n "$NAMESPACE" --field-selector status.successful!=1

# Remove job finalizers if stuck
JOB_NAME="copy-cert-job"
kubectl patch job "$JOB_NAME" -n "$NAMESPACE" --type=json -p='[{"op":"remove","path":"/metadata/finalizers"}]'
kubectl delete job "$JOB_NAME" -n "$NAMESPACE" --force --grace-period=0
```

### 4. Fix OutOfSync Applications

For applications showing OutOfSync status:

**Using ArgoCD UI:**
1. Navigate to the OutOfSync application in ArgoCD UI
2. Click the **REFRESH** button (or hard refresh from the three-dot menu)
3. Click the **SYNC** button
4. Enable sync options:
   - Check **REPLACE** checkbox
   - Check **FORCE** checkbox if needed
5. Click **SYNCHRONIZE**

**Using ArgoCD CLI:**

```bash
APP_NAME="platform-keycloak"

# Hard refresh the application
argocd app get "onprem/$APP_NAME" --hard-refresh --grpc-web

# Sync with force and replace
argocd app sync "onprem/$APP_NAME" --force --replace --grpc-web
```

**Using kubectl:**

```bash
APP_NAME="platform-keycloak"
NS="onprem"

# Trigger sync operation
kubectl patch application "$APP_NAME" -n "$NS" --type=merge -p='{"operation":{"sync":{"revision":"HEAD","syncOptions":["Replace=true"]}}}'
```

### 5. Handle CRD Version Mismatches

If applications fail due to CRD version issues:

**Using ArgoCD UI:**
1. Navigate to the affected application (e.g., `external-secrets`)
2. Click the three-dot menu → **Hard Refresh**
3. Click **SYNC** button
4. Enable sync options:
   - Check **REPLACE** checkbox
   - Check **FORCE** checkbox
   - Check **SERVER-SIDE APPLY** checkbox
5. Click **SYNCHRONIZE**

**Using ArgoCD CLI:**

```bash
# For external-secrets CRD issues
argocd app get onprem/external-secrets --hard-refresh --grpc-web
argocd app sync onprem/external-secrets --force --replace --server-side --grpc-web
```

**Check CRD versions using kubectl:**

```bash
kubectl get crd <crd-name> -o jsonpath='{.spec.versions[*].name}'
```

### 6. Restart Degraded Applications

For applications showing Degraded health:

**Using ArgoCD UI:**
1. Navigate to the Degraded application
2. If operation is in progress, click three-dot menu → **Terminate**
3. Click three-dot menu → **Hard Refresh**
4. Wait for refresh to complete
5. Click **SYNC** button and then **SYNCHRONIZE**

**Using ArgoCD CLI:**

```bash
APP_NAME="infra-external"

# Terminate ongoing operations
argocd app terminate-op "onprem/$APP_NAME" --grpc-web

# Hard refresh
argocd app get "onprem/$APP_NAME" --hard-refresh --grpc-web

# Re-sync
argocd app sync "onprem/$APP_NAME" --grpc-web
```

## Automated Recovery

If you have access to an automated recovery script, it can help streamline the recovery process by:
- Syncing all applications in wave order
- Cleaning up failed jobs automatically
- Handling stuck applications with retries
- Syncing root-app after all apps are healthy

## Common Issues and Solutions

### Issue: Application Stuck in "Progressing" State

**Solution (ArgoCD UI):**
1. Find the stuck application in ArgoCD UI
2. Click three-dot menu → **Terminate**
3. Click **SYNC** button → **SYNCHRONIZE**

**Solution (ArgoCD CLI):**
```bash
APP_NAME="<app-name>"
argocd app terminate-op "onprem/$APP_NAME" --grpc-web
argocd app sync "onprem/$APP_NAME" --grpc-web
```

### Issue: External Secrets Not Syncing

**Solution (ArgoCD UI):**
1. Navigate to `external-secrets` application
2. Click three-dot menu → **Delete**
3. Confirm deletion
4. Navigate to `root-app` application
5. Click **SYNC** → **SYNCHRONIZE**

**Solution (kubectl + ArgoCD CLI):**
```bash
# Delete and recreate external-secrets
kubectl patch application external-secrets -n onprem --type=json -p='[{"op":"remove","path":"/metadata/finalizers"}]'
kubectl delete application external-secrets -n onprem --force --grace-period=0
argocd app sync onprem/root-app --grpc-web
```

### Issue: Copy Certificate Jobs Failing

**Solution:**
```bash
# Delete all copy-ca-cert applications
for app in copy-ca-cert-boots-to-gateway copy-ca-cert-boots-to-infra copy-ca-cert-gateway-to-cattle copy-ca-cert-gateway-to-infra copy-ca-cert-gitea-to-app copy-ca-cert-gitea-to-cluster; do
    kubectl patch application "$app" -n onprem --type=json -p='[{"op":"remove","path":"/metadata/finalizers"}]'
    kubectl delete application "$app" -n onprem --force --grace-period=0
done

# Sync root-app to recreate
argocd app sync onprem/root-app --grpc-web
```

### Issue: Platform Keycloak Stuck

**Solution (ArgoCD UI):**
1. Navigate to `platform-keycloak` application
2. If operation in progress: three-dot menu → **Terminate**
3. Click **SYNC** button
4. Enable options: **FORCE**, **REPLACE**, **SERVER-SIDE APPLY**
5. Click **SYNCHRONIZE**

**Solution (ArgoCD CLI):**
```bash
# Force sync with server-side apply
argocd app terminate-op onprem/platform-keycloak --grpc-web
argocd app sync onprem/platform-keycloak --force --replace --server-side --grpc-web
```

## Verification Steps

After recovery, verify all applications are healthy:

```bash
# Check overall status
kubectl get applications -n onprem

# Count healthy and synced applications
kubectl get applications -n onprem -o json | jq '[.items[] | select(.status.health.status == "Healthy" and .status.sync.status == "Synced")] | length'

# List any remaining issues
kubectl get applications -n onprem -o json | jq -r '.items[] | select(.status.health.status != "Healthy" or .status.sync.status != "Synced") | "\(.metadata.name): Health=\(.status.health.status) Sync=\(.status.sync.status)"'
```

## Emergency Recovery

If all else fails, perform a complete application reset:

```bash
# 1. Delete all applications except root-app
kubectl get applications -n onprem -o name | grep -v root-app | xargs kubectl delete -n onprem

# 2. Wait for deletion to complete
sleep 30

# 3. Sync root-app to recreate everything
argocd app sync onprem/root-app --grpc-web

# 4. Monitor applications until all are healthy
watch kubectl get applications -n onprem
```

## Contact Support

If issues persist after following this guide:
1. Collect logs: `kubectl get applications -n onprem -o yaml > applications-status.yaml`
2. Collect pod logs: `kubectl get pods --all-namespaces > pods-status.txt`
3. Contact support with the collected information

## Additional Resources

- ArgoCD Documentation: https://argo-cd.readthedocs.io/
- Kubernetes Troubleshooting: https://kubernetes.io/docs/tasks/debug/
- Check application logs: `kubectl logs -n <namespace> <pod-name>`
