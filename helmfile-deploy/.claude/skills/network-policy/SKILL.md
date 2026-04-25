---
name: network-policy
description: Create and validate Kubernetes network policies
---

You are working with network policies. Follow these steps:

1. **Analyze traffic patterns**:
   - Map service-to-service communication
   - Identify ingress and egress requirements
   - Document external dependencies
   - Review current network policies

2. **Design policy strategy**:
   - Start with deny-all baseline
   - Define necessary allow rules
   - Group services by security zones
   - Consider namespace-level policies

3. **Create policies**:
   - Write NetworkPolicy YAML with clear labels
   - Use meaningful names and annotations
   - Define pod selectors precisely
   - Specify protocols and ports explicitly
   - Document the policy purpose

4. **Test policies**:
   - Apply in test environment first
   - Verify allowed traffic works
   - Confirm denied traffic is blocked
   - Test with: `kubectl run test --rm -it --image=busybox -- wget <service>`
   - Check for unintended blocks

5. **Validate and monitor**:
   - Review policy conflicts
   - Monitor logs for denied connections
   - Document policy decisions
   - Provide troubleshooting guide
   - Plan for policy updates
