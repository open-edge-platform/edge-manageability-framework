# Edge Manageability Framework - Helmfile Deploy

## Project Overview

This directory contains the helmfile-based deployment configuration for the **Edge Manageability Framework (EMF)**, specifically for deploying the **Edge Orchestrator** product. It replaces the previous ArgoCD-based deployment with a direct helm chart approach.

The Edge Orchestrator is a comprehensive solution for managing edge environments, providing automated application deployment, multitenancy, identity & access management, observability, and lifecycle management for edge infrastructure across geographically distributed sites.

## Architecture

The deployment is split into two main phases:

### 1. Pre-Orchestrator (`pre-orch/`)
Sets up the foundational Kubernetes cluster and infrastructure components:
- **Cluster Setup**: Supports KIND, K3s, or RKE2 providers
- **Storage**: OpenEBS LocalPV for persistent storage
- **Load Balancing**: MetalLB for external IP assignment
- **Pre-Configuration**: Namespaces, secrets, and vault setup

### 2. Post-Orchestrator (`post-orch/`)
Deploys the Edge Orchestrator application stack via helmfile:
- Network management (Traefik, HAProxy, Istio)
- Security components (Vault, External Secrets, Cert Manager, Kyverno)
- Identity management (Keycloak)
- Databases (PostgreSQL with operator, TimescaleDB)
- Observability (Prometheus, Loki, Tempo, Grafana)
- Edge-specific services (EIM, vPro management)

## Directory Structure

```
helmfile-deploy/
├── pre-orch/                          # Cluster and infrastructure setup
│   ├── pre-orch.sh                    # Main installer script
│   ├── pre-orch-config.sh             # Namespace/secret configuration
│   ├── pre-orch.env                   # Configuration file
│   ├── helmfile.yaml.gotmpl           # Helmfile for OpenEBS + MetalLB
│   ├── openebs-localpv/               # Storage provisioner values
│   └── metallb/                       # Load balancer configuration
│
└── post-orch/                         # Application deployment
    ├── post-orch-deploy.sh            # Main deployment script
    ├── post-orch.env                  # Configuration file
    ├── helmfile.yaml.gotmpl           # Main helmfile (40+ charts)
    ├── environments/                  # Environment-specific configurations
    ├── values/                        # Chart value overrides
    ├── hooks/                         # Helmfile lifecycle hooks
    └── docs/                          # Additional documentation
```

## Key Technologies

- **Helmfile**: Declarative helm chart orchestration with templating
- **Go Templates**: Used in `.gotmpl` files for dynamic configuration
- **Kubernetes Providers**: KIND (dev), K3s (edge), RKE2 (production)
- **Service Mesh**: Istio for traffic management
- **Secrets Management**: Vault + External Secrets Operator
- **Observability**: Prometheus/Grafana/Loki/Tempo stack

## Deployment Profiles

Three main deployment profiles are supported:
1. **onprem-eim**: Standard on-premises deployment with EIM
2. **onprem-vpro**: On-premises with Intel vPro management features
3. **aws**: AWS-specific configuration (future)

## Common Commands

### Pre-Orchestrator Setup
```bash
cd pre-orch/
vi pre-orch.env                         # Configure settings
./pre-orch.sh install                   # Full cluster setup
./pre-orch.sh k3s install --no-metallb  # K3s without MetalLB

# Helmfile management
cd pre-orch/
source pre-orch.env
helmfile -f helmfile.yaml.gotmpl apply
helmfile -f helmfile.yaml.gotmpl -l app=metallb destroy
```

### Post-Orchestrator Deployment
```bash
cd post-orch/
vi post-orch.env                        # Configure settings
./post-orch-deploy.sh install           # Full deployment

# Helmfile management
helmfile -e onprem-eim sync             # Deploy all charts
helmfile -e onprem-eim list             # List releases
helmfile -e onprem-eim -l app=traefik sync    # Single chart
helmfile -e onprem-eim -l app=traefik diff    # Preview changes
```

## Configuration Management

### Environment Variables
- **pre-orch.env**: Cluster provider, max pods, component flags, IPs
- **post-orch.env**: Orchestrator settings, feature flags, versions

### Helmfile Environments
Located in `post-orch/environments/`:
- `defaults-disabled.yaml.gotmpl`: Base configuration (all disabled)
- `onprem-eim-settings.yaml.gotmpl`: Common settings for on-prem
- `onprem-eim-features.yaml.gotmpl`: Feature flags per chart
- `profile-vpro.yaml.gotmpl`: vPro-specific overrides

### Values Overrides
Chart-specific values in `post-orch/values/*.yaml`:
- Named after the application (e.g., `traefik.yaml`, `vault.yaml`)
- Override default chart values for EMF requirements

## Special Considerations

### Single IP Deployments (Coder)
When deploying on Coder workspaces, set all IPs to the Coder host IP:
```bash
EMF_ORCH_IP=<coder-host-ip>
EMF_TRAEFIK_IP=<coder-host-ip>
EMF_HAPROXY_IP=<coder-host-ip>
```

### Component Dependencies
The deployment order matters:
1. Cert Manager (certificates for other services)
2. Vault (secrets backend)
3. External Secrets (injects secrets)
4. Network components (Traefik, HAProxy, Istio)
5. Databases (PostgreSQL, TimescaleDB)
6. Application services

### Helmfile Hooks
Located in `post-orch/hooks/`, executed at specific lifecycle points:
- Pre-sync: Validation and preparation
- Post-sync: Configuration and testing
- Pre-delete: Cleanup preparation

## Development Workflow

1. **Modifying Charts**: Edit values in `post-orch/values/<chart>.yaml`
2. **Preview Changes**: Use `helmfile diff` before applying
3. **Incremental Updates**: Use label selectors to update specific charts
4. **Testing**: Use `./watch-deploy.sh` to monitor rollout status
5. **Rollback**: Use `helmfile -l app=<name> delete` then reinstall

## Git Workflow

- **Current branch**: EMF_HELMDEPLOY_2026_01
- **Main branch**: main
- **Recent work**: Fixed lint and YAML issues in EMF
- Commits should include: `Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>`

## Important Files to Know

- `helmfile.yaml.gotmpl`: Main orchestration file (both pre/post)
- `*.env`: Configuration sources (source before running helmfile)
- `.gotmpl` files: Go template syntax, variables from environment
- `hooks/*.sh`: Lifecycle automation scripts
- `environments/*.yaml.gotmpl`: Environment-specific feature toggles

## Common Issues

1. **IP Conflicts**: Ensure MetalLB IP ranges don't conflict with DHCP
2. **Timeout Issues**: Increase `timeout` in helmDefaults if needed
3. **Order Dependencies**: Some charts must deploy before others
4. **Storage Class**: OpenEBS must be installed before PVC-dependent charts
5. **Secrets**: Vault must be unsealed before External Secrets can function

## Best Practices

- Always source the `.env` file before running helmfile commands
- Use `helmfile diff` to preview changes before `sync`
- Use label selectors (`-l app=<name>`) for targeted operations
- Check component status with `kubectl get pods -A` after deployment
- Review hooks in `post-orch/hooks/` when debugging deployment issues
- Test changes in development (KIND) before production (K3s/RKE2)

## Related Documentation

- Main project README: `../README.md`
- Pre-orch README: `pre-orch/README.md`
- Post-orch docs: `post-orch/docs/`
- Helmfile documentation: https://helmfile.readthedocs.io/
