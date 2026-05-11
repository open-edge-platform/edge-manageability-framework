---
name: post-uninstall
description: Uninstall post-orchestrator EMF application components
---

# Post-Uninstall

Uninstall post-orchestrator EMF application components.

## What this does
- Removes specific EMF components
- Cleans up application deployments
- Removes databases and data
- Cleans up secrets and configurations
- Preserves or removes PVCs based on choice

## Usage
Use this skill when you want to:
- Remove specific components
- Clean up failed deployments
- Redeploy components from scratch
- Completely remove EMF stack

---

Run post-uninstall by:
1. **Confirm what to uninstall** - never assume!
2. **Check dependencies**:
   - List components that depend on target
   - Identify services that will be affected
   - Check for active connections
3. **Backup data** (if needed):
   - Identify PVCs with important data
   - Backup databases
   - Export configurations
4. **Preview deletion**:
   ```bash
   cd post-orch/
   source post-orch.env
   helmfile -e onprem-eim -l app=<component> list
   ```
5. **Execute removal**:
   ```bash
   # Single component:
   helmfile -e onprem-eim -l app=<component> destroy
   
   # All components:
   helmfile -e onprem-eim destroy
   ```
6. **Manual cleanup** if needed:
   - Remove stuck finalizers
   - Delete PVCs: `kubectl delete pvc -n <namespace> <pvc-name>`
   - Clean up namespaces if needed
7. **Verify cleanup**:
   - Check resources are removed
   - Confirm no lingering pods/PVCs
   - Check for stuck finalizers
8. Report completion and any remaining resources

**Safety checks**:
- Never delete without explicit confirmation
- Warn about PVC data loss
- Check for dependent services first
- Avoid `--force` operations
- Be careful with namespace deletion

**Uninstall order** (reverse of install):
1. Applications
2. Databases
3. Network components
4. External Secrets
5. Vault
6. Cert Manager
