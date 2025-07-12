# EMF On-Prem Upgrade Guide

**Upgrade Path:** EMF On-Prem v3.0 → v3.1  
**Document Version:** 1.0

## Overview

This document provides step-by-step instructions to upgrade your On-Prem Edge Manageability Framework (EMF) from version 3.0 to 3.1.

### Important Notes

> **⚠️ DISRUPTIVE UPGRADE WARNING**  
> This upgrade requires edge node re-onboarding due to architecture changes (RKE2 → K3s).  
> Plan for service downtime and manual data backup/restore procedures.

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
cp edge-manageability-framework/on-prem-installers/onprem/onprem_upgrade.sh .
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

Note: if any dokcer limit hit issue user should set docker login credential as env 

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

```
Ready to proceed with installation? (yes/no)
```

**⚠️ DO NOT enter "yes" yet - proceed to Step 5 first**

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
   
   # Verity LB IP configuration are updated
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
1. OS Configuration upgrade
3. Gitea upgrade
4. ArgoCD upgrade
5. Edge Orchestrator upgrade
5. Unseal Vault


## Post-Upgrade Verification

### System Health Check
```bash
# Verify package versions
dpkg -l | grep onprem-

# Check cluster status
kubectl get nodes
kubectl get pods --all-namespaces

# Verify ArgoCD applications
kubectl get applications -n onprem
```

### Service Validation
- Watch ArgoCD applications until they are in 'Healthy' state
- Verify Gitea repository access
- Test PostgreSQL connectivity

### Web UI Access Verification
After successful EMF upgrade, verify you can access the web UI with the same project/user/credentials.

## Edge Node Re-onboarding

After successful EMF upgrade and web UI access verification:

1. **Re-onboard edge nodes:** Follow [onboarding guide](https://docs.openedgeplatform.intel.com/edge-manage-docs/dev/user_guide/set_up_edge_infra/edge_node_onboard.html)
2. **Create edge clusters:** Follow [cluster creation guide](https://docs.openedgeplatform.intel.com/edge-manage-docs/dev/user_guide/set_up_edge_infra/create_clusters.html)
3. **Restore applications:** Redeploy applications and restore backed-up data

## Troubleshooting

### Common Issues

