---
name: emf-cleanup
description: Safe cleanup operations for EMF components
---

# EMF Cleanup

Safe cleanup operations for EMF components.

## What this does
- Safely removes specific EMF components
- Cleans up failed deployments
- Removes test deployments
- Preserves data when appropriate

## Usage
Use this skill when you want to:
- Remove a specific component
- Clean up after testing
- Redeploy a failed component
- Free up cluster resources

---

Perform safe cleanup by:
1. **Confirm what to clean up** - never assume!
2. **Check for dependencies** - warn if other components depend on it
3. **Backup data** - identify PVCs that would be affected
4. **Preview deletion** with helmfile diff/list
5. **Execute removal**:
   - Use `helmfile -e onprem-eim -l app=<component> destroy` for single component
   - Or `kubectl delete` for specific resources
6. **Verify cleanup** - check resources are removed
7. **Report any remaining resources** that need manual cleanup

**Safety checks**:
- Never delete namespaces without explicit confirmation
- Warn about PVC data loss
- Avoid `--force` operations
- Check for dependent services first
