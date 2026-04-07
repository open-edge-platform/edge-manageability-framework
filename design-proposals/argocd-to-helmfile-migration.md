# ADR: Migrating from ArgoCD to Helmfile for EMF Deployment

Author(s): Sunil Parida

Last updated: 2026-03-30

## Abstract

This proposal outlines the migration of EMF deployment from ArgoCD to Helmfile.
The primary objective is to eliminate ArgoCD as a cluster dependency and adopt a
lightweight, CLI-based approach, enabling EMF to be deployed on any Kubernetes
cluster without requiring ArgoCD.

The initial focus will be on the eim and vpro deployment profiles for on-premises
environments. Once this phase is completed, the migration will be extended to
include AO, CO, and observability-related Helm components.

## Problem Statement

Right now, our deployment approach depends on ArgoCD running alongside the
orchestrator. While it helps with automation, it also brings in extra components
(like the ArgoCD server, controller, and Redis) that we need to install and
maintain just to deploy our software. This creates an added dependency, making
it harder to deploy EMF in environments where ArgoCD isn't already set up

1. **Keep it lightweight.** ArgoCD adds multiple components to every cluster just to manage deployments.
   We want to remove that overhead and use a simple CLI-based tool instead, so the cluster only runs
   what actually matters — our own services.

2. **Deploy anywhere without extra dependencies.** Right now, if a cluster doesn't have ArgoCD set up, we
   can't deploy EMF on it. That's a hard blocker. Helmfile only needs `helm`, `helmfile`, and a kubeconfig.
   That means we can deploy on any cluster — on-prem or air-gapped — without needing to
   install anything special first.

## Decision

Replace ArgoCD with [**Helmfile**](https://github.com/helmfile/helmfile) as the primary deployment orchestrator
for all EMF environments. Helmfile is a declarative wrapper around Helm that lets you define all your releases
in one YAML file, manage per-environment overrides, and run install/diff/destroy across many charts in one command.

The Helmfile-based deployment will be implemented under `helmfile-deploy/` with the following design:

### Directory structure

```text
helmfile-deploy/
├── helmfile.yaml                  # Main helmfile with all releases
├── helmfile-deploy.sh             # Wrapper script (install/uninstall/list/diff)
├── onprem.env                     # Environment variables for on-prem
├── pre-deploy-config.sh           # Pre-deployment configuration
├── functions.sh                   # Shared helper functions
├── environments/                  # Profile-specific configs
│   ├── defaults-onprem.yaml.gotmpl       # Shared on-prem defaults
│   └── onprem-eim.yaml.gotmpl            # EIM profile
├── values/                        # Helm values per release
│   ├── traefik.yaml.gotmpl        # .gotmpl = uses env vars
│   └── ...
```

### Single declarative file

All releases are defined in a single `helmfile.yaml` with explicit `needs:` dependency declarations and
`condition:` toggles driven by environment values. This replaces the scattered ArgoCD Application templates.

### Environment profiles as composable value layers

Each deployment profile (on-prem, EIM, etc.) is a pair of `.yaml.gotmpl` files under `environments/`:
a shared defaults file and a profile-specific overlay. Adding a new profile requires only a new overlay file and
a helmfile environment entry — no template authoring.

Supported deploy types:

| Profile | What's included |
| --- | --- |
| `eim` | Platform + Edge Infrastructure Manager |
| `vpro` | Platform + vPro management |
| `eim-co` | EIM + Cluster Orchestration |
| `eim-co-ao` | EIM + Cluster Orch + App Orchestration |
| `eim-co-ao-o11y` | EIM + Cluster Orch + App Orch + Observability |

### Label-based targeting

Every release carries an `app:` label enabling operators to install, uninstall, diff, or sync individual charts:

```bash
helmfile -e onprem -l app=traefik sync       # install one chart
helmfile -e onprem -l app=traefik destroy     # remove one chart
helmfile -e onprem -l app=traefik diff        # preview changes
```

### Dependency ordering via `needs:`

Helmfile's `needs:` directive replaces ArgoCD sync wave annotations with explicit, auditable dependency edges.
Waves are preserved as comments for readability, but ordering is enforced structurally.

### Wrapper script

`helmfile-deploy.sh` will be the main entry point for deployment. It supports the following operations:

```bash
./helmfile-deploy.sh install                  # Install all charts
./helmfile-deploy.sh install traefik          # Install a single chart
./helmfile-deploy.sh uninstall traefik        # Uninstall a single chart
./helmfile-deploy.sh uninstall                # Uninstall all charts
./helmfile-deploy.sh list                     # List all charts
./helmfile-deploy.sh diff                     # Preview changes
```

## Consequences

### Benefits

- **No extra infrastructure needed.** ArgoCD requires its own server, controller,
  and Redis running in the cluster. Helmfile is just a CLI tool — nothing to
  install or maintain inside the cluster.
- **Install, update, or remove individual charts easily.** With labels, you can
  target a single chart (`helmfile -l app=traefik sync`) instead of dealing
  with the full ArgoCD sync process.
- **Preview changes before applying.** `helmfile diff` shows exactly what will
  change before you run anything — something ArgoCD doesn't offer out of
  the box.

### Risks and Mitigations

- **No automatic drift detection.** ArgoCD continuously watches for changes and
  self-heals. With Helmfile, we lose that. But in our current setup, ArgoCD
  runs on the same cluster where we deploy the orchestrator — it's not a
  separate server. So if the cluster goes down, ArgoCD goes down with it
  anyway. For our use case this is fine — our clusters are operator-managed
  and we do planned upgrades, not continuous delivery.
- **No ArgoCD UI.** Teams used to the ArgoCD dashboard will need to switch to
  `helmfile list` and `helm status` commands. Wrapper scripts and README docs
  help bridge that gap.
- **Both systems will coexist during migration.** We'll keep the `argocd/`
  directories intact until all clusters are moved over, so there's a clear
  rollback path.

## Affected Components and Teams

- **Platform Installer Team** — Owns the migration. Updates CI pipelines to replace ArgoCD sync with `helmfile sync`.
- **All service teams** — No chart changes required. Helmfile consumes the same Helm charts as ArgoCD.
- **VIP / HIP** — Migration of updated changes to the validation team. Update deployment automation scripts to use `helmfile-deploy.sh`.
- **Documentation** — Update deployment guides to reference Helmfile commands and profiles.

## Implementation Plan

1. **Helmfile setup**: Create `helmfile-deploy/` with `helmfile.yaml`, environment configs,
   value files, and wrapper scripts. Focus on `eim` and `vpro` profiles.
2. **On-prem validation**: Deploy `eim` and `vpro` profiles using Helmfile on on-prem clusters. Verify all
   charts install correctly and services come up healthy.
3. **VIP/HIP handoff**: Migrate updated deployment scripts to the validation team. Update VIP/HIP automation
   to use `helmfile-deploy.sh` for `eim` and `vpro` profiles.

## Open Issues

TBD
