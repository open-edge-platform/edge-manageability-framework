# Helmfile Deployment Guide

Helmfile-based deployment for the Edge Manageability Framework (EMF).
This guide covers the deployment flow, available profiles, configuration, and troubleshooting for on-premises installations.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Folder Structure](#folder-structure)
- [Deployment Profiles](#deployment-profiles)
  - [onprem-eim](#onprem-eim)
  - [onprem-vpro](#onprem-vpro)
  - [onprem-eim-co](#onprem-eim-co)
  - [Profile Comparison](#profile-comparison)
- [Configuration](#configuration)
  - [Required Variables](#required-variables)
  - [Feature Toggles](#feature-toggles)
  - [Registry](#registry)
  - [Proxy Settings](#proxy-settings)
- [Deployment](#deployment)
  - [Step 1: Configure Environment](#step-1-configure-environment)
  - [Step 2: Run Setup](#step-2-run-setup)
  - [Step 3: Deploy](#step-3-deploy)
  - [Deploying a Single Chart](#deploying-a-single-chart)
  - [Preview Changes](#preview-changes)
  - [Uninstall](#uninstall)
- [Deployment Flow](#deployment-flow)
- [Re-running After Failure](#re-running-after-failure)
- [Troubleshooting](#troubleshooting)
- [Advanced Usage](#advanced-usage)

---

## Overview

The `helmfile-deploy/` folder contains everything needed to deploy EMF onto a Kubernetes cluster using [Helmfile](https://github.com/helmfile/helmfile). The deployment is driven by a single script (`orch-deploy.sh`) that:

1. Loads configuration from `onprem.env`
2. Validates all required settings
3. Deploys Helm charts one-by-one in dependency order
4. Automatically recovers from common failure modes on rerun

Each deployment profile determines **which components** are installed. You choose a profile based on what features you need (Edge Infrastructure Management only, vPro management, Cluster Orchestration, etc.).

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| [helmfile](https://github.com/helmfile/helmfile) | v0.150+ | Declarative Helm chart management |
| [helm](https://helm.sh/) | v3.12+ | Kubernetes package manager |
| [helm-diff](https://github.com/databus23/helm-diff) | latest | Preview changes before applying |
| kubectl | matching cluster | Kubernetes CLI (configured for target cluster) |

## Folder Structure

```
helmfile-deploy/
├── orch-deploy.sh              # Main deployment script (install/uninstall/diff/list)
├── orch-setup.sh               # Pre-deployment setup (creates namespaces)
├── onprem.env                  # Environment variable configuration
├── helmfile.yaml.gotmpl        # Main helmfile template (all 106 release definitions)
├── functions.sh                # Shared helper functions
│
├── environments/               # Profile definitions (which components to enable)
│   ├── defaults-disabled.yaml.gotmpl       # Base: all optional components disabled
│   ├── onprem-eim-settings.yaml.gotmpl     # Global settings (registry, storage, DB)
│   ├── onprem-eim-features.yaml.gotmpl     # EIM feature toggles
│   ├── profile-vpro.yaml.gotmpl            # vPro overrides
│   ├── profile-co.yaml.gotmpl              # Cluster Orchestration overrides
│   ├── onprem-eim-co-ao.yaml.gotmpl        # CO + App Orchestration
│   └── onprem-eim-co-ao-o11y.yaml.gotmpl   # Full stack (all features)
│
├── values/                     # Helm values per chart (130+ files)
│   ├── traefik.yaml.gotmpl     # .gotmpl files read EMF_* env vars
│   ├── kyverno.yaml            # .yaml files are static values
│   └── ...
│
├── hooks/                      # Lifecycle hooks
│   ├── sync-db-passwords.sh    # Post-sync: syncs DB passwords after upgrade
│   ├── create-tls-autocert.sh  # TLS certificate automation
│   └── helmfile.yaml           # Hook definitions
│
├── logs/                       # Deployment logs (auto-created)
├── docs/                       # Additional documentation
└── .computed-values/           # Resolved helmfile values (auto-generated)
```

## Deployment Profiles

EMF uses a layered profile system. Each profile builds on the EIM base and adds additional components.

### onprem-eim

**Edge Infrastructure Management** — the base on-premises profile.

Deploys: **44 releases**

Includes:
- **Platform services**: Keycloak (auth), Vault (secrets), PostgreSQL (database), Traefik (ingress), cert-manager (TLS)
- **Edge Infrastructure**: inventory, host-manager, onboarding-manager, tinkerbell (provisioning), networking-manager, maintenance-manager, and more
- **AMT/vPro support**: MPS, RPS, DM-Manager (device management)
- **Web UI**: admin panel, infrastructure management UI
- **Supporting services**: auth-service, external-secrets, reloader, metadata-broker, tenancy management

```bash
EMF_HELMFILE_ENV=onprem-eim ./orch-deploy.sh install
```

### onprem-vpro

**vPro-focused** — lightweight profile for managing Intel vPro devices (~100 nodes).

Deploys: **40 releases** (4 fewer than `onprem-eim`)

Compared to `onprem-eim`, this profile **removes**:
- `web-ui`, `web-ui-admin`, `web-ui-infra` (no browser UI)
- `metadata-broker`

And **reconfigures** the infra components for vPro-only scenarios (reduced footprint):
- `infra-core`: only credentials, apiv2, inventory, tenant-controller
- `infra-managers`: only host-manager
- `infra-external`: only AMT services (mps, rps, dm-manager)

```bash
EMF_HELMFILE_ENV=onprem-vpro ./orch-deploy.sh install
```

### onprem-eim-co

**EIM + Cluster Orchestration** — adds the ability to manage Kubernetes clusters on edge nodes.

Deploys: **54 releases** (10 more than `onprem-eim`)

Compared to `onprem-eim`, this profile **adds**:
- `capi-operator-pre`, `capi-operator` — Cluster API operator
- `capi-providers-config` — CAPI provider configuration
- `cluster-manager` — cluster lifecycle management
- `cluster-connect-gateway`, `cluster-connect-gateway-crd` — remote cluster connectivity
- `cluster-template-crd` — cluster templates
- `intel-infra-provider`, `intel-infra-provider-crds` — Intel infrastructure provider for CAPI
- `web-ui-cluster-orch` — cluster orchestration UI

```bash
EMF_HELMFILE_ENV=onprem-eim-co ./orch-deploy.sh install
```

### Profile Comparison

| Component | onprem-eim | onprem-vpro | onprem-eim-co |
|-----------|:----------:|:-----------:|:-------------:|
| **Total releases** | 44 | 40 | 54 |
| Platform (Keycloak, Vault, DB) | ✅ | ✅ | ✅ |
| Ingress (Traefik, HAProxy) | ✅ | ✅ | ✅ |
| TLS & Certificates | ✅ | ✅ | ✅ |
| Edge Infra (inventory, provisioning) | ✅ | ✅ (reduced) | ✅ |
| AMT/vPro (MPS, RPS) | ✅ | ✅ | ✅ |
| Tenancy management | ✅ | ✅ | ✅ |
| Metadata broker | ✅ | ❌ | ✅ |
| Web UI | ✅ | ❌ | ✅ |
| Cluster Orchestration (CAPI) | ❌ | ❌ | ✅ |
| Cluster Orch UI | ❌ | ❌ | ✅ |

### Profile Configuration Layering

Each profile is composed from multiple configuration files applied in order (later files override earlier ones):

```
onprem-eim:
  1. defaults-disabled.yaml.gotmpl     ← All optional components OFF
  2. onprem-eim-settings.yaml.gotmpl   ← Global settings (registry, storage, DB)
  3. onprem-eim-features.yaml.gotmpl   ← EIM components ON

onprem-vpro:
  1. defaults-disabled.yaml.gotmpl
  2. onprem-eim-settings.yaml.gotmpl
  3. onprem-eim-features.yaml.gotmpl   ← EIM components ON
  4. profile-vpro.yaml.gotmpl          ← Disables UI, reduces infra footprint

onprem-eim-co:
  1. defaults-disabled.yaml.gotmpl
  2. onprem-eim-settings.yaml.gotmpl
  3. onprem-eim-features.yaml.gotmpl   ← EIM components ON
  4. profile-co.yaml.gotmpl            ← Enables CAPI, cluster-manager, etc.
```

## Configuration

All configuration is controlled through environment variables defined in `onprem.env`.

### Required Variables

These **must** be set before deployment:

| Variable | Example | Description |
|----------|---------|-------------|
| `EMF_CLUSTER_NAME` | `onprem` | Name of the Kubernetes cluster |
| `EMF_CLUSTER_DOMAIN` | `cluster.onprem` | Base domain for all services |
| `EMF_REGISTRY` | `registry-rs.edgeorchestration.intel.com` | Container/chart registry |
| `EMF_TRAEFIK_IP` | `192.168.99.30` | Static IP for Traefik load balancer |
| `EMF_HAPROXY_IP` | `192.168.99.40` | Static IP for HAProxy load balancer |
| `EMF_STORAGE_CLASS` | `openebs-hostpath` | Kubernetes storage class |

### Feature Toggles

Enable or disable optional features:

| Variable | Default | Description |
|----------|---------|-------------|
| `EMF_ENABLE_ISTIO` | `false` | Istio service mesh + Kiali dashboard |
| `EMF_ENABLE_EMAIL` | `false` | Email alerting via SMTP |
| `EMF_ENABLE_KYVERNO` | `false` | Kyverno policy engine |
| `EMF_ENABLE_PXE` | `true` | PXE boot services for bare-metal provisioning |
| `EMF_ENABLE_SQUID` | `false` | Squid proxy for edge nodes |

### Registry

`EMF_REGISTRY` is the single variable from which all registry URLs are derived:

```
EMF_REGISTRY=registry-rs.edgeorchestration.intel.com
  → Chart repository:      registry-rs.edgeorchestration.intel.com/edge-orch
  → Container registry:    registry-rs.edgeorchestration.intel.com/edge-orch
  → OCI registry:          registry-rs.edgeorchestration.intel.com
  → File server:           files-rs.edgeorchestration.intel.com
```

### Proxy Settings

If your environment requires an HTTP proxy:

| Variable | Description |
|----------|-------------|
| `EMF_HTTP_PROXY` | HTTP proxy URL |
| `EMF_HTTPS_PROXY` | HTTPS proxy URL |
| `EMF_NO_PROXY` | Comma-separated bypass list |
| `EMF_EN_HTTP_PROXY` | Edge node HTTP proxy (can differ from orchestrator) |
| `EMF_EN_HTTPS_PROXY` | Edge node HTTPS proxy |
| `EMF_EN_NO_PROXY` | Edge node bypass list |

## Deployment

### Step 1: Configure Environment

Edit `onprem.env` with values specific to your environment:

```bash
cd helmfile-deploy/

# Edit the configuration file
vi onprem.env
```

At minimum, update:
- `EMF_CLUSTER_NAME` and `EMF_CLUSTER_DOMAIN`
- `EMF_TRAEFIK_IP` and `EMF_HAPROXY_IP` (free IPs in your network for LoadBalancer services)
- `EMF_HTTP_PROXY` / `EMF_NO_PROXY` (if behind a proxy, otherwise clear them)
- `EMF_DOCKER_USERNAME` / `EMF_DOCKER_PASSWORD` (if using Docker Hub images)

### Step 2: Run Setup

Create the required Kubernetes namespaces:

```bash
./orch-setup.sh
```

This creates: `orch-boots`, `orch-database`, `orch-platform`, `orch-app`, `orch-cluster`, `orch-infra`, `orch-sre`, `orch-ui`, `orch-secret`, `orch-gateway`, `orch-harbor`, and others.

### Step 3: Deploy

Choose your profile and run:

```bash
# EIM only (44 releases)
EMF_HELMFILE_ENV=onprem-eim ./orch-deploy.sh install

# vPro only (40 releases)
EMF_HELMFILE_ENV=onprem-vpro ./orch-deploy.sh install

# EIM + Cluster Orchestration (54 releases)
EMF_HELMFILE_ENV=onprem-eim-co ./orch-deploy.sh install
```

The script will:
1. Validate your configuration (errors abort, warnings are shown)
2. Add all Helm repositories
3. Deploy each release in dependency order
4. Show live progress: `[15/44] 📦 Deploying: platform-keycloak`
5. Print a summary with pass/fail counts and per-release timings

You can also pass inline overrides:

```bash
EMF_HELMFILE_ENV=onprem-eim EMF_TRAEFIK_IP=10.0.1.50 ./orch-deploy.sh install
```

Logs are saved to `logs/<profile>_install_<timestamp>.log`.

### Deploying a Single Chart

```bash
# Deploy only traefik
EMF_HELMFILE_ENV=onprem-eim ./orch-deploy.sh install traefik

# Deploy only platform-keycloak
EMF_HELMFILE_ENV=onprem-eim ./orch-deploy.sh install platform-keycloak
```

### Preview Changes

See what would change without applying:

```bash
EMF_HELMFILE_ENV=onprem-eim ./orch-deploy.sh diff
EMF_HELMFILE_ENV=onprem-eim ./orch-deploy.sh diff traefik
```

### Uninstall

```bash
# Uninstall everything (reverse order)
EMF_HELMFILE_ENV=onprem-eim ./orch-deploy.sh uninstall

# Uninstall a single chart
EMF_HELMFILE_ENV=onprem-eim ./orch-deploy.sh uninstall traefik
```

### List Releases

```bash
EMF_HELMFILE_ENV=onprem-eim ./orch-deploy.sh list
```

## Deployment Flow

The deployment installs charts in a fixed dependency order. Major groups are:

```
1. Operators           keycloak-operator, postgresql-operator
2. Infrastructure      namespace-label, cert-manager, external-secrets
3. TLS & Ingress       self-signed-cert, platform-autocert, traefik, haproxy
4. Database            postgresql-cluster, postgresql-secrets
5. Platform            vault, secrets-config, platform-keycloak, auth-service
6. Tenancy             tenancy-datamodel, tenancy-manager, nexus-api-gw
7. Edge Infra          infra-core, infra-managers, infra-external, infra-onboarding
8. Cluster Orch        capi-operator, cluster-manager, intel-infra-provider (onprem-eim-co only)
9. Web UI              web-ui-admin, web-ui-infra, web-ui (root proxy)
```

Each release is deployed individually with `helmfile sync`. If a release fails, the script continues to the next one and reports all failures in the summary.

## Re-running After Failure

The deploy script is designed to be **safe to re-run**. Simply run the same command again:

```bash
EMF_HELMFILE_ENV=onprem-eim-co ./orch-deploy.sh install
```

The script automatically handles common rerun issues:

| Issue | Automatic Fix |
|-------|--------------|
| Kubernetes Jobs are immutable (can't be patched on upgrade) | Deletes all stale Jobs before each release sync |
| Helm release stuck in `failed` or `pending-*` state | Rolls back to last good revision or uninstalls for fresh install |
| PostgreSQL password mismatch after password regeneration | Post-sync hook syncs K8s secret passwords into PostgreSQL |
| Derived connection-string secrets out of sync | Hook updates `mps`/`rps` secrets and restarts their deployments |
| Helmfile hook failure mistaken as release failure | Checks actual Helm release status to detect false positives |

If a specific release keeps failing, you can deploy it individually to see detailed output:

```bash
EMF_HELMFILE_ENV=onprem-eim ./orch-deploy.sh install platform-keycloak
```

## Troubleshooting

### Check Deployment Status

```bash
# List all Helm releases and their status
helm list -A -a

# Check for failed releases
helm list -A -a | grep -E "failed|pending"

# Check pod status in a specific namespace
kubectl get pods -n orch-platform
kubectl get pods -n orch-infra
kubectl get pods -n orch-database
```

### Common Issues

#### Jobs Fail with "spec.template: field is immutable"

**Cause:** Kubernetes Jobs cannot be modified after creation. On rerun, Helm tries to patch the existing Job.

**Fix:** The deploy script handles this automatically. If using helmfile directly, delete the Job manually:

```bash
kubectl get jobs -n <namespace>
kubectl delete job <job-name> -n <namespace>
```

#### PostgreSQL Password Authentication Failed (28P01)

**Cause:** The `postgresql-secrets` chart generates new random passwords on every `helm upgrade`, but PostgreSQL still has the old passwords.

**Fix:** The deploy script's post-sync hook handles this automatically. For manual fix:

```bash
PG_POD=$(kubectl get pods -n orch-database -l cnpg.io/cluster=postgresql-cluster \
  -o jsonpath='{.items[0].metadata.name}')

for secret in $(kubectl get secrets -n orch-database \
  -l managed-by=edge-manageability-framework \
  --field-selector type=kubernetes.io/basic-auth \
  -o jsonpath='{range .items[*]}{.metadata.name}{" "}{end}'); do

  user=$(kubectl get secret "$secret" -n orch-database -o jsonpath='{.data.username}' | base64 -d)
  pass=$(kubectl get secret "$secret" -n orch-database -o jsonpath='{.data.password}' | base64 -d)
  kubectl exec -n orch-database "$PG_POD" -c postgres -- \
    psql -U postgres -c "ALTER ROLE \"$user\" WITH PASSWORD '$pass';"
done
```

Then restart affected pods (e.g., `kubectl delete pod platform-keycloak-0 -n orch-platform`).

#### Helm Release Stuck in "failed" State

```bash
# Find last good revision
helm history <release> -n <namespace>

# Rollback to it
helm rollback <release> <revision> -n <namespace>

# If no good revision exists, uninstall and re-deploy
helm uninstall <release> -n <namespace>
EMF_HELMFILE_ENV=onprem-eim ./orch-deploy.sh install <release>
```

#### Pods not Starting — Check Logs

```bash
# Describe the pod for events
kubectl describe pod <pod-name> -n <namespace>

# Check container logs
kubectl logs <pod-name> -n <namespace> --tail=50

# Check previous container logs (if restarting)
kubectl logs <pod-name> -n <namespace> --previous --tail=50
```

### View Deployment Logs

Every deployment creates a timestamped log file:

```bash
ls -lt helmfile-deploy/logs/
# Example: onprem-eim-co_install_20260406-073828.log
```

## Advanced Usage

### Using helmfile Directly

For fine-grained control, you can use helmfile directly instead of `orch-deploy.sh`:

```bash
# Load environment variables
set -a; source onprem.env; set +a

# Full deployment
helmfile -e onprem-eim sync

# Single chart
helmfile -e onprem-eim -l app=traefik sync

# Multiple charts
helmfile -e onprem-eim -l app=traefik -l app=platform-keycloak sync

# Preview changes
helmfile -e onprem-eim diff

# Destroy
helmfile -e onprem-eim destroy
```

> **Note:** When using helmfile directly, the automatic Job cleanup, password sync, and failed-release recovery provided by `orch-deploy.sh` are **not** available. You will need to handle these manually on rerun (see Troubleshooting above).

### Inspect Computed Values

After a deployment, the resolved Helm values are saved to `.computed-values/`

