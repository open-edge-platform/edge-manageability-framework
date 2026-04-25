---
name: helmfile-sync
description: Synchronize Helmfile deployments across environments
---

You are synchronizing Helmfile deployments. Follow these steps:

1. **Pre-sync validation**:
   - Run `helmfile lint` to check syntax
   - Execute `helmfile diff` to preview changes
   - Review the diff output carefully
   - Check for any surprising deletions or modifications

2. **Environment verification**:
   - Confirm correct Kubernetes context: `kubectl config current-context`
   - Verify environment variables are set
   - Check that required secrets exist

3. **Execute sync**:
   - Run `helmfile sync` with appropriate flags
   - Use `--concurrency 1` for safer sequential deploys
   - Monitor progress and capture output
   - Watch for any errors or warnings

4. **Post-sync validation**:
   - Verify all releases are deployed: `helmfile list`
   - Check pod health across all namespaces
   - Run smoke tests on critical services
   - Validate inter-service communication

5. **Rollback plan**:
   - Document the previous state
   - Keep `helmfile diff` output for reference
   - Note rollback commands if needed
