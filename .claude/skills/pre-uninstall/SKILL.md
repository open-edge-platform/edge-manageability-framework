---
name: pre-uninstall
description: Uninstall pre-orchestrator infrastructure components
---

# Pre-Uninstall

Uninstall pre-orchestrator infrastructure components.

## What this does
- Removes MetalLB load balancer
- Removes OpenEBS LocalPV storage provisioner
- Cleans up namespaces and configurations
- Optionally destroys cluster (KIND/K3s/RKE2)

## Usage
Use this skill when you want to:
- Remove infrastructure components
- Clean up test deployments
- Reset cluster to clean state
- Completely remove pre-orch setup

---

Run pre-uninstall by:
1. **Check current state**:
   - Check if post-orch is still running (should uninstall first)
   - Identify PVCs that would be lost
   - List services using MetalLB IPs
   - Check which cluster provider is running (KIND/K3s/RKE2)
2. **Show warnings** if issues found:
   - Warn about post-orch components still running
   - List PVCs that will be lost
   - List LoadBalancer services using MetalLB
3. **Ask for confirmation** - MUST confirm before proceeding:
   - Show what will be uninstalled
   - Confirm data loss understanding
   - Get explicit user approval
4. **Run uninstallation** (only after confirmation):
   ```bash
   cd pre-orch/
   ./pre-orch.sh [kind|k3s|rke2] uninstall
   ```
5. **Monitor uninstallation**:
   - Watch script output
   - Note any errors or warnings
6. **Verify cleanup**:
   - Check all pre-orch pods removed
   - Confirm storage class deleted
   - Check namespaces removed (openebs-system, metallb-system)
   - Verify no lingering PVs
7. Report completion status and any remaining resources

**Safety checks**:
- ALWAYS ask for user confirmation before uninstalling
- Warn if post-orch is still running
- Warn about data loss from PVCs
- Never proceed without explicit approval
- Check dependencies first
