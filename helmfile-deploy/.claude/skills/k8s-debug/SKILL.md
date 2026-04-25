---
name: k8s-debug
description: Debug Kubernetes pods and services systematically
---

You are debugging a Kubernetes issue. Follow this systematic approach:

1. **Identify the problem scope**:
   - Get pod status: `kubectl get pods -A`
   - Check events: `kubectl get events --sort-by='.lastTimestamp'`
   - Review recent changes in git history

2. **Investigate pod issues**:
   - Check pod logs: `kubectl logs <pod> --previous` (if crashed)
   - Describe pod: `kubectl describe pod <pod>`
   - Look for: ImagePullBackOff, CrashLoopBackOff, Pending states
   - Check resource constraints (CPU/memory limits)

3. **Check networking**:
   - Verify service endpoints: `kubectl get endpoints`
   - Test DNS resolution: `kubectl run -it --rm debug --image=busybox -- nslookup <service>`
   - Check network policies and Istio configurations

4. **Review configurations**:
   - Verify ConfigMaps and Secrets exist
   - Check RBAC permissions
   - Validate environment variables

5. **Provide diagnosis**:
   - Summarize root cause
   - Suggest fix with specific commands
   - Include preventive measures
