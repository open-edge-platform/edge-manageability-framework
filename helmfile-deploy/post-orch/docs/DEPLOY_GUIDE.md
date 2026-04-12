<!-- SPDX-FileCopyrightText: 2026 Intel Corporation -->
<!--                                                          -->
<!-- SPDX-License-Identifier: Apache-2.0                      -->

# Helmfile Deployment Guide

Helmfile-based deployment for the Edge Manageability
Framework (EMF). This guide covers the deployment flow,
available profiles, configuration, and troubleshooting
for on-premises installations.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Folder Structure](#folder-structure)
- [Deployment Profiles](#deployment-profiles)
  - [onprem-eim](#onprem-eim)
  - [onprem-vpro](#onprem-vpro)
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

---

## Overview

The `helmfile-deploy/` folder contains everything needed
to deploy EMF onto a Kubernetes cluster using
[Helmfile](https://github.com/helmfile/helmfile).
The deployment is driven by a single script
(`post-orch-deploy.sh`) that:

1. Loads configuration from `post-orch.env`
2. Validates all required settings
3. Deploys Helm charts in dependency order
4. Automatically recovers from common failure modes

Each deployment profile determines **which components**
are installed. You choose a profile based on what features
you need (Edge Infrastructure Management only, vPro
management, Cluster Orchestration, etc.).

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| helmfile | v0.150+ | Declarative Helm management |
| helm | v3.12+ | Kubernetes package manager |
| helm-diff | latest | Preview changes |
| kubectl | matching | Kubernetes CLI |

## Folder Structure

```text
helmfile-deploy/
├── post-orch-deploy.sh        # Main script
├── post-orch-setup.sh         # Pre-deployment setup
├── post-orch.env              # Environment config
├── helmfile.yaml.gotmpl       # Release definitions
├── functions.sh               # Shared helpers
│
├── environments/              # Profiles
│   ├── defaults-disabled.yaml.gotmpl
│   ├── onprem-eim-settings.yaml.gotmpl
│   ├── onprem-eim-features.yaml.gotmpl
│   ├── profile-vpro.yaml.gotmpl
│   └── profile-coder.yaml.gotmpl
│
├── values/                    # Helm values
│   ├── traefik.yaml.gotmpl
│   ├── kyverno.yaml
│   └── ...
│
├── hooks/                     # Lifecycle hooks
│   ├── sync-db-passwords.sh
│   ├── create-tls-autocert.sh
│   └── cleanup-vault-keys.sh
│
├── logs/                      # Auto-created
├── docs/                      # Documentation
└── .computed-values/          # Auto-generated
```

## Deployment Profiles

EMF uses a layered profile system. Each profile builds
on the EIM base and adds additional components.

### onprem-eim

**Edge Infrastructure Management** — the base profile.

Deploys: **44 releases**

Includes:

- **Platform**: Keycloak, Vault, PostgreSQL,
  Traefik, cert-manager
- **Edge Infra**: inventory, host-manager,
  onboarding-manager, tinkerbell, networking-manager,
  maintenance-manager
- **AMT/vPro**: MPS, RPS, DM-Manager
- **Web UI**: admin panel, infrastructure UI
- **Supporting**: auth-service, external-secrets,
  reloader, metadata-broker, tenancy management

```bash
EMF_HELMFILE_ENV=onprem-eim ./post-orch-deploy.sh install
```

### onprem-vpro

**vPro-focused** — lightweight profile for managing
Intel vPro devices. Skips OS provisioning.

Deploys: **40 releases** (4 fewer than `onprem-eim`)

Compared to `onprem-eim`, this profile **removes**:

- `web-ui`, `web-ui-admin`, `web-ui-infra`
- `metadata-broker`
- OS provisioning (tinkerbell, PXE boot services)

And **reconfigures** infra for vPro-only scenarios:

- `infra-core`: only credentials, apiv2, inventory,
  tenant-controller
- `infra-managers`: only host-manager
- `infra-external`: only AMT services (mps, rps,
  dm-manager)

```bash
EMF_HELMFILE_ENV=onprem-vpro ./post-orch-deploy.sh install
```

### Profile Comparison

Both profiles share the same platform (Keycloak, Vault,
DB), ingress (Traefik, HAProxy), TLS, AMT/vPro (MPS,
RPS), and tenancy services. Key differences:

- **onprem-vpro** has no Web UI
- **onprem-vpro** skips OS provisioning
- **onprem-vpro** uses reduced Edge Infra footprint

### Profile Configuration Layering

Each profile is composed from multiple configuration
files applied in order (later files override earlier):

```text
onprem-eim:  defaults-disabled -> settings -> features
             -> profile-coder
onprem-vpro: defaults-disabled -> settings -> features
             -> profile-vpro
```

## Configuration

All configuration is controlled through environment
variables defined in `post-orch.env`.

### Required Variables

These **must** be set before deployment:

| Variable | Example | Description |
|---|---|---|
| `EMF_CLUSTER_DOMAIN` | `cluster.onprem` | Base domain for all services |
| `EMF_REGISTRY` | `registry-rs...` | Chart registry |
| `EMF_TRAEFIK_IP` | `192.168.99.30` | Traefik LB IP |
| `EMF_HAPROXY_IP` | `192.168.99.40` | HAProxy LB IP |
| `EMF_STORAGE_CLASS` | `openebs-hostpath` | Storage class |

> **Multi-IP mode (default):** Set `EMF_TRAEFIK_IP` and
> `EMF_HAPROXY_IP` to separate free IPs on your network.
> Both services listen on `:443`.
>
> **Single-IP mode:** Set `EMF_ORCH_IP` instead to share
> one IP for all services. Traefik listens on `:443` and
> HAProxy shifts to `:9443` to avoid port conflicts.

### Feature Toggles

Enable or disable optional features in `post-orch.env`:

- `EMF_ENABLE_ISTIO` — Istio service mesh + Kiali
  (default: `false`)
- `EMF_ENABLE_KYVERNO` — Kyverno policy engine
  (default: `false`)

### Registry

`EMF_REGISTRY` is the single variable from which all
chart, container, and OCI registry URLs are derived.

### Proxy Settings

If your environment requires an HTTP proxy, set the
following in `post-orch.env`:

- `EMF_HTTP_PROXY` — HTTP proxy URL
- `EMF_HTTPS_PROXY` — HTTPS proxy URL
- `EMF_NO_PROXY` — Comma-separated bypass list
- `EMF_EN_HTTP_PROXY` — Edge node HTTP proxy
- `EMF_EN_HTTPS_PROXY` — Edge node HTTPS proxy
- `EMF_EN_NO_PROXY` — Edge node proxy bypass list

### Deployment Profile

Set the deployment profile in `post-orch.env`:

- `EMF_HELMFILE_ENV` — Profile name
  (default: `onprem-eim`).
  Options: `onprem-eim`, `onprem-vpro`

## Deployment

### Step 1: Configure Environment

Edit `post-orch.env` with your environment values:

```bash
cd helmfile-deploy/

# Edit the configuration file
vi post-orch.env
```

At minimum, update:

- `EMF_CLUSTER_DOMAIN`
- `EMF_TRAEFIK_IP` and `EMF_HAPROXY_IP`
  (free IPs for LoadBalancer services)
- `EMF_HTTP_PROXY` / `EMF_NO_PROXY`
  (if behind a proxy, otherwise clear them)
- `EMF_DOCKER_USERNAME` / `EMF_DOCKER_PASSWORD`

### Step 2: Run Setup

Prepare the cluster for EMF deployment:

```bash
./post-orch-setup.sh install
```

This script:

- Creates all required Kubernetes namespaces
- Generates and stores initial secrets and passwords
- Configures Gitea for chart/config storage
  (only when App Orchestration is enabled)

To tear down the setup:

```bash
./post-orch-setup.sh uninstall
```

> **Note:** Only run `uninstall` **after** you have
> uninstalled all Helm releases with
> `./post-orch-deploy.sh uninstall`.

### Step 3: Deploy

Set `EMF_HELMFILE_ENV` in `post-orch.env` to your
desired profile, then run:

```bash
./post-orch-deploy.sh install
```

The script will:

1. Validate your configuration
2. Add all Helm repositories
3. Deploy each release in dependency order
4. Print a summary with pass/fail status

You can also pass inline overrides:

```bash
EMF_TRAEFIK_IP=10.0.1.50 ./post-orch-deploy.sh install
```

Logs are saved to
`logs/<profile>_install_<timestamp>.log`.

### Deploying a Single Chart

```bash
# Deploy only traefik
./post-orch-deploy.sh install traefik

# Deploy only platform-keycloak
./post-orch-deploy.sh install platform-keycloak
```

### Preview Changes

See what would change without applying:

```bash
./post-orch-deploy.sh diff
./post-orch-deploy.sh diff traefik
```

### Uninstall

```bash
# Uninstall everything (reverse order)
./post-orch-deploy.sh uninstall

# Uninstall a single chart
./post-orch-deploy.sh uninstall traefik
```

### List Releases

```bash
./post-orch-deploy.sh list
```

## Deployment Flow

The deployment installs charts in dependency order.
Major groups are:

```text
 1. Operators      keycloak-operator, postgresql-operator
 2. Infra          namespace-label, cert-manager
 3. TLS & Ingress  self-signed-cert, traefik, haproxy
 4. Database       postgresql-cluster, postgresql-secrets
 5. Platform       vault, secrets-config, keycloak
 6. Tenancy        tenancy-datamodel, tenancy-manager
 7. Edge Infra     infra-core, infra-managers
 8. Web UI         web-ui-admin, web-ui-infra
```

Each release is deployed individually with
`helmfile sync`. If a release fails, the script
continues and reports all failures in the summary.

## Re-running After Failure

The deploy script is **safe to re-run**:

```bash
./post-orch-deploy.sh install
```

The script automatically handles common rerun issues:

| Issue | Automatic Fix |
|---|---|
| Jobs are immutable | Deletes stale Jobs first |
| Helm stuck in `failed` | Rolls back or reinstalls |
| DB password mismatch | Hook syncs passwords |
| Connection-string stale | Hook updates secrets |

If a specific release keeps failing, deploy it
individually to see detailed output:

```bash
./post-orch-deploy.sh install platform-keycloak
```

## Troubleshooting

### Check Deployment Status

```bash
# List all Helm releases
helm list -A -a

# Check for failed releases
helm list -A -a | grep -E "failed|pending"

# Check pod status
kubectl get pods -n orch-platform
kubectl get pods -n orch-infra
kubectl get pods -n orch-database
```

### Common Issues

#### Jobs Fail: "spec.template is immutable"

**Cause:** Kubernetes Jobs cannot be modified after
creation. On rerun, Helm tries to patch the existing Job.

**Fix:** The deploy script handles this automatically.
If using helmfile directly, delete the Job manually:

```bash
kubectl get jobs -n <namespace>
kubectl delete job <job-name> -n <namespace>
```

#### PostgreSQL Password Auth Failed (28P01)

**Cause:** `postgresql-secrets` generates new random
passwords on every `helm upgrade`, but PostgreSQL still
has the old passwords.

**Fix:** The post-sync hook handles this automatically.
For manual fix:

```bash
PG_POD=$(kubectl get pods -n orch-database \
  -l cnpg.io/cluster=postgresql-cluster \
  -o jsonpath='{.items[0].metadata.name}')

for secret in $(kubectl get secrets \
  -n orch-database \
  -l managed-by=edge-manageability-framework \
  --field-selector type=kubernetes.io/basic-auth \
  -o jsonpath='{range .items[*]}{.metadata.name}{" "}{end}')
do
  user=$(kubectl get secret "$secret" \
    -n orch-database \
    -o jsonpath='{.data.username}' | base64 -d)
  pass=$(kubectl get secret "$secret" \
    -n orch-database \
    -o jsonpath='{.data.password}' | base64 -d)
  kubectl exec -n orch-database "$PG_POD" \
    -c postgres -- \
    psql -U postgres \
    -c "ALTER ROLE \"$user\" WITH PASSWORD '$pass';"
done
```

Then restart affected pods:

```bash
kubectl delete pod platform-keycloak-0 -n orch-platform
```

#### Helm Release Stuck in "failed" State

```bash
# Find last good revision
helm history <release> -n <namespace>

# Rollback to it
helm rollback <release> <revision> -n <namespace>

# If no good revision, uninstall and re-deploy
helm uninstall <release> -n <namespace>
./post-orch-deploy.sh install <release>
```

#### Pods not Starting — Check Logs

```bash
# Describe the pod for events
kubectl describe pod <pod-name> -n <namespace>

# Check container logs
kubectl logs <pod-name> -n <namespace> --tail=50

# Check previous container logs (if restarting)
kubectl logs <pod-name> -n <namespace> \
  --previous --tail=50
```

### View Deployment Logs

Every deployment creates a timestamped log file:

```bash
ls -lt helmfile-deploy/logs/
```

### Inspect Computed Values

After a deployment, the resolved Helm values are saved
to `.computed-values/`.
