---
name: emf-pre-install
description: Run pre-orchestrator installation checks and setup
---

# EMF Pre-Install

Run pre-orchestrator installation checks and setup.

## What this does
- Validates pre-requisites for EMF deployment
- Checks cluster readiness
- Verifies configuration files
- Runs pre-orch installation steps

## Usage
Use this skill when you want to:
- Set up a new cluster for EMF
- Verify pre-installation requirements
- Install OpenEBS and MetalLB
- Prepare cluster infrastructure

---

Run pre-installation by:
1. **Check prerequisites**:
   - Kubernetes cluster available (KIND/K3s/RKE2)
   - kubectl configured and accessible
   - helmfile and helm installed
   - Required ports available
2. **Review pre-orch.env** configuration:
   - CLUSTER_PROVIDER set correctly
   - IP addresses configured (MetalLB range, EMF_ORCH_IP)
   - Component flags set appropriately
3. **Validate helmfile** template in pre-orch/
4. **Run pre-orch.sh install** if confirmed
5. **Verify installation**:
   - OpenEBS LocalPV storage class created
   - MetalLB IP pool configured
   - Namespaces created
   - Secrets configured
6. Report readiness for post-orch deployment
