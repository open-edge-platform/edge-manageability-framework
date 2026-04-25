# Agent Guidelines for Edge Manageability Framework - Helmfile Deploy

## Code Modification Rules

### DO
- **Preserve existing patterns**: Follow the established helmfile structure and Go template syntax
- **Maintain SPDX headers**: All files start with SPDX copyright and license headers
- **Use label selectors**: Leverage helmfile's `-l` flag for targeted operations
- **Test incrementally**: Use `helmfile diff` before `sync` to preview changes
- **Check dependencies**: Understand chart dependencies before making changes
- **Source env files**: Always source `.env` files before running helmfile commands
- **Use hooks appropriately**: Leverage existing hooks for pre/post deployment logic
- **Validate YAML**: Use linters to check YAML syntax before committing

### DON'T
- **Skip linting**: This project has had recent lint/YAML issues - always validate
- **Hardcode values**: Use environment variables and helmfile environments instead
- **Break deployment order**: Some charts have hard dependencies on others
- **Bypass hooks**: The `--no-verify` or hook-skipping flags should not be used
- **Modify without testing**: Always test changes with `diff` before `sync`
- **Add unnecessary abstractions**: Keep the helmfile structure straightforward
- **Remove SPDX headers**: These are required for licensing compliance
- **Commit secrets**: Never commit actual secrets, only references to external sources

## File Patterns to Recognize

### Helmfile Templates (`.gotmpl`)
These use Go template syntax:
- `{{ .Environment.Values.feature_flag }}` - Environment-specific values
- `{{ env "VARIABLE_NAME" }}` - Environment variables from shell
- `{{ readFile "path/to/file" }}` - Include file contents
- Conditionals: `{{ if .Values.enabled }}...{{ end }}`

### Configuration Files (`.env`)
Bash-style environment variables:
```bash
PROVIDER=k3s
INSTALL_METALLB=true
EMF_TRAEFIK_IP=192.168.1.100
```

### Values Files (`.yaml`)
Standard YAML for helm chart value overrides:
```yaml
service:
  type: LoadBalancer
  annotations:
    metallb.universe.tf/loadBalancerIPs: "{{ env "EMF_TRAEFIK_IP" }}"
```

## Common Operations

### Adding a New Chart to Post-Orchestrator
1. Add repository to `repositories:` section in `helmfile.yaml.gotmpl`
2. Add release definition in `releases:` section with appropriate labels
3. Create values override file in `values/<chart-name>.yaml`
4. Add feature flag to `environments/onprem-eim-features.yaml.gotmpl`
5. Test with `helmfile -e onprem-eim -l app=<name> diff`

### Modifying Chart Configuration
1. Identify the values override file in `post-orch/values/`
2. Edit the values (preserve Go template syntax if present)
3. Preview: `helmfile -e <env> -l app=<name> diff`
4. Apply: `helmfile -e <env> -l app=<name> sync`

### Changing Deployment Order
Charts deploy in order listed in `helmfile.yaml.gotmpl`. To change:
1. Identify dependencies (check chart documentation)
2. Move release definition in the file
3. Consider using `needs:` directive for explicit dependencies
4. Test full deployment: `helmfile -e <env> sync`

### Debugging Deployment Issues
1. Check pod status: `kubectl get pods -A`
2. Review logs: `kubectl logs -n <namespace> <pod-name>`
3. Check helmfile diff: `helmfile -e <env> -l app=<name> diff`
4. Examine hooks: Review scripts in `post-orch/hooks/`
5. Verify env vars: `source post-orch.env && env | grep EMF`

## Architecture Patterns

### Label-Based Organization
All releases use labels for grouping and selection:
- `app: <chart-name>` - Primary identifier
- `tier: networking|security|data|observability` - Functional grouping
- `enabled: {{ .Values.feature_flags.chart_name }}` - Feature toggle

### Environment-Based Configuration
Three-layer configuration hierarchy:
1. **Base defaults**: `environments/defaults-disabled.yaml.gotmpl`
2. **Environment settings**: `environments/onprem-eim-settings.yaml.gotmpl`
3. **Profile overrides**: `environments/profile-vpro.yaml.gotmpl`

### Secrets Management Pattern
Secrets are never hardcoded:
1. Vault stores actual secrets
2. External Secrets Operator syncs to Kubernetes secrets
3. Charts reference Kubernetes secrets by name
4. Values files use environment variable references

## Testing Strategy

### Before Committing
1. **Lint YAML**: `yamllint helmfile.yaml.gotmpl`
2. **Validate templates**: `helmfile -e onprem-eim template` (check for errors)
3. **Diff changes**: `helmfile -e onprem-eim diff`
4. **Check git status**: Ensure no unintended files are staged

### During Development
1. **Local testing**: Use KIND cluster for rapid iteration
2. **Incremental deployment**: Use label selectors to test single charts
3. **Watch deployment**: Use `./watch-deploy.sh` or `kubectl get pods -w`
4. **Rollback plan**: Know how to revert (`helmfile destroy` then reinstall)

### Integration Testing
1. **Full deployment**: Test complete `helmfile sync` in clean cluster
2. **Upgrade path**: Test upgrading from previous version
3. **Feature toggles**: Test with features enabled/disabled
4. **Multi-environment**: Verify onprem-eim, onprem-vpro, aws profiles

## Security Considerations

### Never Commit
- Actual passwords, tokens, or API keys
- Private keys or certificates
- IP addresses for production systems (use env vars)
- Vault unseal keys or root tokens

### Always Use
- Environment variables for sensitive data
- External Secrets for production secrets
- RBAC configurations for service accounts
- TLS for all external communications

### Check For
- Exposed secrets in values files
- Insecure default passwords
- Missing authentication on services
- Open ports without firewall rules

## Git Commit Guidelines

### Commit Message Format
```
<type>: <concise description>

<optional details>

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>
```

Types: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`

### Before Committing
1. Run `git status` - check for untracked files
2. Run `git diff` - review staged changes
3. Stage specific files: `git add <file>` (avoid `git add .`)
4. Create commit with proper message format
5. Verify commit: `git log -1 --stat`

### Current Branch Context
- Working on: `EMF_HELMDEPLOY_2026_01`
- Base: `main`
- Recent work: Fixed lint and YAML issues
- Focus: Helmfile deployment improvements

## Troubleshooting Guide

### Helmfile Errors

**"environment not found"**
- Ensure environment is defined in `environments:` section
- Check spelling: `onprem-eim` vs `onprem_eim`

**"template rendering failed"**
- Check Go template syntax in `.gotmpl` files
- Verify environment variables are set (`source <file>.env`)
- Look for undefined variables in templates

**"release not found"**
- Check label selector spelling
- Verify release has matching label
- Use `helmfile list` to see all releases

### Kubernetes Issues

**Pods stuck in Pending**
- Check storage class: `kubectl get sc`
- Verify PVC creation: `kubectl get pvc -A`
- Check node resources: `kubectl describe node`

**Pods CrashLoopBackOff**
- Check logs: `kubectl logs -n <ns> <pod> --previous`
- Verify secrets exist: `kubectl get secrets -n <ns>`
- Check dependencies: Are required services running?

**Service not accessible**
- Check service: `kubectl get svc -n <ns>`
- Verify MetalLB: `kubectl get ipaddresspool -n metallb-system`
- Test internally: `kubectl run -it --rm debug --image=busybox -- wget <svc>`

## Performance Optimization

### Helmfile Operations
- Use label selectors to limit scope: `-l app=<name>`
- Parallel deployment: Set `concurrency` in helmDefaults
- Skip unchanged: Use `--skip-deps` if deps haven't changed
- Cache templates: Helmfile caches rendered templates

### Kubernetes Resources
- Set appropriate resource requests/limits in values files
- Use node selectors for workload placement
- Configure HPA for scalable services
- Monitor resource usage: `kubectl top nodes/pods`

## Documentation Standards

When adding features or making changes:
1. Update relevant README files
2. Add inline comments for complex logic
3. Document new environment variables
4. Update this file if patterns change
5. Include examples in commit messages

## Questions to Ask

Before making changes, consider:
- Does this change affect multiple charts?
- Are there dependency implications?
- Will this break existing deployments?
- Do environment-specific configs need updates?
- Are there security implications?
- Is this documented?

## Getting Help

If stuck:
1. Review existing patterns in similar charts
2. Check helmfile documentation: https://helmfile.readthedocs.io/
3. Review helm chart README: `helm show readme <repo>/<chart>`
4. Check issue tracker: GitHub issues for this project
5. Ask user for clarification on requirements
