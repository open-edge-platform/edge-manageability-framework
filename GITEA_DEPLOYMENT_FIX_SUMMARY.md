# Gitea Deployment Issue Resolution - On-Prem and OXM Profiles

## Problem Summary
Gitea pod deployment was failing with "context deadline exceeded" errors in both on-prem and OXM profile deployments in CI. The error appeared during the DEB post-installation script execution:

```
null_resource.exec_installer[0]: Error: INSTALLATION FAILED: context deadline exceeded
null_resource.exec_installer[0] (remote-exec): dpkg: error processing package onprem-gitea-installer (--configure):
null_resource.exec_installer[0] (remote-exec): installed onprem-gitea-installer package post-installation script subprocess returned error exit status 1
```

Installation would timeout after ~20 minutes with no clear indication of what was blocking pod startup.

## Root Cause Analysis

The deployment failure had **three compounding timeout issues**:

### Issue 1: Terraform Remote-Exec Provisioner Default Timeout (5 minutes)
- **Location**: `terraform/orchestrator/main.tf` - `null_resource.exec_installer`
- **Problem**: The remote-exec provisioner calls the on-prem installer script, but had no explicit timeout set
- **Default Behavior**: Terraform defaults to 5 minutes for remote-exec provisioners
- **Impact**: Installer script (including DEB post-install scripts) would be terminated prematurely, even though the script was still running

### Issue 2: Gitea Helm Installation Timeout (15 minutes)
- **Location**: 
  - `on-prem-installers/cmd/onprem-gitea/after-install.sh`
  - `on-prem-installers/cmd/onprem-gitea/after-upgrade.sh`
- **Problem**: Helm install command waits for pod to be ready (--wait flag) with only 15 minutes timeout
- **Trigger**: Storage class provisioning or other pod startup dependencies were taking >15 minutes
- **Impact**: Helm would timeout, causing dpkg to fail with error exit status 1

### Issue 3: Lack of Diagnostic Information
- **Location**: CI deployment action and installer scripts
- **Problem**: When failures occurred, no detailed logs were captured about:
  - Gitea pod status
  - Storage class and PVC provisioning state
  - ArgoCD Application status
  - Event logs
- **Impact**: Difficult to diagnose root cause of timeout or pod startup failures

## Solutions Implemented

### 1. Increased Terraform Remote-Exec Provisioner Timeout
**File**: `terraform/orchestrator/main.tf`
```terraform
provisioner "remote-exec" {
  inline = [
    "set -o errexit",
    "bash -c 'cd /home/ubuntu; source onprem.env; ./onprem_installer.sh ...'",
  ]
  when = create
  # Increased timeout to 45 minutes to accommodate full DEB installation
  timeout = "45m"
}
```
**Rationale**: 
- Full DEB installation sequence includes multiple components
- Gitea Helm install alone needs 25 minutes
- 45 minutes provides buffer for other components (ArgoCD, Harbor, etc.)

### 2. Increased Gitea Helm Installation Timeout
**Files**: 
- `on-prem-installers/cmd/onprem-gitea/after-install.sh`
- `on-prem-installers/cmd/onprem-gitea/after-upgrade.sh`

**Changes**:
- Helm timeout increased from **15m0s to 25m0s**
- Added pre-installation diagnostics to check storage class availability
- Added comprehensive error handling with detailed pod diagnostics if installation fails

**Before**:
```bash
helm install gitea /tmp/gitea/gitea --values /tmp/gitea/values.yaml ... --timeout 15m0s --wait
```

**After**:
```bash
echo "Starting Gitea Helm installation with increased timeout to 25 minutes..."
echo "Checking storage class availability before installation..."
kubectl get storageclass -o wide || true

if ! helm install gitea /tmp/gitea/gitea --values /tmp/gitea/values.yaml ... --timeout 25m0s --wait; then
  echo "ERROR: Gitea Helm installation failed or timed out"
  echo "=== Gitea Pod Status ==="
  kubectl get pods -n gitea -o wide || true
  echo "=== Gitea Pod Describe ==="
  kubectl describe pods -n gitea || true
  echo "=== Recent Gitea Pod Logs ==="
  kubectl logs -n gitea --all-containers=true --tail=100 || true
  echo "=== Gitea Events ==="
  kubectl get events -n gitea || true
  echo "=== Storage Class and PVC Status ==="
  kubectl get storageclass -o wide || true
  kubectl get pvc -n gitea -o wide || true
  kubectl get pv -o wide || true
  exit 1
fi
```

### 3. Enhanced CI Diagnostic Logging
**File**: `.github/actions/deploy_on_prem/action.yaml`

**New Diagnostic Sections Added**:
- **Gitea-specific diagnostics**: 
  - Pod status and descriptions
  - Container logs (500 lines tail)
  - PVC and PV status
  
- **ArgoCD Gitea Application status**:
  - Gitea ArgoCD Application details
  - Gitea ArgoCD AppProject configuration

- **Storage diagnostics**:
  - Storage class inventory
  - OpenEBS system pod status

**All diagnostics are captured and uploaded as CI artifacts** for post-mortem analysis even if deployment fails.

## Files Modified

1. **terraform/orchestrator/main.tf**
   - Added `timeout = "45m"` to remote-exec provisioner
   - Lines: ~392-396

2. **on-prem-installers/cmd/onprem-gitea/after-install.sh**
   - Increased Helm timeout from 15m to 25m
   - Added pre-installation diagnostics
   - Added comprehensive error handling with pod/storage diagnostics
   - Lines: ~126-156

3. **on-prem-installers/cmd/onprem-gitea/after-upgrade.sh**
   - Increased Helm timeout from 15m to 25m
   - Added pre-upgrade diagnostics
   - Added comprehensive error handling with pod/storage diagnostics
   - Lines: ~126-156

4. **.github/actions/deploy_on_prem/action.yaml**
   - Enhanced diagnostic capture with Gitea-specific sections
   - Added ArgoCD Application status capture
   - Added storage class and OpenEBS diagnostics
   - Added artifact uploads for all diagnostic files
   - Lines: ~83-148

## Expected Behavior After Fix

### Success Path
1. Terraform provisioner waits up to 45 minutes for installer to complete
2. DEB installer runs with sufficient time for all components
3. Gitea Helm install now has 25 minutes to:
   - Pull Docker image
   - Provision storage (PVC/PV)
   - Start pod and dependencies (Redis, PostgreSQL)
   - Reach ready state
4. Post-install script creates Gitea accounts and secrets
5. Installer completes successfully

### Failure Path (with better diagnostics)
If any component fails:
1. Helm install error is caught with detailed output
2. Pod status and descriptions are captured
3. Container logs are saved
4. Storage class and PVC status are recorded
5. All diagnostics uploaded to CI artifacts
6. CI job artifacts show exactly what was blocking the pod

## Testing Recommendations

### For Local Testing
```bash
# Test Gitea installer with verbose output
cd /home/seu/workspace/edge-manageability-framework/on-prem-installers
mage build:giteaInstaller

# Monitor Gitea pod startup
kubectl logs -f -n gitea deployment/gitea
kubectl describe pods -n gitea
kubectl get events -n gitea
```

### For CI Testing
1. Trigger the "Deploy On-Prem" CI job
2. Monitor the installation progress
3. Check CI artifacts for diagnostic logs
4. If still timing out, increase timeouts further and analyze artifact logs

## Future Improvements

### Additional Monitoring
- Add metrics collection for pod startup time
- Track storage provisioning latency
- Monitor Helm installation performance

### Timeout Optimization
- Make timeouts configurable via environment variables
- Adjust based on actual measured times from CI runs
- Implement progressive backoff for resource waits

### Helm Installation Improvements
- Consider breaking Gitea Helm install into separate steps
- Add health checks before Gitea account creation
- Implement retry logic for transient failures

## Related Configuration

**Current Gitea Chart Version**: 10.6.0 (from `/mage/Magefile.go`)

**Storage Configuration**: 
- Uses OpenEBS LocalPV provisioner with openebs-hostpath storage class
- Gitea persistence: 1Gi (from `/pod-configs/module/gitea/gitea-values.yaml.tpl`)
- Redis persistence: Uses efs-1000 storage class

**Pod Dependencies**:
- PostgreSQL (handled by Bitnami chart)
- Redis (handled by Bitnami chart)
- TLS certificates (pre-provisioned)
- Storage volumes (must be available before pod start)
