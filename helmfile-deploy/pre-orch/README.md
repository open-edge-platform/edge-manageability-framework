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
| `LOCALPV_VERSION` | `4.3.0` | OpenEBS LocalPV chart version |
| `EMF_TRAEFIK_IP` | — | Traefik LoadBalancer IP (required for MetalLB) |
| `EMF_HAPROXY_IP` | — | HAProxy LoadBalancer IP (required for MetalLB) |

## Skipping Components During Install

Use `--no-openebs` and/or `--no-metallb` to skip components during cluster setup:

```bash
# Install cluster only (no OpenEBS, no MetalLB)
./pre-orch.sh k3s install --no-openebs --no-metallb

# Install cluster + OpenEBS only (no MetalLB)
./pre-orch.sh k3s install --no-metallb

# Install cluster + MetalLB only (no OpenEBS)
./pre-orch.sh k3s install --no-openebs
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

```
pre-orch/
├── pre-orch.sh              # Main installer script
├── pre-orch.env             # Configuration file
├── helmfile.yaml.gotmpl     # Combined helmfile (OpenEBS + MetalLB)
├── README.md
├── openebs-localpv/         # OpenEBS LocalPV values
└── metallb/                 # MetalLB values + local config chart
```
