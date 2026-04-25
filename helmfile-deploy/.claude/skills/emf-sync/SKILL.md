---
name: emf-sync
description: Deploy or update specific EMF components
---

# EMF Sync

Deploy or update specific EMF components.

## What this does
- Deploys specific components using helmfile sync
- Can target single components or groups
- Handles dependencies automatically
- Shows deployment progress

## Usage
Use this skill when you want to:
- Deploy a specific component
- Update a single service
- Apply configuration changes
- Incrementally deploy components

---

Deploy EMF components by:
1. Confirm the component name to deploy
2. Source post-orch.env environment variables
3. Preview changes with `helmfile -e onprem-eim -l app=<component> diff`
4. If approved, execute `helmfile -e onprem-eim -l app=<component> sync`
5. Monitor deployment progress
6. Verify deployment success with pod status check
7. Report any issues or failures

**Important**: Always diff before sync to preview changes.
