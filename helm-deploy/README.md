# Helm Deploy — ArgoCD-Free Orchestrator Deployment

Deploy Edge Orchestrator helm charts **without ArgoCD**, using the same chart sources and configuration profiles that ArgoCD uses.

## Overview

This tooling provides a two-step workflow:

```
┌──────────────────────┐      ┌──────────────────────┐
│  generate-values.sh  │ ───▶ │  helm-deploy-orch.sh │
│                      │      │                      │
│  Merges profiles +   │      │  Reads ArgoCD app    │
│  configs + templates │      │  templates, extracts │
│  into per-app values │      │  chart info, runs    │
│  override files      │      │  helm upgrade        │
└──────────────────────┘      └──────────────────────┘
```

**Step 1:** `generate-values.sh` produces `values_overwrite/values_<app>.yaml` for each application.
**Step 2:** `helm-deploy-orch.sh` reads `argocd/applications/templates/<app>.yaml` to extract chart coordinates and deploys using `helm upgrade --install`.

---

## Prerequisites

| Tool     | Version    | Purpose                          |
|----------|------------|----------------------------------|
| `helm`   | v3.11+     | Chart installation               |
| `yq`     | v4.x       | YAML merging (generate-values)   |
| `kubectl`| any        | Namespace creation, pod checks   |
| `grep`   | GNU        | Template parsing (`-oP` flag)    |

Ensure your `KUBECONFIG` points to the target cluster.

---

## File Structure

```
helm-deploy/
├── generate-values.sh          # Step 1: Generate per-app values files
├── helm-deploy-orch.sh         # Step 2: Deploy/uninstall charts
├── onprem.env                  # Environment config (IPs, domain, creds)
├── onprem-eim.yaml             # Profile-specific base values (EIM)
├── onprem-vpro.yaml            # Profile-specific base values (vPro)
├── application-list-vpro       # Ordered list of apps for vPro profile
├── application-list-eim        # Ordered list of apps for EIM profile
├── merge.yaml                  # Cached merged profile (auto-generated)
└── values_overwrite/           # Generated values files (one per app)
    ├── values_keycloak-operator.yaml
    ├── values_web-ui-root.yaml
    └── ...
```

---

## Step 1: generate-values.sh

### What It Does

Generates `values_overwrite/values_<app>.yaml` for each application by replicating ArgoCD's values resolution:

```
Profile YAMLs          ArgoCD configs/         ArgoCD custom/
(enable-*.yaml,    +   <app>.yaml          +   <app>.tpl
 profile-*.yaml)       (base config)           (Go template)
       │                    │                       │
       ▼                    ▼                       ▼
   ┌───────────────────────────────────────────────────┐
   │              yq merge + helm template             │
   │                                                   │
   │  1. Merge all profile YAMLs → merge.yaml          │
   │  2. For each app:                                 │
   │     a. Merge configs/<app>.yaml + merge.yaml      │
   │     b. Render custom/<app>.tpl with helm template │
   │     c. Merge base config + rendered template      │
   │     d. Write → values_overwrite/values_<app>.yaml │
   └───────────────────────────────────────────────────┘
```

### Profiles

Set via `ORCH_INSTALLER_PROFILE` (in `onprem.env` or environment):

| Profile        | App List File            | Description              |
|----------------|--------------------------|--------------------------|
| `onprem-vpro`  | `application-list-vpro`  | vPro-enabled deployment  |
| `onprem-eim`   | `application-list-eim`   | Edge Infra Manager only  |

### Usage

```bash
# Generate values for ALL apps in the profile's app list
./generate-values.sh

# Generate values for specific apps only
./generate-values.sh vault traefik web-ui-root

# Use a different profile
ORCH_INSTALLER_PROFILE=onprem-eim ./generate-values.sh
```

### Post-Processing

After generating values, the script automatically:

1. **Fixes nil top-level keys** — Converts bare `key:` (null) entries to `key: {}` to prevent Helm template errors.
2. **Handles Istio sidecar annotations** — If Istio is in the app list, ensures Traefik has `excludeInboundPorts` annotations. If Istio is absent, injects `sidecar.istio.io/inject: "false"` into Traefik values.

### Caching

The merged profile file (`merge.yaml`) is cached. Delete it to force regeneration:

```bash
rm merge.yaml
./generate-values.sh
```

---

## Step 2: helm-deploy-orch.sh

### What It Does

Reads ArgoCD application templates to extract chart coordinates (repo, chart name, version, namespace, releaseName) and runs native `helm upgrade --install`.

```
argocd/applications/templates/<app>.yaml
       │
       ▼  parse_template()
  ┌──────────────────────────┐
  │ repoURL  → OCI/repo URL  │
  │ chart    → chart path     │
  │ version  → targetRevision │
  │ namespace → destination   │
  │ releaseName → release     │
  └──────────────────────────┘
       │
       ▼
  helm upgrade --install <release> <chart> \
    --version <version> \
    --namespace <namespace> \
    -f values_overwrite/values_<app>.yaml \
    --wait --timeout 5m
```

### Usage

```bash
# Install specific apps
./helm-deploy-orch.sh install vault traefik web-ui-root web-ui-admin web-ui-infra

# Install all apps from the app list file
./helm-deploy-orch.sh install

# Uninstall specific apps
./helm-deploy-orch.sh uninstall web-ui-root web-ui-admin web-ui-infra

# Uninstall all (reverse order)
./helm-deploy-orch.sh uninstall

# Show what helm commands would run (no changes)
./helm-deploy-orch.sh dry-run vault traefik

# List all apps with chart info and values status
./helm-deploy-orch.sh list
```

### Chart Resolution

The script handles three chart source types:

| Source Pattern             | Type     | Resolution                                        |
|----------------------------|----------|---------------------------------------------------|
| `chartRepoURL` / `rsChartRepoURL` | OCI    | `oci://<RELEASE_SERVICE_URL>/<chart>`        |
| `ghcr.io` / `gcr.io`      | OCI-ext  | `oci://<repoURL>/<chart>`                        |
| `https://...`              | Repo     | `helm repo add <alias> <url>` + `<alias>/<chart>` |

### Key Behaviors

**Install ordering** — Apps ending in `*-root` (e.g. `web-ui-root`) are automatically moved to the end of the install queue. This ensures sub-charts that provide upstream services (e.g. `web-ui-admin`, `web-ui-infra`) are deployed first.

**Shared releaseName handling** — Some charts share a `releaseName` in ArgoCD (e.g. all `web-ui-*` charts use `releaseName: web-ui`). The script handles this by:
- Using the shared name (`web-ui`) as the Helm release for `*-root` charts (which reference `{{ .Release.Name }}` in Go templates for cross-service URLs).
- Using the app name as the Helm release for sub-charts (their `fullname` helpers detect the suffix).
- Auto-generating a temporary values override for sub-charts whose default values contain `{{ .Release.Name }}` references.

**Stuck release cleanup** — Before installing, detects releases in `pending-install`, `pending-upgrade`, `pending-rollback`, or `failed` state and automatically cleans them up.

**Pre-install hooks** — App-specific cleanup for:
- `vault` — Removes stale `vault-keys` secret and truncates PostgreSQL tables for clean reinit.
- `orch-utils` — Removes stale `vault-keys` secret.

### Environment Variables

| Variable              | Default                                                  | Description                    |
|-----------------------|----------------------------------------------------------|--------------------------------|
| `RELEASE_SERVICE_URL` | `registry-rs.edgeorchestration.intel.com/edge-orch`      | OCI registry base URL          |

---

## Quick Start

```bash
cd helm-deploy/

# 1. Configure environment
cp onprem.env.example onprem.env    # Edit with your cluster IPs, domain, etc.
vi onprem.env

# 2. Generate values files for all apps
./generate-values.sh

# 3. Review what will be deployed
./helm-deploy-orch.sh list
./helm-deploy-orch.sh dry-run

# 4. Deploy everything
./helm-deploy-orch.sh install

# Or deploy specific apps
./helm-deploy-orch.sh install metadata-broker web-ui-root web-ui-admin web-ui-infra
```

### Redeploying a Single App

```bash
# Regenerate values for one app, then redeploy
./generate-values.sh traefik
./helm-deploy-orch.sh install traefik
```

### Full Teardown

```bash
./helm-deploy-orch.sh uninstall
```

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `host not found in upstream "web-ui-*-infra"` | Wrong release name → wrong service names in nginx config | Ensure `web-ui-root` is installed **after** `web-ui-admin`/`web-ui-infra` (automatic with script ordering) |
| `another operation (install/upgrade/rollback) is in progress` | Stuck Helm release | Script auto-cleans; or manually: `helm uninstall <release> -n <ns>` |
| `UPGRADE FAILED: rendered manifests contain a resource that already exists` | Orphaned resources from previous install method | Delete the conflicting resource manually, then retry |
| Pod `CrashLoopBackOff` after fresh install | Upstream services not ready when root app starts | Script installs `*-root` last; if still crashing, wait for sub-chart pods to be Ready then reinstall root |
| `yq v4 is required` | Wrong yq version | Install yq v4: `go install github.com/mikefarah/yq/v4@latest` |
