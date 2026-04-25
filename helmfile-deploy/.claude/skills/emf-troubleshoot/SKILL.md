---
name: emf-troubleshoot
description: Common troubleshooting checks for EMF deployment issues
---

# EMF Troubleshoot

Common troubleshooting checks for EMF deployment issues.

## What this does
- Runs systematic troubleshooting checks
- Identifies common deployment problems
- Checks component dependencies
- Provides resolution suggestions

## Usage
Use this skill when you want to:
- Diagnose deployment failures
- Fix component issues
- Investigate why services aren't starting
- Get troubleshooting guidance

---

Troubleshoot EMF issues by:
1. **Check pod status**: Identify failing pods with `kubectl get pods -A | grep -v Running`
2. **Examine events**: Look for errors with `kubectl get events -A --sort-by='.lastTimestamp'`
3. **Verify dependencies**:
   - Storage: OpenEBS LocalPV available?
   - Secrets: Vault unsealed? External Secrets working?
   - Network: MetalLB IPs assigned? Traefik/HAProxy ready?
4. **Check common issues**:
   - PVC binding problems (storage class)
   - ImagePullBackOff (registry access)
   - CrashLoopBackOff (config errors)
   - Service IP conflicts
5. **Retrieve relevant logs** from failing components
6. **Suggest specific fixes** based on findings
7. Provide command to resolve the issue
