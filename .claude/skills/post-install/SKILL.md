---
name: post-install
description: Install post-orchestrator EMF application stack
---

# Post-Install

Install post-orchestrator EMF application stack.

## What this does
- Deploys all EMF application components
- Installs network layer (Traefik, HAProxy, Istio)
- Sets up security (Vault, Keycloak, Cert Manager)
- Deploys databases (PostgreSQL, TimescaleDB)
- Configures observability (Prometheus, Grafana, Loki)
- Installs edge services (EIM, vPro management)

## Usage
Use this skill when you want to:
- Deploy complete EMF stack
- Install specific components
- Update existing deployment
- Roll out new versions

---

Run post-installation by:
1. **Verify prerequisites**:
   - Pre-orch installed successfully
   - OpenEBS storage class available
   - MetalLB working
   - Cluster has sufficient resources
2. **Review post-orch.env** configuration (use /post-env-setting)
3. **Validate configuration**:
   - DEPLOYMENT_PROFILE set (onprem-eim, onprem-vpro, aws)
   - Feature flags match requirements
   - IP addresses configured
   - Version tags set
4. **Preview deployment**:
   ```bash
   cd post-orch/
   source post-orch.env
   helmfile -e onprem-eim list
   helmfile -e onprem-eim diff
   ```
5. **Run installation**:
   ```bash
   ./post-orch-deploy.sh install
   # OR for specific component:
   helmfile -e onprem-eim -l app=traefik sync
   ```
6. **Monitor deployment**:
   - Watch pod status
   - Check for errors
   - Verify services are ready
7. **Verify installation**:
   - All pods running
   - Services accessible
   - Ingress configured
8. Report deployment status and access URLs

**Deployment order matters**:
1. Cert Manager (certificates)
2. Vault (secrets)
3. External Secrets
4. Network components
5. Databases
6. Applications
