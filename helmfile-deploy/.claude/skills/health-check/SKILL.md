---
name: health-check
description: Comprehensive cluster and application health check
---

You are performing a health check. Follow these steps:

1. **Cluster components**:
   - Check control plane: `kubectl get componentstatuses` (deprecated but useful)
   - Verify node status: `kubectl get nodes -o wide`
   - Check system pods: `kubectl get pods -n kube-system`
   - Review cluster events for issues

2. **Application health**:
   - Check all pod statuses: `kubectl get pods -A`
   - Review pod restarts and ages
   - Verify readiness and liveness probes
   - Check service endpoints are populated

3. **Resource health**:
   - Review node resource utilization
   - Check for pods in pending state
   - Identify resource pressure (disk, memory, PIDs)
   - Review storage health

4. **Network health**:
   - Test DNS resolution within cluster
   - Verify service-to-service connectivity
   - Check ingress/gateway functionality
   - Test external connectivity

5. **Report summary**:
   - Overall health score
   - Critical issues requiring immediate attention
   - Warnings and recommendations
   - Trending issues
   - Next steps and follow-up actions
