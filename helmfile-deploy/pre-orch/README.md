<!-- SPDX-FileCopyrightText: 2026 Intel Corporation -->
<!--                                                          -->
<!-- SPDX-License-Identifier: Apache-2.0                      -->

# Pre-Orchestrator Setup

Sets up a Kubernetes cluster (KIND, K3s, or RKE2) and optionally installs
**OpenEBS LocalPV** and **MetalLB** via helmfile.

## Quick Start

```bash
# Edit configuration
vi pre-orch.env

# Install cluster + all pre-orch components
./pre-orch.sh install

# Uninstall cluster
./pre-orch.sh uninstall
```

## Configuration

All settings are in [`pre-orch.env`](pre-orch.env). CLI flags override env values.

| Variable | Default | Description |
|---|---|---|
| `PROVIDER` | `k3s` | Kubernetes provider: `kind`, `k3s`, `rke2` |
| `MAX_PODS` | `500` | Kubelet max pods |
| `INSTALL_OPENEBS` | `true` | Install OpenEBS LocalPV |
| `INSTALL_METALLB` | `true` | Install MetalLB |
| `INSTALL_PRE_CONFIG` | `true` | Run pre-orch-config (namespaces, secrets) |
| `LOCALPV_VERSION` | `4.3.0` | OpenEBS LocalPV chart version |
| `EMF_TRAEFIK_IP` | — | Traefik LB IP (for MetalLB) |
| `EMF_HAPROXY_IP` | — | HAProxy LoadBalancer IP (required for MetalLB) |

## Skipping Components During Install

Use `--no-openebs`, `--no-metallb`, and/or
`--no-pre-config` to skip components during setup:

```bash
# Install cluster only (no OpenEBS, no MetalLB, no pre-config)
./pre-orch.sh k3s install --no-openebs --no-metallb --no-pre-config

# Install cluster + OpenEBS only (no MetalLB)
./pre-orch.sh k3s install --no-metallb --no-pre-config

# Install cluster + MetalLB only (no OpenEBS)
./pre-orch.sh k3s install --no-openebs --no-pre-config

# Install everything including pre-config (default)
./pre-orch.sh k3s install
```

## Installing / Uninstalling Components via Helmfile

If you skipped OpenEBS and/or MetalLB during cluster install, you can manage
them later using the helmfile directly. First source the env file, then run
from the `pre-orch/` directory:

```bash
cd helmfile-deploy/pre-orch
source pre-orch.env
```

```bash
# Install all components
helmfile -f helmfile.yaml.gotmpl apply

# Install individual components
helmfile -f helmfile.yaml.gotmpl -l app=openebs-localpv apply
helmfile -f helmfile.yaml.gotmpl -l app=metallb apply
helmfile -f helmfile.yaml.gotmpl -l app=metallb-config apply

# Uninstall all components
helmfile -f helmfile.yaml.gotmpl destroy

# Uninstall individual components
helmfile -f helmfile.yaml.gotmpl -l app=metallb-config destroy
helmfile -f helmfile.yaml.gotmpl -l app=metallb destroy
helmfile -f helmfile.yaml.gotmpl -l app=openebs-localpv destroy
```

## Directory Structure

```text
pre-orch/
├── pre-orch.sh              # Main installer script (cluster + storage + LB)
├── pre-orch-config.sh       # Pre-deploy config (namespaces, secrets, vault)
├── pre-orch.env             # Configuration file
├── functions.sh             # Shared helper functions
├── helmfile.yaml.gotmpl     # Combined helmfile (OpenEBS + MetalLB)
├── README.md
├── openebs-localpv/         # OpenEBS LocalPV values
└── metallb/                 # MetalLB values + local config chart
```

## Pre-Deploy Configuration (pre-orch-config.sh)

Creates namespaces, Keycloak/Postgres secrets, and validates vault keys.
Runs automatically at the end of `pre-orch.sh install` (controlled by `INSTALL_PRE_CONFIG`).
Can also be run standalone:

```bash
./pre-orch-config.sh install      # Create namespaces and secrets
./pre-orch-config.sh uninstall    # Remove secrets and namespaces
```
