# Edge Manageability Framework - Helmfile Deployment

Helmfile-based deployment for the Edge Manageability Framework.
Replaces the ArgoCD-based deployment with direct Helm chart releases managed by [Helmfile](https://github.com/helmfile/helmfile).

## Prerequisites

- [helmfile](https://github.com/helmfile/helmfile) v0.150+
- [helm](https://helm.sh/) v3.12+
- [helm-diff](https://github.com/databus23/helm-diff) plugin
- kubectl configured for target cluster

## Directory Structure

```
debug/
├── helmfile.yaml                          # Main helmfile (104 releases, labeled)
├── post-deploy-orch.sh                    # Deployment script (install/uninstall/list)
├── onprem.env                             # On-premises environment variables
├── aws.env                                # AWS environment variables
├── environments/                          # Environment-specific configs (.gotmpl)
│   ├── defaults.yaml.gotmpl              # Shared defaults (all EMF_* env vars)
│   ├── onprem.yaml.gotmpl                # On-premises full
│   ├── onprem-1k.yaml.gotmpl             # On-premises 1K scale
│   ├── onprem-oxm.yaml.gotmpl            # On-premises OXM
│   ├── onprem-explicit-proxy.yaml.gotmpl  # On-premises with squid proxy
│   ├── aws.yaml.gotmpl                   # AWS (Aurora, EFS, Vault HA)
│   ├── vpro.yaml.gotmpl                  # vPro (platform only)
│   ├── eim.yaml.gotmpl                   # EIM (platform + edge infra)
│   ├── eim-co.yaml.gotmpl                # EIM + Cluster Orchestration
│   ├── eim-co-ao.yaml.gotmpl             # EIM + CO + App Orchestration
│   ├── eim-co-ao-o11y.yaml.gotmpl        # EIM + CO + AO + Observability
│   ├── dev.yaml.gotmpl                   # KIND local development
│   ├── dev-minimal.yaml.gotmpl           # KIND minimal
│   └── bkc.yaml.gotmpl                   # BKC AWS
└── values/                                # Helm values per release
    ├── traefik.yaml.gotmpl                # .gotmpl = env var aware
    ├── kyverno.yaml                       # .yaml = static values
    └── ...
```

## Quick Start

### Using post-deploy-orch.sh (recommended)

```bash
# Edit onprem.env with your values, then:
./post-deploy-orch.sh install                  # Full deployment
./post-deploy-orch.sh install traefik          # Install single chart
./post-deploy-orch.sh uninstall traefik        # Uninstall single chart
./post-deploy-orch.sh uninstall                # Full teardown
./post-deploy-orch.sh list                     # List all charts
```

Change the profile with `EMF_HELMFILE_ENV`:
```bash
EMF_HELMFILE_ENV=eim ./post-deploy-orch.sh install
```

### Using helmfile directly

```bash
# Load environment variables
set -a; source onprem.env; set +a

# Full deployment
helmfile -e onprem sync

# Individual chart install/uninstall (uses labels)
helmfile -e onprem -l app=traefik sync         # install
helmfile -e onprem -l app=traefik destroy      # uninstall
helmfile -e onprem -l app=traefik diff         # preview

# Multiple charts
helmfile -e onprem -l app=traefik -l app=haproxy sync

# List releases
helmfile -e onprem list

# Destroy everything
helmfile -e onprem destroy
```

## Profiles (Environments)

| Profile | Description |
|---|---|
| `onprem` | On-premises with all features |
| `onprem-1k` | On-premises optimized for 1K edge nodes |
| `onprem-oxm` | On-premises OXM (reduced features) |
| `onprem-explicit-proxy` | On-premises with squid proxy |
| `aws` | AWS (Aurora DB, EFS, Vault HA, target groups) |
| `vpro` | vPro - platform only (no AO/CO/O11y/email) |
| `eim` | EIM - platform + edge infra (no AO/CO/O11y/email) |
| `eim-co` | EIM + Cluster Orchestration |
| `eim-co-ao` | EIM + CO + App Orchestration |
| `eim-co-ao-o11y` | Full stack (EIM + CO + AO + Observability) |
| `dev` | Local KIND cluster with all features |
| `dev-minimal` | Local KIND, no observability/kyverno |
| `bkc` | AWS BKC with all features |

## Environment Variables

All configuration is controlled via `EMF_*` environment variables.
See [onprem.env](onprem.env) for the full list with defaults.

### Key Variables

| Variable | Default | Description |
|---|---|---|
| `EMF_CLUSTER_NAME` | (per env) | Cluster name |
| `EMF_CLUSTER_DOMAIN` | (per env) | Base domain for services |
| `EMF_REGISTRY` | `registry-rs.edgeorchestration.intel.com/edge-orch` | Single source for all chart/image/OCI/file URLs |
| `EMF_SERVICE_TYPE` | `LoadBalancer` | Kubernetes service type |
| `EMF_TRAEFIK_IP` | (unset) | Traefik load balancer IP |
| `EMF_HAPROXY_IP` | (unset) | HAProxy load balancer IP |
| `EMF_STORAGE_CLASS` | (per env) | Kubernetes storage class |
| `EMF_VAULT_HA` | `false` | Vault HA mode |
| `EMF_DB_TYPE` | `local` | Database type |
| `EMF_HTTP_PROXY` | (unset) | HTTP proxy URL |
| `EMF_HELMFILE_ENV` | `onprem` | Helmfile environment (for post-deploy-orch.sh) |

### Feature Toggles

| Variable | Default | Description |
|---|---|---|
| `EMF_ENABLE_EMAIL` | `true` | Enable/disable email alerting (alerting-monitor) |
| `EMF_ENABLE_ISTIO` | `true` | Enable/disable Istio service mesh + Kiali |
| `EMF_ENABLE_PXE` | `true` | Enable/disable PXE boot services |
| `EMF_ENABLE_SQUID` | `false` | Enable/disable Squid proxy |
| `EMF_ENABLE_AWS_LB` | `false` | Enable/disable AWS load balancer controller |
| `EMF_ENABLE_AWS_SM` | `false` | Enable/disable AWS Secrets Manager proxy |
| `EMF_ENABLE_AUTOSCALER` | `false` | Enable/disable cluster autoscaler |
| `EMF_ENABLE_VPA` | `false` | Enable/disable vertical pod autoscaler |
| `EMF_GITEA_ENABLED` | `false` | Install Gitea via post-deploy-orch.sh |

### Registry

The `EMF_REGISTRY` variable is the single source from which all registry URLs are derived:

```
EMF_REGISTRY=registry-rs.edgeorchestration.intel.com/edge-orch
  → chartRepoURL:          registry-rs.edgeorchestration.intel.com/edge-orch
  → containerRegistryURL:  registry-rs.edgeorchestration.intel.com/edge-orch
  → ociRegistry:           registry-rs.edgeorchestration.intel.com
  → fileServer:            files-rs.edgeorchestration.intel.com
```

## Pre-Deploy Validation

The `post-deploy-orch.sh` script automatically validates configuration before deploying.
Validation runs on every `install` and `uninstall` action.

### Errors (abort deployment)

| Check | Condition |
|---|---|
| Valid profile | `EMF_HELMFILE_ENV` must be a recognized profile name |
| Cluster name | `EMF_CLUSTER_NAME` must be set |
| Cluster domain | `EMF_CLUSTER_DOMAIN` must be set |
| Registry | `EMF_REGISTRY` must be set |
| IP format | `EMF_TRAEFIK_IP` / `EMF_HAPROXY_IP` must be valid IPv4 if set |
| OXM PXE vars | `EMF_OXM_PXE_SERVER_INT`, `_IP`, `_SUBNET` required for `onprem-oxm` profile |

### Warnings (non-blocking)

| Check | Condition |
|---|---|
| Load balancer IPs | `EMF_TRAEFIK_IP` / `EMF_HAPROXY_IP` not set when `EMF_SERVICE_TYPE=LoadBalancer` |
| SMTP address | `EMF_ENABLE_EMAIL=true` but `EMF_SMTP_ADDRESS` not set |
| SRE password | `EMF_SRE_USERNAME` set but `EMF_SRE_PASSWORD` empty |
| No-proxy list | `EMF_HTTP_PROXY` set but `EMF_NO_PROXY` empty |
