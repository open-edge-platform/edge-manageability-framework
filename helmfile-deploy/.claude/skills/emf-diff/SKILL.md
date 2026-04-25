---
name: emf-diff
description: Preview changes before deploying EMF components
---

# EMF Diff Preview

Preview changes before deploying EMF components.

## What this does
- Shows what would change if you applied the current helmfile configuration
- Compares current state vs desired state
- Helps identify unintended changes
- Allows review before actual deployment

## Usage
Use this skill when you want to:
- Preview changes before deployment
- Verify your configuration changes
- Check what will be updated
- Avoid surprises during deployment

---

Preview pending changes by:
1. Source the post-orch.env file if needed
2. Run `helmfile -e onprem-eim diff` for all components
3. If specific component requested, use `-l app=<component>` selector
4. Summarize the key changes that would be applied
5. Highlight any potentially breaking changes
