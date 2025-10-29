# Keycloak Operator Architecture - Complete Overview

## Executive Summary

The Keycloak migration from Bitnami Helm chart to Keycloak Operator uses a **two-tier ArgoCD Application model**:

1. **keycloak-operator** Application - Deploys the Keycloak Operator CRD and controller
2. **platform-keycloak** Application - Deploys the Keycloak instance via the Keycloak CRD

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         root-app (sync wave 1)                  │
│           Renders all Applications from templates/               │
└────────────────────────────────┬────────────────────────────────┘
                                 │
                    ┌────────────┴────────────┐
                    │                         │
        ┌───────────▼────────────┐  ┌────────▼──────────────┐
        │ keycloak-operator      │  │ platform-keycloak    │
        │ (sync wave 1)          │  │ (sync wave 150)      │
        │                        │  │                      │
        │ Deploys:               │  │ Deploys:             │
        │ - Operator CRD         │  │ - Keycloak CRD       │
        │ - RBAC                 │  │ - RealmImport CRD    │
        │ - Webhooks             │  │ - Service            │
        │                        │  │ - RBAC               │
        └────────────────────────┘  └────────────────────────┘
              ▲                               ▲
              │                               │
              │ From external repo:           │ From git repo:
              │ github.com/keycloak/          │ edge-manageability-framework/
              │ keycloak-k8s-resources        │ argocd/keycloak-operator/
              │                               │
              │ Operator ready ──────────────►│
              │                               │
              └───────────────────────────────┘
```

## File Organization

```
argocd/
├── applications/
│   ├── templates/
│   │   ├── keycloak-operator.yaml          # Application definition for Operator
│   │   └── platform-keycloak.yaml          # Application definition for Keycloak instance
│   │
│   └── custom/
│       └── platform-keycloak.tpl           # Backup of cluster-specific config (legacy)
│
└── keycloak-operator/                      # Helm chart for Keycloak Operator deployment
    ├── Chart.yaml                          # Helm chart metadata
    ├── values.yaml                         # Value schema and documentation
    └── templates/
        ├── keycloak.yaml                   # Keycloak CRD instance
        ├── realm-import.yaml               # RealmImport CRD with realm configuration
        ├── service.yaml                    # Kubernetes Service
        └── rbac-config.yaml                # RBAC configuration
```

## Application Definitions

### 1. keycloak-operator Application

**File**: `argocd/applications/templates/keycloak-operator.yaml`

- **Sync Wave**: 1 (early deployment)
- **Namespace**: keycloak-system
- **Source**: External GitHub repository
  - `https://github.com/keycloak/keycloak-k8s-resources`
  - Path: `kubernetes`
  - Pattern: `keycloak-operator-*.yaml`
- **Purpose**: Install the Keycloak Operator and its CRDs

**This must be deployed FIRST** before platform-keycloak can run.

### 2. platform-keycloak Application

**File**: `argocd/applications/templates/platform-keycloak.yaml`

- **Sync Wave**: 150 (after Operator is ready)
- **Namespace**: orch-platform
- **Source**: Internal git repository (Helm chart)
  - `<deployRepoURL>/argocd/keycloak-operator`
  - Processed as Helm chart
- **Values**: Passed via `valuesObject` section in Application spec
- **Purpose**: Deploy Keycloak instance using the Operator

## How Cluster-Specific Values Are Resolved

### Flow

```
root-app (Helm template processing)
    │
    ├─ Input: .Values.argo.clusterDomain (from profiles)
    │
    ├─ Renders: argocd/applications/templates/platform-keycloak.yaml
    │
    └─ Creates: Application CRD with embedded valuesObject
            │
            ├─ clusterSpecific.webuiClientRootUrl 
            │  = "https://web-ui.{{ clusterDomain }}"
            │
            ├─ clusterSpecific.telemetryClientRootUrl
            │  = "https://observability-ui.{{ clusterDomain }}"
            │
            ├─ clusterSpecific.*RedirectUrls
            │  = Array of resolved URLs
            │
            └─ ... (other config values)
```

### Value Propagation

1. **Profile configuration** (e.g., `profiles/profile-dev.yaml`)
   ```yaml
   argo:
     clusterDomain: "orch-10-139-218-125.pid.infra-host.com"
     proxy:
       httpsProxy: "http://proxy-dmz.intel.com:912"
       httpProxy: "http://proxy-dmz.intel.com:912"
   ```

2. **root-app renders** `platform-keycloak.yaml`
   - Resolves `{{ .Values.argo.clusterDomain }}` 
   - Creates Application with concrete values

3. **ArgoCD applies** the Application
   - Helm processes `argocd/keycloak-operator/Chart.yaml`
   - Uses the `valuesObject` from the Application
   - Renders `templates/keycloak.yaml` and `realm-import.yaml`

4. **Keycloak Operator reconciles** the manifests
   - Creates Keycloak pod with correct configuration
   - Syncs realm via RealmImport CRD

## Helm Chart Structure: keycloak-operator

### Chart.yaml
```yaml
apiVersion: v2
name: keycloak-operator
description: Keycloak Operator deployment with realm configuration
type: application
version: 1.0.0
appVersion: "25.0.0"
```

### values.yaml
- **Purpose**: Schema documentation and defaults
- **NOT merged** with Application valuesObject
- Provides default values if no valuesObject is passed
- Serves as a reference for what values the templates expect

### templates/

**keycloak.yaml**
- Creates: `Keycloak` CRD instance
- Uses values:
  - `keycloak.instances` (replica count)
  - `database.*` (PostgreSQL connection)
  - `http.*` (port configuration)
  
**realm-import.yaml**
- Creates: `RealmImport` CRD for realm configuration
- **Contains Helm template variables** for cluster-specific URLs:
  ```yaml
  rootUrl: {{ .Values.clusterSpecific.webuiClientRootUrl | toJson }}
  redirectUris: {{ .Values.clusterSpecific.webuiRedirectUrls | toJson }}
  ```

**service.yaml**
- Creates: Kubernetes Service for Keycloak
- Uses labels from `keycloak` values

**rbac-config.yaml**
- Creates: ServiceAccount, Role, RoleBinding for Keycloak pod

## The Key Insight: Why Two Applications?

### Why separate keycloak-operator from platform-keycloak?

The **Keycloak Operator CRD** is the infrastructure dependency:

```
Sync Wave 1:  keycloak-operator Application
  ├─ Install keycloak.org/Keycloak CRD
  ├─ Install keycloak.org/RealmImport CRD  
  ├─ Install operator controller
  └─ Operator is ready to reconcile

Sync Wave 150: platform-keycloak Application
  └─ Can now safely create Keycloak resources (CRD instances)
```

If we tried to deploy both in the same Application:
- ❌ RealmImport CRD might not exist yet
- ❌ Keycloak manifest might fail to apply
- ❌ Race condition between operator and instances

## Answer to Your Questions

### Q1: Is the mergeOverride logic in platform-keycloak.yaml?

**A: No, there is no merge logic.**

The `valuesObject` section in `platform-keycloak.yaml` is a **direct embedding** of all the values that will be passed to Helm. The old `mergeOverwrite` logic from the backup file is NOT used.

Instead:
- All cluster-specific values are **resolved at root-app render time**
- The complete `valuesObject` is embedded in the Application spec
- Helm receives the already-resolved values

### Q2: Are both keycloak-operator.yaml and platform-keycloak.yaml needed?

**A: YES, both are essential.**

They serve different purposes:
- `keycloak-operator.yaml` → Install the Operator (sync wave 1)
- `platform-keycloak.yaml` → Deploy Keycloak instance (sync wave 150)

Without the Operator, there's nowhere to deploy the CRD instance.
Without the platform-keycloak Application, the instance never gets created.

### Q3: Where is the Keycloak CRD getting installed?

**A: In three places:**

1. **Keycloak CRD** (apiVersion: `keycloak.org/v2alpha1`)
   - Installed by: `keycloak-operator` Application
   - Source: `github.com/keycloak/keycloak-k8s-resources/kubernetes`

2. **RealmImport CRD** (apiVersion: `keycloak.org/v2alpha1`)
   - Installed by: `keycloak-operator` Application (same source)
   - Used by: `realm-import.yaml` in `platform-keycloak` Application

3. **Keycloak Instance** (kind: `Keycloak`, apiVersion: `keycloak.org/v2alpha1`)
   - Created by: `platform-keycloak` Application
   - File: `argocd/keycloak-operator/templates/keycloak.yaml`
   - Triggers: Keycloak Operator to create the actual pod

## Deployment Sequence

```
Time 0: root-app syncs
  └─ Renders all Application templates

Time 1: root-app applies resources
  ├─ Creates keycloak-operator Application (sync wave 1)
  ├─ Creates platform-keycloak Application (sync wave 150)
  ├─ Creates 50+ other Applications
  └─ root-app stays in sync state

Time 5-10: keycloak-operator syncs
  ├─ ArgoCD pulls from github.com/keycloak/keycloak-k8s-resources
  ├─ Applies keycloak-operator-*.yaml files
  ├─ Installs CRDs (Keycloak, RealmImport, etc.)
  ├─ Installs operator controller
  └─ keycloak-operator Application shows: Synced/Healthy

Time 15-20: platform-keycloak syncs (waits for keycloak-operator sync wave)
  ├─ ArgoCD processes platform-keycloak Application
  ├─ Helm processes argocd/keycloak-operator/Chart.yaml
  ├─ Uses valuesObject with resolved cluster-specific URLs
  ├─ Renders keycloak.yaml with Keycloak CRD instance
  ├─ Renders realm-import.yaml with resolved URLs
  ├─ Applies to orch-platform namespace
  └─ platform-keycloak Application shows: Synced/Healthy

Time 20-30: Keycloak Operator reconciles
  ├─ Keycloak controller sees new Keycloak CRD
  ├─ Creates StatefulSet for Keycloak pod
  ├─ Creates supporting resources (PVC, ConfigMaps, etc.)
  ├─ Keycloak pod initializes database
  └─ Keycloak becomes ready

Time 30-40: RealmImport reconciliation
  └─ RealmImport controller creates realm, clients, groups, users
      with the resolved cluster-specific URLs
```

## Troubleshooting

### If platform-keycloak shows "OutOfSync/Missing"
- Check keycloak-operator Application status first
- Ensure Keycloak Operator is fully deployed
- Check if Chart.yaml exists in argocd/keycloak-operator

### If platform-keycloak shows error about Chart.yaml
- Verify structure: `argocd/keycloak-operator/Chart.yaml` exists
- Verify Helm chart structure is correct (Chart.yaml, values.yaml, templates/)
- Check git repo is up to date: `git status` should be clean

### If Keycloak pod doesn't start
- Check platform-keycloak Application status
- Verify realm-import.yaml is applied: `kubectl get realmimport -n orch-platform`
- Check Keycloak pod logs: `kubectl logs -n orch-platform keycloak-0`

## Related Files

- Migration guide: `KEYCLOAK_MIGRATION_COMPLETE.md`
- Status report: `KEYCLOAK_STATUS_REPORT.md`
- Before/after comparison: `BEFORE_AFTER_COMPARISON.md`
- Template resolution guide: `TEMPLATE_VARIABLE_RESOLUTION_GUIDE.md`

---

**Last Updated**: 2025-10-17  
**Status**: ✅ Complete and tested
