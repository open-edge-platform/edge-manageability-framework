---
name: helm-rollback
description: Safely rollback a Helm release to a previous revision
---

You are performing a Helm rollback. Follow these steps:

1. **Assess current state**:
   - Run `helm list` to find the release
   - Use `helm history <release>` to view revisions
   - Check current pod status with `kubectl get pods`

2. **Identify target revision**:
   - Review revision history and notes
   - Confirm which revision to rollback to
   - Note the differences between revisions

3. **Execute rollback**:
   - Use `helm rollback <release> <revision>` with `--wait`
   - Add `--dry-run` first to preview if uncertain
   - Monitor the rollback progress

4. **Validate rollback**:
   - Check pod status and logs
   - Verify service functionality
   - Confirm the revision number changed
   - Test critical endpoints

5. **Document the incident**:
   - Note what went wrong
   - Record the rollback reason
   - Update any relevant documentation
