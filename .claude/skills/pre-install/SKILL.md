---
name: pre-install
description: Install pre-orchestrator infrastructure (OpenEBS, MetalLB, cluster setup)
---

# Pre-Install

Install pre-orchestrator infrastructure components.

## What this does
- Sets up Kubernetes cluster (KIND/K3s/RKE2)
- Installs OpenEBS LocalPV storage provisioner
- Configures MetalLB load balancer
- Creates required namespaces and secrets
- Prepares cluster for EMF deployment

## Usage
Use this skill when you want to:
- Set up a new cluster for EMF
- Install infrastructure components
- Prepare cluster before deploying EMF applications

---

Run pre-installation by:
1. **Check prerequisites**:
   - Kubernetes cluster available (KIND/K3s/RKE2)
   - kubectl configured and accessible
   - helmfile and helm installed
   - Required ports available
2. **Review pre-orch.env** configuration (use /pre-env-setting)
3. **Validate configuration**:
   - CLUSTER_PROVIDER set correctly
   - IP addresses configured (MetalLB range, EMF_ORCH_IP)
   - Component flags set appropriately
4. **Run installation**:
   ```bash
   cd pre-orch/
   source pre-orch.env
   ./pre-orch.sh install
   ```
5. **Verify installation**:
   - OpenEBS LocalPV storage class created
   - MetalLB IP pool configured
   - Namespaces created
   - Secrets configured
6. Report readiness for post-orch deployment
