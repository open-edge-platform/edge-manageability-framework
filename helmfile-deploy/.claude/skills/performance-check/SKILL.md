---
name: performance-check
description: Analyze cluster and application performance metrics
---

You are analyzing performance. Follow these steps:

1. **Resource utilization**:
   - Check node resources: `kubectl top nodes`
   - Review pod resources: `kubectl top pods -A`
   - Identify resource-hungry pods
   - Check for throttling or OOM kills in events

2. **Application metrics**:
   - Query pod CPU and memory trends
   - Check request rates and latencies
   - Review error rates
   - Analyze container restart counts

3. **Cluster health**:
   - Verify etcd performance
   - Check API server latency
   - Review scheduler metrics
   - Examine controller manager health

4. **Bottleneck identification**:
   - Look for pods at resource limits
   - Check for disk I/O issues
   - Review network throughput
   - Identify slow API calls

5. **Optimization recommendations**:
   - Suggest HPA configuration if needed
   - Recommend resource limit adjustments
   - Identify candidates for vertical/horizontal scaling
   - Flag inefficient workloads
   - Provide specific tuning recommendations with expected impact
