---
name: rbac-audit
description: Audit and optimize Kubernetes RBAC permissions
---

You are auditing RBAC. Follow these steps:

1. **Permission inventory**:
   - List all roles and clusterroles: `kubectl get roles,clusterroles -A`
   - Review rolebindings and clusterrolebindings
   - Identify service accounts and their permissions
   - Map users/groups to permissions

2. **Least privilege analysis**:
   - Find overly permissive roles (wildcard permissions)
   - Identify unused service accounts
   - Check for cluster-admin bindings
   - Review cross-namespace access

3. **Security gaps**:
   - Look for pods without service accounts
   - Check for default service account usage in prod
   - Identify privilege escalation paths
   - Review secret access permissions

4. **Policy recommendations**:
   - Create role templates for common use cases
   - Implement namespace-scoped roles where possible
   - Use RoleBindings over ClusterRoleBindings
   - Document permission requirements per application

5. **Reporting**:
   - Generate permission matrix
   - Highlight security risks with severity levels
   - Provide remediation steps
   - Create example RBAC resources
   - Document approval workflow for permissions
