---
name: cost-analysis
description: Analyze Kubernetes cluster costs and optimization opportunities
---

You are analyzing cluster costs. Follow these steps:

1. **Resource inventory**:
   - List all pods with their resource requests/limits
   - Calculate total cluster capacity
   - Identify unused or over-provisioned resources
   - Check for pods without resource limits

2. **Cost breakdown**:
   - Analyze cost by namespace
   - Break down by application/team
   - Identify most expensive workloads
   - Calculate waste from over-provisioning

3. **Optimization opportunities**:
   - Find pods with high request-to-usage ratio
   - Identify candidates for spot/preemptible instances
   - Suggest rightsizing recommendations
   - Review storage costs and cleanup opportunities

4. **Efficiency metrics**:
   - Calculate cluster utilization percentage
   - Measure resource request-to-limit ratios
   - Identify idle resources
   - Check for zombie PVs or unused storage

5. **Recommendations**:
   - Prioritize optimizations by potential savings
   - Suggest HPA/VPA configurations
   - Recommend node pool adjustments
   - Provide cost-benefit analysis
   - Create action plan with expected ROI
