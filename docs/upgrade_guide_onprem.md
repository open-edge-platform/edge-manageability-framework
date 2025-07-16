# EMF On-Prem Upgrade Guide

**Upgrade Path:** EMF On-Prem v3.0 → v3.1  
**Document Version:** 1.0

## Overview

This document provides step-by-step instructions to upgrade
On-Prem Edge Manageability Framework (EMF) from version 3.0 to 3.1.

### Important Notes

> **⚠️ DISRUPTIVE UPGRADE WARNING**  
> This upgrade requires edge node re-onboarding due to architecture changes (RKE2 → K3s).  
> Plan for edge nodes service downtime and
manual data backup/restore procedures in edge nodes.

## Prerequisites

### System Requirements

- Current EMF On-Prem installation version 3.0
- Root/sudo privileges on orchestrator node
- PostgreSQL service running and accessible
- Sufficient disk space for backups ~200+GB
- docker user credential if any pull limit hit

### Pre-Upgrade Checklist

- [ ] Back up critical application data from edge nodes
- [ ] Document current edge node configurations  
- [ ] Remove all edge clusters and hosts:
  - [Delete clusters](https://docs.openedgeplatform.intel.com/edge-manage-docs/dev/user_guide/set_up_edge_infra/clusters/delete_clusters.html)
  - [De-authorize hosts](https://docs.openedgeplatform.intel.com/edge-manage-docs/dev/user_guide/set_up_edge_infra/deauthorize_host.html)
  - [Delete hosts](https://docs.openedgeplatform.intel.com/edge-manage-docs/dev/user_guide/set_up_edge_infra/delete_host.html)

## Upgrade Procedure

### Step 1: Copy Latest OnPrem Upgrade Script

On the orchestrator deployed node, copy the latest upgrade script:

```bash
cd
cp edge-manageability-framework/on-prem-installers/onprem/*.sh ~/
chmod +x onprem_upgrade.sh
```

### Step 2: Open Two Terminals

You will need two terminals for this upgrade process:

- **Terminal 1:** To run the upgrade script
- **Terminal 2:** To update proxy and load balancer configurations when prompted

### Step 3: Terminal 1 - Set Environment Variables

In **Terminal 1**, set the required environment variables:

```bash
# Load environment configuration
source .env

Note: if any docker limit hit issue user should set docker login credential as env 

# Unset PROCEED to allow manual confirmation
unset PROCEED

# Set deployment version (replace with your actual version tag)
export DEPLOY_VERSION="3.1.0-dev-a4bba78"
```

### Step 4: Terminal 1 - Run OnPrem Upgrade Script

In **Terminal 1**, execute the upgrade script:

```bash
./onprem_upgrade.sh
```

The script will:

- Validate current installation
- Check PostgreSQL status
- Download packages and artifacts
- Eventually prompt for confirmation:

```bash
Ready to proceed with installation? (yes/no)
```

- **⚠️ DO NOT enter "yes" yet - proceed to Step 5 first**

### Step 5: Terminal 2 - Update Configuration

Before confirming in Terminal 1, open **Terminal 2** and update configurations:

1. **Update proxy settings (if applicable):**

   ```bash
   cp proxy_config.yaml repo_archives/tmp/edge-manageability-framework/orch-configs/profiles/proxy-none.yaml
   ```

2. **Verify load balancer IP configuration:**

   ```bash
   # Check current LoadBalancer IPs
   kubectl get svc argocd-server -n argocd
   kubectl get svc traefik -n orch-gateway
   kubectl get svc ingress-nginx-controller -n orch-boots
   
   # Verify LB IP configuration are updated
   nano repo_archives/tmp/edge-manageability-framework/orch-configs/clusters/onprem.yaml
   ```

3. **Ensure all configurations are correct**

### Step 6: Terminal 1 - Confirm and Continue

Once proxy and load balancer configurations are updated in Terminal 2, switch back to **Terminal 1** and enter:

```bash
yes
```

The upgrade will then proceed automatically through all components.

### Step 7: Monitor Upgrade Progress

The upgrade process includes:

- OS Configuration upgrade
- Gitea upgrade
- ArgoCD upgrade
- Edge Orchestrator upgrade
- Unseal Vault

## Post-Upgrade Verification

### System Health Check

```bash
# Verify package versions
dpkg -l | grep onprem-

# Check cluster status
kubectl get nodes
kubectl get pods -A

# Verify ArgoCD applications
kubectl get applications -A

### Service Validation
- Watch ArgoCD applications until they are in 'Healthy' state

### Web UI Access Verification
After successful EMF upgrade, verify you can access the web UI with the same project/user/credentials used in before upgrade.

### ArgoCD

- **Username:** `admin`
- **Retrieve argocd password:**
  ```bash
  kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d
  ```

### Gitea

- **Retrieve Gitea username:**

  ```bash
  kubectl get secret gitea-cred -n gitea -o jsonpath="{.data.username}" | base64 -d
  ```
  
- **Reset Gitea password**

  ```bash
  # Get Gitea pod name
  GITEA_POD=$(kubectl get pods -n gitea -l app=gitea -o jsonpath='{.items[0].metadata.name}')
  
  # Reset password (replace 'test12345' with your desired password)
  kubectl exec -n gitea $GITEA_POD -- \
    bash -c 'export GITEAPASSWORD=test12345 && gitea admin user change-password --username gitea_admin --password $GITEAPASSWORD'
  ```

- **Login to Gitea web UI:**

  ```bash
  kubectl -n gitea port-forward svc/gitea-http 3000:443 --address 0.0.0.0
  #Then open [https://localhost:3000](https://localhost:3000) in your browser and use the above credentials.
  ```

## Troubleshooting

**Symptom:**
Sometimes the `infra-managers` application in ArgoCD may show as **Not Healthy** or **Out of Sync**. This can impact dependent components or cluster state.

**Resolution Steps:**

1. **Delete the application from ArgoCD: and resync reoo-app**

During the `onprem_upgrade`, if Vault appears **sealed** or becomes **unavailable**, manual intervention may be required.

**Symptom:**

- **Vault Unseal Problem**
  
  Vault pod status shows sealed, causing issues with secret access or platform services.
  After running the on-prem upgrade script, if you see the following  vault waiting output: then further vault unseal require

  ```bash
  Deleting Vault pod: vault-0 in namespace: orch-platform
  pod "vault-0" deleted
  Waiting for pod 'vault-0' in namespace 'orch-platform' to be in Running state...
  ```

- **Check Vault status**

   ```bash
   kubectl get pod -A | grep vault-0
   kubectl -n orch-platform exec -i vault-0 -- vault status
   ```

- **Vault Unseal Procedure**

   ```bash
   #Run the Vault unseal script
   source ./vault_unseal.sh
   vault_unseal
   ```

## Open Issues:

**API Gateway does not reflect API changes from v1 to v2 automatically**  
*Workaround:* Manually delete the `nexus-api-gw` pod to recover API changes.

**After upgrade, both RKE2 and K3s Cluster Templates are labeled as default**  
*Workaround:* Manually delete all old cluster templates related to 3.0 release RKE2 base.

**Deployment package extensions are not updated after upgrade**  
*Workaround:* Manually delete the `app-orch-tenant-controller` pod.

---

### Automation Script for Workarounds

To simplify post-upgrade recovery, the following script should be executed as part of the upgrade validation steps:

**Script Name:** `after_upgrade_restart.sh`  
**Purpose:** Automates the following workaround actions:
- Restarts the `nexus-api-gw` pod to reflect API changes from v1 to v2
- Deletes outdated RKE2-based cluster templates from the 3.0 release
- Restarts the `app-orch-tenant-controller` pod to trigger deployment extension updates

> ✅ Run the script after the on-prem upgrade using:
> ```bash
> ./after_upgrade_restart.sh
> ```

**EdgeNode local SSH connection error**
**RPS pod Postgres DB query failure**  
**Host filter in the UI is not functioning correctly**  
**Docker rate limit encountered despite using valid credentials**  
