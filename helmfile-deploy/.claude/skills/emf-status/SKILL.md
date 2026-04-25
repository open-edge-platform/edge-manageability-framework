---
name: emf-status
description: Check the status of all Edge Manageability Framework components
---

# EMF Status Check

Check the status of all Edge Manageability Framework components.

## What this does
- Lists all helmfile releases and their status
- Shows pod status across all namespaces
- Identifies any failing components
- Reports on component readiness

## Usage
Use this skill when you want to:
- Get an overview of the EMF deployment
- Check if components are running correctly
- Identify issues after deployment
- Verify deployment health

---

Check the current status of all EMF components by:
1. Running `helmfile -e onprem-eim list` to show all releases
2. Running `kubectl get pods -A` to show pod status
3. Identifying any pods not in Running state
4. Summarizing component health in a concise format
