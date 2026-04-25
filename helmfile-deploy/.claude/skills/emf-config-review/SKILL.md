---
name: emf-config-review
description: Review and validate EMF configuration files
---

# EMF Config Review

Review and validate EMF configuration files.

## What this does
- Reviews environment configuration files
- Checks helmfile values for consistency
- Validates IP address configurations
- Identifies potential configuration issues

## Usage
Use this skill when you want to:
- Review configuration before deployment
- Validate environment settings
- Check for configuration conflicts
- Ensure settings are appropriate for your environment

---

Review EMF configuration by:
1. **Check pre-orch.env**:
   - CLUSTER_PROVIDER matches actual cluster
   - IP addresses don't conflict (MetalLB, Orchestrator, Traefik, HAProxy)
   - MAX_PODS_PER_NODE appropriate for cluster size
   - Component enable flags match deployment profile
2. **Check post-orch.env**:
   - DEPLOYMENT_PROFILE matches intended environment
   - Feature flags align with requirements
   - Version tags are valid
   - Required variables are set
3. **Review environment files** in post-orch/environments/:
   - Enabled/disabled features match profile
   - Dependencies are correctly configured
   - No conflicting settings
4. **Check values overrides** in post-orch/values/:
   - Resource limits appropriate
   - Service configurations correct
   - Secrets references valid
5. **Special cases**:
   - Coder deployments: All IPs should match Coder host
   - Single-node: Ensure tolerations and node selectors
6. Report configuration summary and any issues found
