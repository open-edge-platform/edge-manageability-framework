---
name: pod-security
description: Enforce pod security standards and policies
---

You are implementing pod security. Follow these steps:

1. **Security assessment**:
   - Audit pods for security misconfigurations
   - Check for privileged containers: `kubectl get pods -A -o jsonpath='{.items[?(@.spec.containers[*].securityContext.privileged==true)].metadata.name}'`
   - Identify root-running containers
   - Review capabilities and sysctls

2. **Pod Security Standards**:
   - Implement Pod Security Admission
   - Set namespace labels (enforce/audit/warn)
   - Choose appropriate level (privileged/baseline/restricted)
   - Test policy impact with --dry-run

3. **Security Context configuration**:
   - Set runAsNonRoot: true
   - Configure runAsUser and fsGroup
   - Drop unnecessary capabilities
   - Add readOnlyRootFilesystem where possible
   - Set allowPrivilegeEscalation: false

4. **Policy enforcement with Kyverno**:
   - Review Kyverno policies
   - Create custom policies for organization standards
   - Test policies in audit mode first
   - Monitor policy violations

5. **Remediation**:
   - Prioritize fixes by risk level
   - Update deployments with security contexts
   - Document exceptions and justifications
   - Provide migration guide for teams
   - Set enforcement timelines
