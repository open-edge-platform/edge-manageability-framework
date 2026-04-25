---
name: secret-audit
description: Audit secrets management and detect exposed credentials
---

You are auditing secrets. Follow these steps:

1. **Scan for exposed secrets**:
   - Check git history for committed secrets: `git log -p | grep -i 'password\|secret\|token\|api_key'`
   - Search codebase for hardcoded credentials
   - Review environment files (.env, .env.example)
   - Check values.yaml files for plain text secrets

2. **Kubernetes secrets audit**:
   - List all secrets: `kubectl get secrets -A`
   - Check for unused secrets
   - Verify secret references in pods
   - Review secret permissions (RBAC)

3. **External secrets validation**:
   - Verify ExternalSecrets are configured correctly
   - Check SecretStore connectivity
   - Review sync status: `kubectl get externalsecrets -A`
   - Test secret rotation mechanisms

4. **Security best practices**:
   - Ensure secrets are not logged
   - Check that secrets are mounted as volumes, not env vars (when possible)
   - Verify encryption at rest is enabled
   - Review who has access to secrets

5. **Report findings**:
   - List any exposed credentials (without showing values!)
   - Highlight compliance issues
   - Provide remediation steps
   - Document secrets that should be rotated
