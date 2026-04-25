---
name: emf-logs
description: Quick access to component logs for troubleshooting
---

# EMF Logs

Quick access to component logs for troubleshooting.

## What this does
- Retrieves logs from EMF components
- Can target specific components or pods
- Helps diagnose deployment or runtime issues
- Shows recent log entries

## Usage
Use this skill when you want to:
- Troubleshoot a failing component
- Investigate deployment issues
- Check application logs
- Debug component behavior

---

Retrieve component logs by:
1. If component name provided, find pods with `kubectl get pods -A | grep <component>`
2. Retrieve logs using `kubectl logs` with appropriate namespace
3. Show the last 50 lines by default (adjustable)
4. Include error patterns or key issues in summary
5. Suggest next troubleshooting steps if issues found
