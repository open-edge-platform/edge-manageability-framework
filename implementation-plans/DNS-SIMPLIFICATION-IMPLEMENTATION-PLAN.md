# DNS Simplification Implementation Plan

**Author:** Scott Baker (with AI-assisted research)
**Date:** 2026-04-27
**Target Release:** 2026.2
**Design Proposal:** [platform-dns-simplification.md](edge-manageability-framework/design-proposals/platform-dns-simplification.md)

---

## Executive Summary

This plan details the concrete work required to consolidate ~35 subdomains into a single
domain with path-based routing. The research covers all **29 repositories** in the
workspace and identifies every file that references external hostnames, subdomain
derivation logic, IngressRoute definitions, Keycloak client configuration, CORS/CSP
policies, and edge node provisioning templates.

The total scope is approximately **~145 files** across **17 repositories** requiring
changes. The remaining 12 repositories require no runtime changes (some have test
fixtures or tooling references to clean up — see Section 2). Estimated effort is
**~13–16 engineer-days AI-assisted** or **~25–30 engineer-days manual**, plus 4–6 days
of E2E validation.

A **dual-routing migration strategy** (Section 16) allows this work to land incrementally:
path-based routes are added alongside existing subdomain routes, so each phase can merge
independently without breaking deployed systems. Existing edge nodes continue using
subdomain configuration while newly onboarded nodes receive single-domain config. This
means there is no big-bang cutover — teams can ship phases in parallel and the system
remains functional throughout.

This document covers migration of the entire stack, including Application Orchestration,
Cluster Orchestration, and Observability (Phases 7–9). Those components are not enabled
in the current helmfile EIM profile and may be deployed separately or added later —
time can be reduced by deferring them.

---

## Table of Contents

1. [Repositories Requiring Changes](#1-repositories-requiring-changes)
2. [Repositories Requiring No Runtime Changes](#2-repositories-requiring-no-runtime-changes)
3. [Phase 1: Traefik Routing (orch-utils, infra-charts, EMF)](#3-phase-1-traefik-routing)
4. [Phase 2: Keycloak Realm Configuration (EMF)](#4-phase-2-keycloak-realm-configuration)
5. [Phase 3: Edge Node Provisioning (EMF, infra-onboarding)](#5-phase-3-edge-node-provisioning)
6. [Phase 4: Edge Node Agents (edge-node-agents)](#6-phase-4-edge-node-agents)
7. [Phase 5: Web UI (orch-ui)](#7-phase-5-web-ui)
8. [Phase 6: CLI (orch-cli)](#8-phase-6-cli)
9. [Phase 7: Cluster Services — *not in helmfile EIM profile*](#9-phase-7-cluster-services)
10. [Phase 8: Application Orchestration — *not in helmfile EIM profile*](#10-phase-8-application-orchestration)
11. [Phase 9: Observability — *not in helmfile EIM profile*](#11-phase-9-observability)
12. [Phase 10: Installer and Documentation](#12-phase-10-installer-and-documentation)
13. [Subdomain-to-Path Mapping (Complete)](#13-subdomain-to-path-mapping-complete)
14. [gRPC Service Path Uniqueness Verification](#14-grpc-service-path-uniqueness-verification)
15. [Dangerous Patterns: Subdomain Derivation by String Replacement](#15-dangerous-patterns)
16. [Migration Strategy: Dual-Routing Support](#16-migration-strategy-dual-routing-support)
17. [Risks and Open Issues](#17-risks-and-open-issues)
18. [Effort Summary](#18-effort-summary)
19. [Repository Audit Status](#19-repository-audit-status)
20. [Appendix A: ArgoCD-Era References](#appendix-a-argocd-era-references-superseded-by-helmfile)

---

## 1. Repositories Requiring Changes

| Repository | Impact | Summary |
|---|---|---|
| **orch-utils** | **HIGH** | Central Traefik routing templates, CORS, CSP, Keycloak middleware |
| **edge-manageability-framework** | **HIGH** | Helmfile values templates (Traefik routing, Keycloak realm, infra-config, CSP/CORS), CI dnsmasq scripts |
| **infra-charts** | **HIGH** | Helm charts for all infra gRPC services (IngressRoutes + values) |
| **edge-node-agents** | **HIGH** | 8 agents with gRPC dial targets and config structs |
| **infra-onboarding** | **HIGH** | Cloud-init templates, onboarding hostname provisioning, subdomain derivation |
| **orch-ui** | **MEDIUM** | Runtime config, subdomain derivation in ExtensionHandler and VNC |
| **orch-cli** | **MEDIUM** | Keycloak endpoint derivation from API subdomain |
| **app-orch-deployment** | **MEDIUM** | 3 IngressRoutes, CORS, URL rewriting |
| **app-orch-catalog** | **LOW** | 1 IngressRoute, test data |
| **o11y-charts** | **LOW** | Grafana root_url, Keycloak OAuth redirect URIs |
| **cluster-connect-gateway** | **LOW** | Helm values only (already path-based) |
| **cluster-manager** | **LOW** | Helm values (clusterdomain reference) |
| **cluster-tests** | **LOW** | Hardcoded test URLs, dnsmasq wildcard script, CI workflow DNS setup |
| **o11y-alerting-monitor** | **LOW** | IngressRoute, Keycloak OIDC endpoint, test config |
| **cluster-api-provider-intel** | **LOW** | Southbound IngressRoute (Helm-driven), test keycloak.kind.internal refs |
| **virtual-edge-node** | **MEDIUM** | dnsmasq script with 40 subdomain entries, Keycloak/API URL construction in provisioning scripts |
| **edge-manage-test-automation** | **MEDIUM** | Robot Framework variables constructing keycloak/api/web-ui/tinkerbell subdomain URLs |

## 2. Repositories Requiring No Runtime Changes

These repos have **no runtime code** that constructs or routes on external subdomain
hostnames. Some may have test fixtures, README references, or hardcoded issuer URLs in
test code that reference subdomains — these are not blocking but should be cleaned up
as part of Phase 10.

| Repository | Reason | Test/Tooling Notes |
|---|---|---|
| **infra-core** | Internal K8s service DNS; gRPC routing already path-based | — |
| **infra-managers** | Backend gRPC services; no IngressRoutes (defined in infra-charts) | — |
| **infra-external** | Internal K8s service DNS only | — |
| **orch-library** | Shared auth/gRPC utilities; endpoints passed via env vars | — |
| **orch-metadata-broker** | No IngressRoute; ClusterIP services only | — |
| **app-orch-tenant-controller** | Internal only | `test/component/component_test.go` uses `harbor.kind.internal` (test fixture) |
| **o11y-tenant-controller** | Internal only | — |
| **cluster-extensions** | No hostname references | — |
| **trusted-compute** | No hostname references | — |
| **orch-ci** | CI tooling, not deployed | — |
| **scorch** | Installer tooling; only Intel-internal proxy/Docker cache config | — |
| **edge-manage-docs** | Documentation content — needs updating (covered in Phase 10) | — |

---

## 3. Phase 1: Traefik Routing

**Goal:** Add path-based routing rules alongside existing Host() rules.

This is the foundation — all other phases depend on Traefik being able to route traffic
by path on the single domain.

### 3.1 orch-utils/charts/traefik-extra-objects

This is the **single most important file** in the entire change. It defines the central
Traefik routing for platform services.

**File:** `orch-utils/charts/traefik-extra-objects/templates/traefik-extra-objects.yaml`

**Current IngressRoutes to modify (add PathPrefix alternatives):**

| IngressRoute Name | Current Match | Proposed Match (add) | Backend | Line |
|---|---|---|---|---|
| harbor-oci | `Host(registry-oci.DOMAIN)` | Keep as subdomain (exception) | harbor-oci-core:80 | 125 |
| edgenode-observability-grafana | `Host(observability-ui.DOMAIN)` | `Host(DOMAIN) && PathPrefix(/grafana/)` | edgenode-observability-grafana:80 | 170 |
| orch-platform-grafana | `Host(observability-admin.DOMAIN)` | `Host(DOMAIN) && PathPrefix(/grafana-admin/)` | orchestrator-observability-grafana:80 | 194 |
| orch-platform-vault | `Host(vault.DOMAIN)` | `Host(DOMAIN) && PathPrefix(/vault/)` | vault:8200 | 238 |
| orch-platform-keycloak | `Host(keycloak.DOMAIN)` | `Host(DOMAIN) && PathPrefix(/auth/)` | platform-keycloak:80 | 262 |
| orch-platform-logs-node | `Host(logs-node.DOMAIN)` | `Host(DOMAIN) && PathPrefix(/logs-node/)` | opentelemetry-collector:4318 | 584 |
| orch-platform-metrics-node | `Host(metrics-node.DOMAIN) && PathPrefix(...)` | `Host(DOMAIN) && PathPrefix(/metrics-node/)` + existing paths | mimir-gateway:8181 | 612 |
| svc-fleet-https | `Host(fleet.DOMAIN)` | `Host(DOMAIN) && PathPrefix(/fleet/)` | kubernetes:443 | 649 |
| ma-gitea | `Host(gitea.DOMAIN)` | `Host(DOMAIN) && PathPrefix(/gitea/)` | gitea-http:3000 | 308 |

**Values file:** `orch-utils/charts/traefik-extra-objects/values.yaml`

Add new values for path-based mode:

```yaml
# New: single-domain mode
singleDomain:
  enabled: false  # Phase 1: false. Phase 2: true for new deployments.
  rootHost: ""    # e.g., "example.com"
```

**CORS simplification** (lines 53–55 in values.yaml):
- Current: `allowedOrigins` lists `https://DOMAIN` and `https://web-ui.DOMAIN`
- New (single-domain): Only `https://DOMAIN` needed

**CSP simplification** (lines 24–35 in values.yaml):
- Current: Lists 8+ subdomain-specific URLs for `connectCSPs` and `scriptSources`
- New (single-domain): Collapse to `https://DOMAIN` for all

**Middleware: sniStrict** (line 42):
- `sniStrict: true` may reject requests if the SNI doesn't match the cert. With a
  single domain this simplifies, but during dual-routing (Phase 1) both the wildcard
  cert and the single-domain cert must be present.

### 3.2 infra-charts (gRPC IngressRoutes)

Each infra service has its own Helm chart with an IngressRoute. These currently use
`Host()` matching. For single-domain gRPC routing, they need `PathPrefix()` matching
on the gRPC service path.

| Chart | File | Current Host | gRPC Service Path | Port |
|---|---|---|---|---|
| host-manager | `templates/service.yaml:36` | `infra-node.DOMAIN` | `/hostmgr_southbound_proto.Hostmgr/` | 50001 |
| maintenance-manager | `templates/service.yaml:36` | `update-node.DOMAIN` | `/maintmgr.v1.MaintmgrService/` | 50002 |
| telemetry-manager | `templates/service.yaml:36` | `telemetry-node.DOMAIN` | `/telemetrymgr.v1.TelemetryMgr/` | 50004 |
| attestationstatus-manager | `templates/service.yaml:37` | `attest-node.DOMAIN` | `/attestmgr.v1.AttestationStatusMgrService/` | 50007 |
| onboarding-manager | `templates/service.yaml:42,73` | `onboarding-node.DOMAIN`, `onboarding-stream.DOMAIN` | `/onboardingmgr.v1.InteractiveOnboardingService/`, `/onboardingmgr.v1.NonInteractiveOnboardingService/` | 50054, 50055 |
| amt/dm-manager | `templates/service.yaml:36` | `device-manager-node.DOMAIN` | `/device_management.DeviceManagement/` | 50058 |
| amt/rps | `templates/ingress.yaml:31,40` | `rps.DOMAIN`, `rps-wss.DOMAIN` | `/rps/` (WebSocket) | 8080, 8081 |
| amt/mps | `templates/ingress.yaml:17` | `mps-wss.DOMAIN` | `/mps/` (WebSocket) | 3000 |
| apiv2 | `templates/ingress.yaml:53,67` | `api.DOMAIN` | `/v1/projects/...` (already path-based) | 8082 |
| tinkerbell | `templates/ingress.yaml:14` | `tinkerbell-server.DOMAIN` | Keep as special case (PXE boot) | 42113 |

**Pattern for each chart:**

Add a **second route** in each `service.yaml` / `ingress.yaml` (not if/else — both
routes must coexist for dual-routing migration):

```yaml
  routes:
    # Existing subdomain route — keep during migration, remove in Phase 3 (deprecation)
    - match: Host(`{{ .Values.traefikReverseProxy.host.grpc.name }}`)
      ...
    # New path-based route — added in Phase 1
    {{- if .Values.singleDomain.enabled }}
    - match: Host(`{{ .Values.singleDomain.rootHost }}`) && PathPrefix(`{{ .Values.singleDomain.grpcPathPrefix }}`)
      ...
    {{- end }}
```

This is critical: the `singleDomain.enabled` flag controls whether the **additional**
path-based route is generated. The existing subdomain route is always present until
the deprecation phase. This ensures both old and new clients work simultaneously.

### 3.3 Certificate SAN List (orch-utils, EMF)

**File:** `orch-utils/charts/self-signed-cert/templates/self-signed-certs.yaml:145-196`

The self-signed certificate currently uses a wildcard SAN (`*.DOMAIN`) at line 147, which
covers all subdomains automatically. However, the same file also generates a ConfigMap
(`kubernetes-docker-internal`, lines 160-196) that **enumerates every subdomain** as
individual dnsNames entries. This ConfigMap is consumed by the autocert flow for real
certificate generation in production/on-prem.

**Impact:** Under single-domain mode, the SAN list simplifies to just `DOMAIN` (plus
exceptions like `registry-oci.DOMAIN`, `tinkerbell-haproxy.DOMAIN`). The wildcard
`*.DOMAIN` can be dropped entirely for non-dev deployments.

**Files to modify:**

| File | Lines | What |
|---|---|---|
| `orch-utils/charts/self-signed-cert/templates/self-signed-certs.yaml` | 145–147 | Self-signed cert dnsNames (wildcard) |
| `orch-utils/charts/self-signed-cert/templates/self-signed-certs.yaml` | 160–196 | ConfigMap enumerating all 35 subdomain SANs |
| `edge-manageability-framework/helmfile-deploy/post-orch/values/self-signed-cert.yaml.gotmpl` | 12–13 | Passes `clusterDomain` and `certDomain` to the chart |
| `edge-manageability-framework/helmfile-deploy/post-orch/values/platform-autocert.yaml.gotmpl` | 5 | Production cert generation — must support both modes |

**During dual-routing (Phase 1):** The cert must cover both `*.DOMAIN` and `DOMAIN`
(the wildcard already covers subdomains; adding the bare domain is the only change).

**After deprecation:** Replace the 35-entry SAN list with `DOMAIN` plus the 3-4
exception subdomains.

### 3.4 Hardcoded CSP in iamUmbrellaMiddleware (orch-utils)

**File:** `orch-utils/charts/nexus-api-gw/templates/iamUmbrellaMiddleware.yaml:19-30`

This middleware contains a **hardcoded** Content-Security-Policy that embeds subdomain
URLs for API docs (`app-service-proxy.kind.internal`, `keycloak.kind.internal`,
`vnc.kind.internal`, `app-orch.kind.internal`, `api.kind.internal`,
`metadata.kind.internal`, `alerting-monitor.kind.internal`,
`orchestrator-license.kind.internal`).

These are baked into the template, not driven by Helm values. They need to be
**templatized** to support both subdomain and path-based modes, or collapsed to
`https://DOMAIN` under single-domain mode.

### 3.5 edge-manageability-framework (Helmfile values templates)

The helmfile-based deployment uses Go template values files in
`helmfile-deploy/post-orch/values/` that directly set Helm chart values. All subdomain
construction flows from a single variable: `.Values.clusterDomain` (sourced from
`EMF_CLUSTER_DOMAIN` env var via `onprem-eim-settings.yaml.gotmpl:21`).

**Files to modify:**

| File | Subdomains Constructed | Lines |
|---|---|---|
| `traefik-extra-objects.yaml.gotmpl` | fleet, registry-oci, observability-ui, observability-admin, vault, keycloak, cluster-orch-node, logs-node, metrics-node, gitea + CSP/CORS lists | 16–26, 30–41, 62–63 |
| `infra-managers.yaml.gotmpl` | infra-node, update-node, telemetry-node, attest-node | 39, 62, 95, 137 |
| `infra-onboarding.yaml.gotmpl` | All 17+ edge-node-facing hostnames (ConfigMap) | 78–79, 147–149, 218–241 |
| `infra-external.yaml.gotmpl` | mps, mps-wss, rps, rps-wss, device-manager-node | 97, 105, 107, 125, 127, 140 |
| `infra-core.yaml.gotmpl` | api | 88 |
| `web-ui-root.yaml.gotmpl` | keycloak, observability-ui, web-ui, root domain, api (×10) | 13, 18, 33–34, 40–49 |
| `web-ui-admin.yaml.gotmpl` | keycloak, api endpoints | 11, 14–19 |
| `web-ui-infra.yaml.gotmpl` | keycloak, observability-ui, api endpoints | 13, 18, 37–46 |
| `nexus-api-gw.yaml.gotmpl` | api | 18 |
| `metadata-broker.yaml.gotmpl` | *(none — only internal OIDC issuer and registry)* | — |
| `token-fs.yaml.gotmpl` | release | 9 |
| `certificate-file-server.yaml.gotmpl` | root domain | 5 |
| `component-status.yaml.gotmpl` | api | 23 |
| `platform-keycloak-realm.yaml.gotmpl` | All OAuth clients (web-ui, app-service-proxy, vnc, docs-ui, observability-*, cluster-management, registry-oci) | 476–665 |
| `platform-keycloak.yaml.gotmpl` | clusterDomain pass-through | 6 |
| `platform-autocert.yaml.gotmpl` | certDomain | 5 |
| `self-signed-cert.yaml.gotmpl` | clusterDomain, certDomain | 12–13 |
| `haproxy-ingress-pxe-boots.yaml.gotmpl` | tinkerbell-haproxy | 8 |
| `traefik.yaml.gotmpl` | tinkerbell-haproxy (single-IP mode) | 144 |

**Strategy:** Add a `singleDomainMode` flag to the helmfile environment settings
(`onprem-eim-settings.yaml.gotmpl`). When enabled, the values templates generate
path-based Host() + PathPrefix() matches **in addition to** existing subdomain
Host() matches (additive dual-routing). The `.Values.clusterDomain` value remains;
path-based routes are layered on top, and subdomain routes are removed only after full
migration.

### 3.6 Additional IngressRoutes in other repos

| Repo | File | Current Host | Notes |
|---|---|---|---|
| app-orch-catalog | `deployments/.../templates/ingressroute.yaml` | `app-orch.DOMAIN` | Already uses PathRegexp |
| app-orch-deployment/app-deployment-manager | `deployment/.../templates/ingressroute.yaml` | `api.DOMAIN` | Already uses PathRegexp |
| app-orch-deployment/app-resource-manager | `deployments/.../templates/ingressroute.yaml` | `api.DOMAIN` | Already uses PathRegexp |
| app-orch-deployment/app-service-proxy | `deployments/.../templates/service.yaml` | `app-service-proxy.DOMAIN` | Two routes (host + path) |
| cluster-connect-gateway | `deployment/.../templates/traefik-ingress.yaml` | `connect-gateway.DOMAIN` | Already uses PathPrefix |
| orch-utils/charts/nexus-api-gw | `templates/service.yaml` | `api.DOMAIN` | Already uses PathPrefix |
| orch-utils/charts/component-status | `templates/ingressroute.yaml` | `api.DOMAIN` | Already uses PathPrefix |

---

## 4. Phase 2: Keycloak Realm Configuration

**File:** `edge-manageability-framework/helmfile-deploy/post-orch/values/platform-keycloak-realm.yaml.gotmpl`

### 4.1 Keycloak Server Configuration

Set `http-relative-path` to `/auth` so Keycloak serves from `DOMAIN/auth/` instead of
`keycloak.DOMAIN/`.

### 4.2 OAuth Client Redirect URIs

Every Keycloak client needs updated redirect URIs. These are the clients found:

| Client ID | Current rootUrl | New rootUrl | Current redirectUris (pattern) | New redirectUris | Lines |
|---|---|---|---|---|---|
| `webui-client` | `https://web-ui.DOMAIN` | `https://DOMAIN` | `https://web-ui.DOMAIN/*`, `https://app-service-proxy.DOMAIN/*`, `https://vnc.DOMAIN/*`, `https://DOMAIN/*` | `https://DOMAIN/*` | 591–596 |
| `docsui-client` | `https://docs-ui.DOMAIN` | `https://DOMAIN/docs` | `https://docs-ui.DOMAIN/*` | `https://DOMAIN/docs/*` | 624–629 |
| `telemetry-client` | `https://observability-ui.DOMAIN` | `https://DOMAIN/grafana` | `https://observability-admin.DOMAIN/login/*`, `https://observability-ui.DOMAIN/login/*` | `https://DOMAIN/grafana/login/*`, `https://DOMAIN/grafana-admin/login/*` | 476–481 |
| `cluster-management-client` | `https://cluster-management.DOMAIN` | `https://DOMAIN/cluster-mgmt` | `https://cluster-management.DOMAIN/*` | `https://DOMAIN/cluster-mgmt/*` | 510–517 |
| `registry-client` | `https://registry-oci.DOMAIN` | `https://registry-oci.DOMAIN` (unchanged) | `/c/oidc/callback` | No change (Harbor stays as subdomain) | 660–665 |

**M2M clients** (alerts-m2m, host-manager-m2m, co-manager-m2m, ktc-m2m,
3rd-party-host-manager-m2m, edge-manager-m2m, en-m2m-template): These are service
accounts and don't use redirect URIs. **No changes needed.**

### 4.3 Keycloak OIDC Issuer URL

The internal OIDC issuer URL (`http://platform-keycloak.orch-platform.svc/realms/master`)
is used by all backend services for JWT validation. This is a Kubernetes-internal URL
and **does not change**.

The external-facing issuer URL changes from `https://keycloak.DOMAIN/realms/master` to
`https://DOMAIN/auth/realms/master`. This affects:
- Keycloak's own `frontendUrl` configuration
- Any service that constructs the external issuer URL (mainly orch-ui)

---

## 5. Phase 3: Edge Node Provisioning

### 5.1 infra-config ConfigMap

**File:** `edge-manageability-framework/helmfile-deploy/post-orch/values/infra-onboarding.yaml.gotmpl:218-241`

This ConfigMap is the **single source of truth** for what hostnames edge nodes use.
All 17+ entries change from subdomain format to single-domain format:

| Key | Current Value | New Value |
|---|---|---|
| `orchInfra` | `infra-node.DOMAIN:443` | `DOMAIN:443` |
| `orchCluster` | `cluster-orch-node.DOMAIN:443` | `DOMAIN:443` |
| `orchUpdate` | `update-node.DOMAIN:443` | `DOMAIN:443` |
| `orchRelease` | `release.DOMAIN` | `DOMAIN` |
| `orchPlatformObsLogs` | `logs-node.DOMAIN:443` | `DOMAIN:443` |
| `orchPlatformObsMetrics` | `metrics-node.DOMAIN:443` | `DOMAIN:443` |
| `orchKeycloak` | `keycloak.DOMAIN:443` | `DOMAIN:443` |
| `orchTelemetry` | `telemetry-node.DOMAIN:443` | `DOMAIN:443` |
| `orchAttestationStatus` | `attest-node.DOMAIN:443` | `DOMAIN:443` |
| `orchMPSHost` | `mps.DOMAIN:4433` | `DOMAIN:4433` (port differentiates) |
| `orchMPSWHost` | `mps-wss.DOMAIN:443` | `DOMAIN:443` |
| `orchRPSHost` | `rps.DOMAIN:443` | `DOMAIN:443` |
| `orchRPSWHost` | `rps-wss.DOMAIN:443` | `DOMAIN:443` |
| `orchDeviceManager` | `device-manager-node.DOMAIN:443` | `DOMAIN:443` |
| `tinkerSvc` | `tinkerbell-server.DOMAIN` | Keep as subdomain (PXE boot) |
| `provisioningSvc` | `tinkerbell-haproxy.DOMAIN` | Keep as subdomain (PXE boot) |
| `omSvc` | `onboarding-node.DOMAIN` | `DOMAIN` |
| `omStreamSvc` | `onboarding-stream.DOMAIN` | `DOMAIN` |

**Note:** Since many ConfigMap values collapse to the same `DOMAIN:443`, the gRPC
routing now depends on the gRPC service path rather than the hostname. This is the
correct behavior — gRPC requests include `/package.ServiceName/MethodName` in every
request, and Traefik routes on this path.

### 5.2 Cloud-Init Templates

**File:** `infra-onboarding/onboarding-manager/pkg/cloudinit/99_infra.cfg`

This template writes all orchestrator URLs to `/etc/edge-node/node/agent_variables` on
each edge node. The template variables come from the curation code.

**File:** `infra-onboarding/dkam/pkg/curation/curation.go` (lines 158–200)

This code maps `InfraConfig` fields into template variables. It uses `strings.Split`
to separate host and port. Under single-domain, many of these variables will have the
same host but the agents distinguish services by gRPC path, not by hostname.

### 5.3 Hook-OS Device Discovery

**File:** `infra-onboarding/hook-os/device_discovery/device-discovery.go`

The hook-OS device discovery process reads `onboarding_manager_svc`,
`onboarding_stream_svc`, `OBM_PORT`, and `KEYCLOAK_URL` to establish initial
connectivity. These are passed as kernel arguments or environment variables. Under
single-domain mode, `onboarding_manager_svc` and `onboarding_stream_svc` both
become `DOMAIN`.

---

## 6. Phase 4: Edge Node Agents

### 6.1 Configuration Changes (8 agents)

Each agent has a configuration struct and YAML file. Under single-domain mode, the
`serviceURL` for each agent points to the same hostname but Traefik routes based on
the gRPC service path.

| Agent | Config File | Config Struct | Current serviceURL pattern |
|---|---|---|---|
| node-agent | `configs/node-agent.yaml` | `config/config.go` | `infra.DOMAIN:443` |
| cluster-agent | `configs/cluster-agent.yaml` | `config/config.go` | `cluster-orch-node.DOMAIN:443` |
| platform-update-agent | `configs/platform-update-agent.yaml` | `config/config.go` | `update-node.DOMAIN:443` |
| platform-manageability-agent | (config via cloud-init) | `config/config.go` | `device-manager-node.DOMAIN:443` |
| hardware-discovery-agent | `config/hd-agent.yaml` | `config/config.go` | `infra-node.DOMAIN:443` |
| platform-telemetry-agent | (config via cloud-init) | `config/config.go` | `telemetry-node.DOMAIN:443` |
| platform-observability-agent | (config via cloud-init) | `config/config.go` | `logs-node.DOMAIN:443` / `metrics-node.DOMAIN:443` |
| device-discovery-agent | CLI flags / env vars | `config/config.go` | `onboarding-node.DOMAIN:443` |

**Under single-domain mode**, all of these become `DOMAIN:443`. No code change is
needed in the gRPC dial logic itself — the `grpc.NewClient(target, ...)` calls will
simply receive the single domain as the target. The gRPC service path is already sent
automatically by the generated protobuf client stubs.

**However**, the agents need to be tested to confirm they work correctly when multiple
agents dial the same hostname but expect different backend services. This should work
because gRPC's HTTP/2 framing includes the service path, but it needs E2E validation.

### 6.2 gRPC Dial Sites (7+ files)

These files contain `grpc.NewClient()` or `grpc.Dial()` calls that will receive the
new single-domain target:

| File | Function | Current target |
|---|---|---|
| `cluster-agent/internal/comms/comms.go:62` | `ConnectToClusterOrch` | `cli.ServerAddr` |
| `platform-update-agent/internal/comms/comms.go` | `ConnectToEdgeInfrastructureManager` | `cli.MMServiceAddr` |
| `platform-manageability-agent/internal/comms/comms.go` | `ConnectToDMManager` | `confs.Manageability.ServiceURL` |
| `node-agent/internal/hostmgr_client/hostmgr_client.go` | `ConnectToHostMgr` | config serviceURL |
| `hardware-discovery-agent/internal/comms/comms.go` | `ConnectToEdgeInfrastructureManager` | config serviceURL |
| `device-discovery-agent/internal/mode/interactive/client.go:41` | gRPC dial | `cfg.ObmSvc:cfg.ObmPort` |
| `device-discovery-agent/internal/mode/noninteractive/client.go` | gRPC streaming dial | `cfg.ObsSvc:cfg.ObmPort` |

---

## 7. Phase 5: Web UI

> **Helmfile EIM status:** `web-ui-root`, `web-ui-infra`, `web-ui-admin` are enabled.
> `web-ui-app-orch` and `web-ui-cluster-orch` are **disabled**.

### 7.1 Runtime Configuration

The UI uses `runtime-config.js` files that define all service endpoint URLs at deploy
time. These are the primary configuration surface.

**Files (5 apps — 3 enabled in EIM, 2 future):**
- `apps/root/public/runtime-config.js` — **enabled**
- `apps/admin/public/runtime-config.js` — **enabled**
- `apps/infra/public/runtime-config.js` — **enabled**
- `apps/app-orch/public/runtime-config.js` — disabled in EIM (future: Phase 8)
- `apps/cluster-orch/public/runtime-config.js` — disabled in EIM (future: Phase 7)

**Current pattern:** `https://api.${fqdn}`, `https://keycloak.${fqdn}`, etc.
**New pattern:** `https://${fqdn}/api`, `https://${fqdn}/auth`, etc.

### 7.2 Subdomain Derivation (BREAKING — must fix)

Three locations derive service URLs by replacing subdomains in `window.location.origin`:

**1. ExtensionHandler** (`apps/root/src/components/atoms/ExtensionHandler/ExtensionHandler.tsx:23–31`)
```typescript
// Current: replaces "web-ui" with "api-proxy" in the origin
baseUrl = `${window.location.origin.replace("web-ui", "api-proxy")}/${serviceName}...`
```
Under single-domain, `window.location.origin` won't contain `web-ui`. This code
already has a fallback for development mode, but the production path **breaks**.

**Fix:** Use the RuntimeConfig to get the api-proxy base URL instead of deriving it.

**2. VNC Console** (`apps/app-orch/src/components/organisms/deployments/ApplicationDetails/ApplicationDetails.tsx:200`)
```typescript
// Current: replaces "web-ui" with "vnc" in the origin
window.location.origin.replace("web-ui", "vnc")
```

**Fix:** Use `RuntimeConfig` for the VNC endpoint (e.g., `${origin}/vnc/`).
This is in the app-orch UI which is disabled in EIM, but the fix belongs here since
it lives in the orch-ui repo.

**3. Cypress Tests** (`tests/cypress/support/commands.ts:89,105`)
```typescript
// Current: replaces "web-ui" with "keycloak" and "api" in the base URL
baseUrl.replace("web-ui", "keycloak")
baseUrl.replace("web-ui", "api")
```

**Fix:** Use explicit test configuration for keycloak and API base URLs.

### 7.3 Helm Deploy Values (5 files)

Each app's `deploy/values.yaml` contains hardcoded service endpoints
(`https://keycloak.kind.internal`, `https://api.kind.internal`, etc.) that need
updating to path-based URLs.

### 7.4 OIDC Configuration

`library/utils/authConfig/authConfig.ts:15-21` constructs the OIDC authority URL from
`KC_URL` runtime config. This only needs the `KC_URL` value to change from
`https://keycloak.DOMAIN` to `https://DOMAIN/auth`.

---

## 8. Phase 6: CLI

### 8.1 Keycloak Endpoint Derivation (BREAKING — must fix)

**File:** `orch-cli/internal/cli/login.go:82–94`

```go
// Current: derives keycloak endpoint from API endpoint by subdomain replacement
parts := strings.SplitN(u.Host, ".", 2)
keycloakEp = fmt.Sprintf("https://keycloak.%s/realms/master", parts[1])
```

Under single-domain, the API endpoint is `https://DOMAIN/api/` and there's no
subdomain to extract. This derivation **breaks**.

**Fix:** Change the derivation to use path-based construction:
```go
keycloakEp = fmt.Sprintf("https://%s/auth/realms/master", u.Host)
```

**File:** `orch-cli/internal/cli/login.go:305–320` — Reverse derivation
(`deriveAPIEndpointFromKeycloakEndpoint`) also needs updating.

**File:** `orch-cli/internal/cli/login_test.go:26–32` — Test cases need updating.

### 8.2 All Other CLI Endpoints

The CLI already routes all API calls through a single `--api-endpoint` with path-based
routing (`/v1/`, `/v2/`, `/v3/`). **No changes needed** for service factory functions
in `internal/cli/utils.go`.

---

## 9. Phase 7: Cluster Services

> **Helmfile EIM status:** `cluster-manager`, `intel-infra-provider` (CAPI), and
> `capi-operator` are all **disabled** in the EIM profile. `cluster-connect-gateway`
> is not deployed via the helmfile but may be deployed separately.
> This phase is **future work** — needed when cluster orchestration is added to
> the helmfile installer.

### 9.1 cluster-connect-gateway

Already uses path-based routing (`/connect`, `/kubernetes`). Only Helm values need
updating:
- `deployment/charts/.../values.yaml:102` — Change `connect-gateway.kind.internal` to the single domain
- `deployment/charts/.../values.yaml:90` — Change `externalUrl`

### 9.2 cluster-manager

Helm values reference:
- `values.yaml:107` — `clusterdomain: kind.internal` — Used for constructing internal URLs
- OIDC issuer references internal K8s service URL — **no change**

### 9.3 cluster-api-provider-intel

**IngressRoute:** `deployment/charts/intel-infra-provider/templates/southbound_api_service.yaml:31-39`
- Uses `Host('{{ .Values.traefikReverseProxy.host.grpc.name }}') && PathPrefix('/')`
- Hostname fed from EMF helmfile values — change handled in Phase 1 (EMF side)
- The PathPrefix(`/`) should be narrowed to the specific gRPC service path

**Test fixtures with hardcoded keycloak.kind.internal:**

| File | Line | Value |
|---|---|---|
| `pkg/testing/testing_utils.go` | 32, 52 | `"iss": "https://keycloak.kind.internal/realms/master"` |
| `pkg/auth/auth_multitenancy/auth_test.go` | 29 | `issuer: "https://keycloak.example.com"` (already generic) |

**Fix:** Update `testing_utils.go` test JWT issuer to be configurable or use the
path-based URL `https://DOMAIN/auth/realms/master`.

---

## 10. Phase 8: Application Orchestration

> **Helmfile EIM status:** `app-orch-catalog` and `app-orch-deployment` (app-service-proxy,
> app-deployment-manager, app-resource-manager) are all **disabled** in the EIM profile.
> These components may be deployed separately outside helmfile today.
> This phase is needed when application orchestration is added to the helmfile installer
> or when DNS simplification is applied to a standalone app-orch deployment.

### 10.1 app-orch-deployment

**IngressRoutes (3 files):**
- `app-service-proxy/deployments/.../templates/service.yaml` — Update `matchRoute` from `Host('app-service-proxy.DOMAIN')` to path-based
- `app-deployment-manager/deployment/.../templates/ingressroute.yaml` — Already uses `Host() && PathRegexp()`, just update Host
- `app-resource-manager/deployments/.../templates/ingressroute.yaml` — Same pattern

**Helm Values (3 files):**
- `app-service-proxy/deployments/.../values.yaml` — `matchRoute`, `domainName`, `hostName`, `allowedOrigins`
- `app-deployment-manager/deployment/.../values.yaml` — `gitServer`, `apiHostname`
- `app-resource-manager/deployments/.../values.yaml` — `domainName`, `hostName`, `allowedOrigins`

**URL Rewriting (1 file):**
- `app-service-proxy/internal/server/transport.go` — Dynamically patches `frame-ancestors` using `X-Forwarded-Host`. This already handles dynamic hostnames via the header, so it should adapt without code changes — but needs testing.

**Embedded Auth/Web Assets (IMPORTANT — understated in earlier versions):**

The app-service-proxy and VNC proxy both contain embedded web login flows with JavaScript
that derives Keycloak URLs from the browser hostname. These are **not just configuration
changes** — they require real path-routing design.

| File | Line | Issue |
|---|---|---|
| `app-service-proxy/web-login/app-service-proxy-main.js` | 52-53 | `domain = window.location.hostname.split('.').slice(1).join('.')` then `keycloakUrl = 'https://keycloak.' + domain` — subdomain derivation breaks under single-domain |
| `app-resource-manager/vnc-proxy-web-ui/vnc-proxy-main.js` | 127-132 | Hardcoded fallback `vnc.kind.internal` and constructs WebSocket address from `window.location.hostname` — WebSocket URL construction needs path-based equivalent |

**Backend URL generation surfaces (Go):**

| File | Lines | Issue |
|---|---|---|
| `app-service-proxy/internal/server/server.go` | 158 | Redirect to `/app-service-proxy-index.html` on missing cookie — hardcoded path must include routing prefix under single-domain |
| `app-service-proxy/internal/server/server.go` | 266-300 | `HandleFunc` routes for `/app-service-proxy-index.html`, `/app-service-proxy-main.js`, `/app-service-proxy-keycloak.min.js`, `/app-service-proxy-styles.css` — all fixed paths that must work under the subpath prefix |
| `app-resource-manager/internal/kubernetes/utils.go` | 125-138 | `createServiceProxyURL()` — generates `{domainName}/app-service-proxy-index.html?project=...&cluster=...&namespace=...&service=...` URLs. The `domainName` is currently the subdomain-based host; under single-domain the path prefix must be prepended to `/app-service-proxy-index.html` |
| `app-resource-manager/internal/kubevirt/manager.go` | 204, 220-224 | `GetVNCConsoleAddr()` — generates `{protocol}://{HostName}/{VNCWebSocketPrefix}/{project}/{app}/{cluster}/{vm}` WebSocket URLs. `HostName` comes from config (`vnc.kind.internal`); under single-domain this becomes the bare domain and the VNC routing prefix must be incorporated |
| `app-resource-manager/internal/vncproxy/manager.go` | 99-130 | `HandleFunc` routes for `/` (serves `vnc-proxy-index.html`), `/vnc-proxy-main.js`, `/vnc-proxy-styles.css`, `/rfb.js`, `/keycloak.min.js` — all fixed paths; the query-parameter-based routing at line 113 also needs path-prefix awareness |
| `app-resource-manager/internal/vncproxy/manager.go` | 67-72 | WebSocket handler registered at `/{VNCWebSocketPrefix}/{project}/{app}/{cluster}/{vm}` — WebSocket path must incorporate routing prefix |

**Fix for app-service-proxy-main.js:** Replace subdomain derivation with either:
1. A `<script>` tag that injects the Keycloak URL from server-side configuration, or
2. A fetch to a `/config` endpoint that returns the Keycloak URL

**Fix for vnc-proxy-main.js:** Use `window.location.origin + '/vnc'` instead of
reconstructing from hostname. The WebSocket upgrade path should work through Traefik's
path-based routing since Traefik supports WebSocket upgrades on any route.

### 10.2 app-orch-catalog

**IngressRoute:** `deployments/.../templates/ingressroute.yaml` — Update `apiHostname`
**Test data:** `test/utils/types/types.go` — Hardcoded `registry-oci.{orchDomain}` URLs

### 10.3 Gitea Subpath Configuration

Gitea is used exclusively by the App Deployment Manager (ADM) to store deployment
configs as Git repos, which Fleet then watches. It is not deployed via helmfile and has
no helmfile values file — the `giteaMatchHost` entry in
`traefik-extra-objects.yaml.gotmpl:26` is dead config under the EIM profile (no Gitea
backend is deployed).

When app-orch is enabled and Gitea is deployed, the Traefik match changes to
PathPrefix-based and Gitea's `ROOT_URL` must be set to `https://DOMAIN/gitea/`. Gitea
supports `ROOT_URL` subpath natively via its `[server].ROOT_URL` setting. This needs
**validation** with a working deployment — asset paths, Git clone URLs, and webhook
callback URLs all derive from `ROOT_URL`. Fleet's interaction with Gitea is the
primary concern.

---

## 11. Phase 9: Observability

> **Helmfile EIM status:** `edgenode-observability`, `orchestrator-observability`,
> `alerting-monitor`, and related dashboards are all **disabled** in the EIM profile.
> These components may be deployed separately outside helmfile today.
> This phase is needed when observability is added to the helmfile installer or when
> DNS simplification is applied to a standalone observability deployment. When added
> to helmfile, the deployment will need observability values files (not present today)
> to set Grafana `root_url`, `serve_from_sub_path`, and Keycloak OAuth URLs.

### 11.1 Grafana Root URL and Subpath Configuration

**Files (o11y-charts — default values):**
- `o11y-charts/charts/edgenode-observability/deployments/.../values.yaml:632`
  `root_url: https://observability-ui.kind.internal` → `https://DOMAIN/grafana/`
- `o11y-charts/charts/orchestrator-observability/deployments/.../values.yaml:682`
  `root_url: https://observability-admin.kind.internal` → `https://DOMAIN/grafana-admin/`

The Traefik routing for observability is already defined in the helmfile deployment via
`traefik-extra-objects.yaml.gotmpl:18-19` (Host match rules for `observability-ui`
and `observability-admin`). When observability is enabled, these must be updated to
path-based matches alongside the other Traefik routes in Phase 1.

The o11y-charts default values set `root_url` to subdomain-based URLs. When
observability is added to the helmfile installer, the helmfile values templates must
override `root_url` to path-based URLs (e.g., `https://DOMAIN/grafana/`).

**VALIDATION REQUIRED: `serve_from_sub_path`**

Grafana's subpath support requires `serve_from_sub_path: true` under `[server]` in
`grafana.ini`. This setting is **not present anywhere** in the current configuration.
Without it, Grafana will serve assets from `/` regardless of `root_url`, causing 404s
on `/grafana/public/...` paths. This must be set in the helmfile observability values
and validated with a working deployment.

### 11.2 OAuth Redirect URLs in Grafana Config

Both Grafana instances have Keycloak OAuth configuration:
- `auth_url` changes from `https://keycloak.DOMAIN/realms/master/...` to `https://DOMAIN/auth/realms/master/...`
- `signout_redirect_url` changes from referencing `web-ui.DOMAIN` to just `DOMAIN`

These are set in the o11y-charts default values. The helmfile deployment must override
them when observability is enabled.

### 11.3 o11y-alerting-monitor

**IngressRoute:** `deployments/alerting-monitor/templates/service.yaml:54-79`
- Uses `{{ .Values.traefik.matchRoute }}` — currently a Host() match
- The helmfile deployment does not include an alerting-monitor values override; the `matchRoute` value comes from the chart's own `values.yaml` (defaulting to `alerting-monitor.domain`)
- Change to path-based: `Host('DOMAIN') && PathPrefix('/alerts/')`

**Keycloak OIDC endpoint:**

| File | Line | Current Value | Change |
|---|---|---|---|
| `deployments/alerting-monitor/values.yaml` | 63 | `oidcServer: "https://keycloak.kind.internal"` | → `"https://DOMAIN/auth"` |
| `internal/config/_testdata/test_config.yaml` | 14 | `oidcServer: "https://keycloak.kind.internal"` | Update test fixture |
| `internal/config/config_test.go` | 22 | Assertion against `keycloak.kind.internal` | Update test assertion |

**No code changes needed** in `internal/app/m2mtoken.go` — it constructs OIDC paths using
`fmt.Sprintf("%s/realms/%s/...", oidcServer, oidcRealm)` which works with either base URL.

---

## 12. Phase 10: Tests, CI, Installer, and Documentation

This phase also includes cleanup of test fixtures in "no runtime changes" repos
(Section 2) — e.g., `app-orch-tenant-controller`'s `harbor.kind.internal` test fixture.
These are non-blocking but should be updated for consistency.

### 12.0 Integration Tests (cluster-tests, orch-ci)

**cluster-tests** has several files that need updating:

| File | Line | What | Change Needed |
|---|---|---|---|
| `tests/utils/auth_utils.go` | 24 | `ConnectGatewayInternalAddress = "https://connect-gateway.kind.internal:443"` | Change to single-domain URL |
| `scripts/ven/dnsmasq_kind_internal.sh` | 24, 63 | Wildcard dnsmasq setup (`*.kind.internal → IP`) | Simplifies — only one name to resolve. Script already uses configurable `KIND_INTERNAL_FQDN_SUFFIX` |
| `.github/workflows/virtual-integration.yml` | 183, 584 | Calls `dnsmasq-setup.sh "kind.internal"` | May need adjustment if dnsmasq strategy changes |
| `tests/auth/jwt.go` | 26 | `platform-keycloak.orch-platform.svc` | No change (internal K8s DNS) |

**orch-ci** has no internal subdomain references. Only external registries
(`registry-rs.edgeorchestration.intel.com`, AWS ECR) which are out of scope.

### 12.1 Virtual Edge Node (virtual-edge-node)

This repo contains the **most comprehensive subdomain inventory** — its dnsmasq script
lists all 40 subdomains. Under single-domain, this simplifies dramatically.

**File:** `vm-provisioning/scripts/ci_setup_dnsmasq.sh:85-125`
- Lists 40 subdomain → IP address mappings (the full inventory)
- **Fix:** Under single-domain, collapse to a single `address=/$CLUSTER_FQDN/$cluster_lb` entry
  (plus exceptions for tinkerbell-haproxy and registry-oci)

**Files with Keycloak/API URL construction:**

| File | Lines | Pattern |
|---|---|---|
| `vm-provisioning/scripts/nio_flow_validation.sh` | 14, 33 | `keycloak.${cluster_fqdn}`, `api.${cluster_fqdn}` |
| `vm-provisioning/scripts/host_status_check.sh` | 29, 35, 49 | `keycloak.${CLUSTER}`, `api.${CLUSTER}` |
| `vm-provisioning/scripts/show_host-status.sh` | 37, 48, 54, 68 | `keycloak.${CLUSTER}`, `api.${CLUSTER}` |
| `vm-provisioning/scripts/update_provider_defaultos.sh` | 37, 42, 92, 122, 135, 140 | `keycloak.${CLUSTER}`, `api.${CLUSTER}` |
| `vm-provisioning/scripts/create_vm.sh` | 56 | `tinkerbell-haproxy.${CLUSTER}` (keep as subdomain) |
| `vm-provisioning/scripts/ci_network_bridge.sh` | 27 | `tinkerbell-haproxy.${CLUSTER}` (keep as subdomain) |
| `vm-provisioning/config` | 9 | `CLUSTER="kind.internal"` |

**Fix pattern:** In each script, change `keycloak.${CLUSTER}` → `${CLUSTER}/auth` and
`api.${CLUSTER}` → `${CLUSTER}/api`. Tinkerbell references remain as subdomains.

### 12.2 Test Automation (edge-manage-test-automation)

This is the end-to-end Robot Framework test suite. It uses a configuration-driven approach
where all URLs are constructed from `cluster_fqdn`.

**Central variable definition:** `resources/utils/Variables.resource:14-17`
```robot
kc_url=https://keycloak.${ORCHESTRATOR.cluster_fqdn}
api_url=https://api.${ORCHESTRATOR.cluster_fqdn}
ui_url=https://web-ui.${ORCHESTRATOR.cluster_fqdn}
tinkerbell_haproxy_url=tinkerbell-haproxy.${ORCHESTRATOR.cluster_fqdn}
```

**Fix:** Update these 4 lines to use path-based URLs:
```robot
kc_url=https://${ORCHESTRATOR.cluster_fqdn}/auth
api_url=https://${ORCHESTRATOR.cluster_fqdn}/api
ui_url=https://${ORCHESTRATOR.cluster_fqdn}
tinkerbell_haproxy_url=tinkerbell-haproxy.${ORCHESTRATOR.cluster_fqdn}  # exception
```

**Onboarding utilities:** `resources/edge_node/OnboardUtils.py`

| Line | Pattern | Change |
|---|---|---|
| 47, 97-98, 122-123, 147 | `tinkerbell-haproxy.{orchestrator['cluster_fqdn']}` | No change (exception) |
| 184 | `api.{orchestrator['cluster_fqdn']}` | → `{orchestrator['cluster_fqdn']}/api` |

**Orchestrator configs:** `orchestrator-configs/kind.yaml`, `on-prem.yaml`
- Contains `cluster_fqdn` setting — no change needed here since URL construction
  happens in the Variables.resource and OnboardUtils.py above

**Security scan tests:** `tests/resilience_performance_and_security/resilience_performance_and_security.robot:193,202`
- References `https://web-ui.${ORCHESTRATOR.cluster_fqdn}` — fix to use `ui_url` variable

### 12.3 EMF CI and Installer Plumbing

**CI DNS setup (both variants):**

| File | Lines | What |
|---|---|---|
| `ci/ven/dnsmasq-setup.sh` | 92-134 | Legacy dnsmasq script — 40+ subdomain address entries with LoadBalancer IP lookup |
| `ci/ven/dnsmasq-setup-helmfile.sh` | 88-131 | Helmfile variant — same 40+ subdomain entries but all mapped to host IP (no LB lookup) |

Both scripts collapse to a single `address=/$CLUSTER_FQDN/$ip_address` entry under
single-domain (plus exceptions for tinkerbell-haproxy and registry-oci).

**Helmfile environment config:**

**File:** `helmfile-deploy/post-orch/post-orch.env:12`
- Sets `EMF_CLUSTER_DOMAIN=cluster.onprem` — the single source of truth for the domain
  variable that feeds into all helmfile values templates via
  `onprem-eim-settings.yaml.gotmpl:21`

### 12.4 Installer Scripts

**Note:** The ArgoCD-based deployment used `on-prem-installers/onprem/onprem.env` and
`generate_cluster_yaml.sh`. These no longer exist in the helmfile branch. The equivalent
is `helmfile-deploy/post-orch/post-orch.env` (covered above) and the helmfile
environment templates in `helmfile-deploy/post-orch/environments/`.

### 12.5 Documentation

**Repo:** `edge-manage-docs`
- All deployment guides reference subdomain setup
- Edge node setup docs reference multiple hostnames
- API reference docs reference `api.DOMAIN` (may keep working with redirect)

---

## 13. Subdomain-to-Path Mapping (Complete)

### User-Facing Services

| Current Subdomain | New Path | Routing Complexity |
|---|---|---|
| `web-ui.DOMAIN` | `DOMAIN/` (root) | Straightforward |
| `api.DOMAIN` | `DOMAIN/api/` | Already path-based internally |
| `keycloak.DOMAIN` | `DOMAIN/auth/` | Moderate (Keycloak native support) |
| `vault.DOMAIN` | `DOMAIN/vault/` | Straightforward |
| `registry-oci.DOMAIN` | **Keep as subdomain** | N/A (Docker API constraint) |
| `observability-ui.DOMAIN` | `DOMAIN/grafana/` | Requires `serve_from_sub_path: true` — validate |
| `observability-admin.DOMAIN` | `DOMAIN/grafana-admin/` | Same — validate |
| `alerting-monitor.DOMAIN` | `DOMAIN/alerts/` | Straightforward |
| `gitea.DOMAIN` | `DOMAIN/gitea/` | App-orch only; requires `ROOT_URL` subpath — validate clone/webhook/LFS |
| `app-orch.DOMAIN` | `DOMAIN/app-orch/` | Straightforward |
| `app-service-proxy.DOMAIN` | `DOMAIN/app-proxy/` | Embedded JS login flow + backend path rewrite (see Phase 8) |
| `ws-app-service-proxy.DOMAIN` | `DOMAIN/ws-app-proxy/` | WebSocket variant of app-service-proxy |
| `vnc.DOMAIN` | `DOMAIN/vnc/` | Embedded JS + WebSocket path rewrite (see Phase 8) |
| `docs-ui.DOMAIN` | `DOMAIN/docs/` | Straightforward |
| `cluster-management.DOMAIN` | `DOMAIN/cluster-mgmt/` | Straightforward |
| `connect-gateway.DOMAIN` | `DOMAIN/connect/` | Already path-based |
| `fleet.DOMAIN` | `DOMAIN/fleet/` | Straightforward |
| `metadata.DOMAIN` | `DOMAIN/metadata/` | Straightforward |
| `release.DOMAIN` | `DOMAIN/release/` | Straightforward |
| `api-proxy.DOMAIN` | `DOMAIN/api-proxy/` | Straightforward |

### Edge-Node-Facing gRPC Services

| Current Subdomain | gRPC Service Path | Routing Method |
|---|---|---|
| `infra-node.DOMAIN` | `/hostmgr_southbound_proto.Hostmgr/*` | gRPC PathPrefix |
| `update-node.DOMAIN` | `/maintmgr.v1.MaintmgrService/*` | gRPC PathPrefix |
| `telemetry-node.DOMAIN` | `/telemetrymgr.v1.TelemetryMgr/*` | gRPC PathPrefix |
| `attest-node.DOMAIN` | `/attestmgr.v1.AttestationStatusMgrService/*` | gRPC PathPrefix |
| `cluster-orch-node.DOMAIN` | `/cluster_orchestrator_southbound_proto.ClusterOrchestratorSouthbound/*` | gRPC PathPrefix |
| `onboarding-node.DOMAIN` | `/onboardingmgr.v1.InteractiveOnboardingService/*` | gRPC PathPrefix |
| `onboarding-stream.DOMAIN` | `/onboardingmgr.v1.NonInteractiveOnboardingService/*` | gRPC PathPrefix |
| `device-manager-node.DOMAIN` | `/device_management.DeviceManagement/*` | gRPC PathPrefix |
| `logs-node.DOMAIN` | `/logs-node/` prefix | HTTP PathPrefix |
| `metrics-node.DOMAIN` | `/metrics-node/` prefix | HTTP PathPrefix |

### Additional Subdomains (from virtual-edge-node inventory)

The virtual-edge-node dnsmasq script (`ci_setup_dnsmasq.sh:85-125`) provided the
**definitive subdomain inventory**. Subdomains not already covered above:

| Current Subdomain | New Path | Notes |
|---|---|---|
| `argo.DOMAIN` | `DOMAIN/argo/` | ArgoCD UI (ArgoCD-era only — not present in helmfile deployment) |
| `cluster-orch.DOMAIN` | `DOMAIN/cluster-orch/` | Cluster orchestration API |
| `cluster-orch-edge-node.DOMAIN` | `DOMAIN/cluster-orch-edge/` | Edge-node facing cluster orch |
| `infra.DOMAIN` | `DOMAIN/infra/` | Infrastructure API |
| `license-node.DOMAIN` | `DOMAIN/license-node/` | License service edge-node facing |
| `log-query.DOMAIN` | `DOMAIN/log-query/` | Log query API |
| `onboarding.DOMAIN` | `DOMAIN/onboarding/` | Onboarding API (orchestrator-facing) |
| `orchestrator-license.DOMAIN` | `DOMAIN/license/` | License service orchestrator-facing |
| `rancher.DOMAIN` | `DOMAIN/rancher/` | Rancher management UI |
| `registry.DOMAIN` | `DOMAIN/registry/` | Registry API (non-OCI) |
| `update.DOMAIN` | `DOMAIN/update/` | Update service orchestrator-facing |
| `telemetry.DOMAIN` | `DOMAIN/telemetry/` | Telemetry API (orchestrator-facing) |

### Special Cases (unchanged)

| Subdomain | Reason |
|---|---|
| `registry-oci.DOMAIN` | Docker registry API requires root path ownership |
| `mps.DOMAIN:4433` | CIRA protocol on dedicated port — no routing change needed |
| `tinkerbell-server.DOMAIN` | PXE boot infrastructure — keep separate |
| `tinkerbell-haproxy.DOMAIN` | PXE boot load balancer — keep separate |

---

## 14. gRPC Service Path Uniqueness Verification

For single-domain gRPC routing to work, every gRPC service must have a unique
`package.ServiceName`. Verified from proto files across all repos:

| Package.Service | Repository | Unique? |
|---|---|---|
| `hostmgr_southbound_proto.Hostmgr` | infra-managers | Yes |
| `maintmgr.v1.MaintmgrService` | infra-managers | Yes |
| `telemetrymgr.v1.TelemetryMgr` | infra-managers | Yes |
| `attestmgr.v1.AttestationStatusMgrService` | infra-managers | Yes |
| `cluster_orchestrator_southbound_proto.ClusterOrchestratorSouthbound` | cluster-api-provider-intel | Yes |
| `onboardingmgr.v1.InteractiveOnboardingService` | infra-onboarding | Yes |
| `onboardingmgr.v1.NonInteractiveOnboardingService` | infra-onboarding | Yes |
| `device_management.DeviceManagement` | infra-external | Yes |
| `catalog.v3.CatalogService` | app-orch-catalog | Yes |
| `deployment.v1.DeploymentService` | app-orch-deployment | Yes |
| `deployment.v1.ClusterService` | app-orch-deployment | Yes |
| `resource.v2.AppWorkloadService` | app-orch-deployment | Yes |
| `resource.v2.EndpointsService` | app-orch-deployment | Yes |
| `inventory.v1.InventoryService` | infra-core | Yes (internal only) |
| `v1.MetadataService` | orch-metadata-broker | Yes (internal only) |

**Result: All gRPC service paths are unique.** Single-domain gRPC routing is viable.

---

## 15. Dangerous Patterns: Subdomain Derivation by String Replacement

These patterns construct one service URL by replacing the subdomain of another. They
**break completely** under single-domain routing and must be replaced with explicit
configuration.

### Pattern 1: Keycloak → Release service (edge node agents)

| File | Line | Code |
|---|---|---|
| `edge-node-agents/device-discovery-agent/internal/auth/auth.go` | 30 | `strings.Replace(keycloakURL, "keycloak", "release", 1) + releaseTokenURL` |
| `edge-node-agents/device-discovery-agent/internal/mode/interactive/tty_auth.go` | 227 | `strings.Replace(t.keycloakURL, "keycloak", "release", 1) + config.ReleaseTokenURL` |
| `infra-onboarding/hook-os/device_discovery/client-secret-auth.go` | 158 | `strings.Replace(keycloakURL, "keycloak", "release", 1) + releaseTokenURL` |

**Fix:** Add an explicit `releaseServiceURL` to the infra-config ConfigMap and agent
configuration. Stop deriving it from the Keycloak URL.

### Pattern 2: API → Keycloak (CLI)

| File | Line | Code |
|---|---|---|
| `orch-cli/internal/cli/login.go` | 88–92 | `strings.SplitN(u.Host, ".", 2)` then `fmt.Sprintf("https://keycloak.%s/realms/master", parts[1])` |
| `orch-cli/internal/cli/login.go` | 305–319 | Reverse: Keycloak → API endpoint |

**Fix:** Under single-domain, derive using path instead of subdomain: `https://HOST/auth/realms/master`.

### Pattern 3: web-ui → other subdomains (UI)

| File | Line | Code |
|---|---|---|
| `orch-ui/apps/root/src/components/atoms/ExtensionHandler/ExtensionHandler.tsx` | 28 | `window.location.origin.replace("web-ui", "api-proxy")` |
| `orch-ui/apps/app-orch/src/.../ApplicationDetails.tsx` | 200 | `window.location.origin.replace("web-ui", "vnc")` |
| `orch-ui/tests/cypress/support/commands.ts` | 89, 105 | `baseUrl.replace("web-ui", "keycloak")`, `baseUrl.replace("web-ui", "api")` |

**Fix:** Use RuntimeConfig for all service URLs instead of deriving from `window.location.origin`.

### Pattern 4: hostname → Keycloak (app-service-proxy embedded JS)

| File | Line | Code |
|---|---|---|
| `app-orch-deployment/app-service-proxy/web-login/app-service-proxy-main.js` | 52-53 | `domain = window.location.hostname.split('.').slice(1).join('.')` then `keycloakUrl = 'https://keycloak.' + domain` |
| `app-orch-deployment/app-resource-manager/vnc-proxy-web-ui/vnc-proxy-main.js` | 127-128 | Hardcoded fallback `vnc.kind.internal`, WebSocket address from `window.location.hostname` |

**Fix:** Inject Keycloak URL from server-side config (see Phase 8 detail).

---

## 16. Migration Strategy: Dual-Routing Support

### Phase 1: Dual-Routing (Non-Breaking)

Add path-based routes **alongside** subdomain routes. Both work simultaneously. This is
the key enabler for incremental delivery — no component needs a synchronized cutover.

**Implementation:**
- All IngressRoute templates accept a `singleDomain.enabled` flag
- When `false` (default): only existing subdomain Host() rules
- When `true`: **adds** additional PathPrefix() rules on the root domain **without
  removing** the subdomain rules. Both route sets are active simultaneously.
- This is additive (`if enabled, also add`), not exclusive (`if/else`). See Section 3.2
  for the concrete template pattern.

**Edge node strategy:**
- Existing deployed edge nodes continue using subdomain-based configuration
- Newly onboarded edge nodes receive single-domain configuration (if
  `singleDomain.enabled`)
- Agents work with either configuration since the gRPC dial target is just a hostname

### Phase 2: Default to Single Domain

New deployments default to `singleDomain.enabled: true`. Existing deployments can
opt in.

### Phase 3: Deprecate Subdomain Mode

Remove the dual-routing conditionals. Subdomain routing configuration is deleted.

---

## 17. Risks and Open Issues

### High Risk

1. **gRPC routing through Traefik with PathPrefix on service paths.** This is the
   architectural bet. Traefik supports gRPC routing via PathPrefix, but it needs to
   correctly parse the HTTP/2 `:path` pseudo-header from gRPC frames. Must be validated
   with a prototype before committing to the full implementation.

2. **Edge node agent upgrade path.** Deployed edge nodes have configuration files at
   `/etc/edge-node/node/agent_variables` with hardcoded hostnames. These nodes need a
   mechanism to update their configuration. Options:
   - Push new config via the maintenance manager (platform update flow)
   - Edge nodes re-onboard (heavy-handed)
   - Dual-routing (Phase 1) means old configs keep working indefinitely

3. **Keycloak path-based deployment.** While Keycloak documents `http-relative-path`
   support, the interaction with the JWT validation middleware in Traefik needs testing.
   The JWKS URL changes from `http://platform-keycloak.orch-platform.svc:80/realms/...`
   — this is internal and unchanged, so it should be fine.

### Medium Risk

4. **Harbor exception.** Keeping `registry-oci` as a subdomain means the system still
   needs 2 DNS records minimum. Consider whether Harbor can be dropped from the default
   profile (as the design proposal suggests).

5. **Tinkerbell exception.** The PXE boot infrastructure may need its own IP/hostname
   for DHCP/TFTP reasons. Investigate whether it can use the single domain.

6. **CSP/CORS during dual-routing.** During Phase 1, the CSP and CORS policies need to
   allow both subdomain and path-based origins. This is slightly more permissive than
   either mode alone.

7. **Grafana sub-path support.** Grafana documents `serve_from_sub_path: true`, but
   this setting is **not present** anywhere in the current configuration. Without it,
   Grafana will not serve static assets correctly under `/grafana/`. Requires
   validation with a real deployment — not just config change.

8. **Gitea sub-path support (app-orch only).** Gitea is used exclusively by app-orch's
   ADM. It supports `ROOT_URL` for sub-path deployment, but Git clone URLs, webhook
   callbacks, and LFS endpoints all derive from `ROOT_URL`. Requires validation,
   especially for Fleet's interaction with Gitea. Not applicable under EIM profile
   where app-orch and Gitea are not deployed.

9. **App-service-proxy and VNC embedded web assets.** Both contain JavaScript login
   flows that derive Keycloak URLs from `window.location.hostname` subdomain parsing.
   These need real refactoring, not just config changes (see Phase 8 detail).

### Low Risk

10. **URL rewriting in app-service-proxy.** The `transport.go` code uses
    `X-Forwarded-Host` dynamically — should adapt without changes, but needs testing.

---

## 18. Effort Summary

### By Phase

| Phase | Description | Repos | Files | Manual | AI-assisted |
|---|---|---|---|---|---|
| 1 | Traefik routing, certs, CSP | orch-utils, infra-charts, EMF | ~40 | 5 days | 2.5 days |
| 2 | Keycloak realm | EMF | ~2 | 1 day | 0.5 days |
| 3 | Edge node provisioning | EMF, infra-onboarding | ~5 | 1 day | 0.5 days |
| 4 | Edge node agents | edge-node-agents | ~20 | 2 days | 1 day |
| 5 | Web UI | orch-ui | ~15 | 2 days | 1 day |
| 6 | CLI | orch-cli | ~5 | 1 day | 0.5 days |
| 7 | Cluster services (*future*) | cluster-connect-gw, cluster-mgr, cluster-api-provider-intel | ~6 | 1 day | 0.5 days |
| 8 | App orchestration (*future*, incl. app-service-proxy/VNC web assets, Gitea subpath) | app-orch-* | ~15 | 3 days | 1.5 days |
| 9 | Observability (*future*, Grafana subpath validation) | o11y-charts, o11y-alerting-monitor | ~8 | 1.5 days | 0.5 days |
| 10 | Tests, CI, installer, docs | EMF (CI scripts), edge-manage-docs, cluster-tests, virtual-edge-node, edge-manage-test-automation | ~25 | 3 days | 1.5 days |
| — | E2E testing and validation | all | — | 5–6 days | 4–5 days |
| **Total** | | **17 repos** | **~145 files** | **~25–30 days** | **~13–16 days** |

### By Team

| Team | Phases | Primary Repos |
|---|---|---|
| Platform | 1, 2, 3, 10 | orch-utils, EMF, infra-onboarding, virtual-edge-node |
| Edge Infrastructure | 4 | edge-node-agents, infra-charts |
| UI | 5 | orch-ui |
| CLI | 6 | orch-cli |
| Cluster | 7 (*future*) | cluster-connect-gateway, cluster-manager, cluster-api-provider-intel |
| App Orchestration | 8 (*future*) | app-orch-catalog, app-orch-deployment |
| Observability | 9 (*future*) | o11y-charts, o11y-alerting-monitor |
| QA / Test Automation | 10 | edge-manage-test-automation, cluster-tests |

### Critical Path

```
Phase 1 (Traefik routing) ──→ Phase 2 (Keycloak) ──→ Phase 3 (Edge provisioning)
                          ├──→ Phase 5 (Web UI)
                          ├──→ Phase 6 (CLI)
                          ├──→ Phase 7 (Cluster — future)
                          ├──→ Phase 8 (App orch — future)
                          └──→ Phase 9 (Observability — future)
                                                   ──→ Phase 4 (Edge agents, depends on 3)
                                                   ──→ Phase 10 (Installer + docs, last)
```

Phases 5–9 can proceed in parallel once Phase 1 is complete. Phase 4 depends on
Phase 3 (the ConfigMap changes). Phase 10 is last. Phases 7–9 are future work
(not enabled in the helmfile EIM profile).

---

## 19. Repository Audit Status

### Fully Analyzed (29 repos)

All 29 repositories in the workspace have been analyzed:

**Require changes (17):** orch-utils, edge-manageability-framework, infra-charts,
edge-node-agents, infra-onboarding, orch-ui, orch-cli, app-orch-deployment,
app-orch-catalog, o11y-charts, o11y-alerting-monitor, cluster-connect-gateway,
cluster-manager, cluster-api-provider-intel, cluster-tests, virtual-edge-node,
edge-manage-test-automation

**No runtime changes needed (12):** infra-core, infra-managers, infra-external, orch-library,
orch-metadata-broker, app-orch-tenant-controller, o11y-tenant-controller,
cluster-extensions, trusted-compute, orch-ci, scorch, edge-manage-docs (content updates
in Phase 10 but no runtime code; some have test fixtures to clean up — see Section 2)

### Not Available for Analysis

| Repository | Likely Impact | What to Check |
|---|---|---|
| docs-ui | LOW | Static site; may have hardcoded links |
| web-ui-root (if separate from orch-ui) | LOW | Redirect configuration |
| Release service / token-fs backend | LOW | URL construction for edge nodes |

---

## Appendix A: ArgoCD-Era References (Superseded by Helmfile)

The following EMF directories and files existed in the ArgoCD-based deployment branch
but have been removed in the helmfile-based branch. They are documented here for
historical reference and in case the ArgoCD branch is still used in parallel.

### A.1 ArgoCD Application Templates (`argocd/applications/custom/`)

These `.tpl` files generated Helm values using `.Values.argo.clusterDomain`. The
helmfile equivalents are `.yaml.gotmpl` files in `helmfile-deploy/post-orch/values/`
using `.Values.clusterDomain`.

| ArgoCD Template | Helmfile Equivalent | Subdomains |
|---|---|---|
| `traefik-extra-objects.tpl` | `traefik-extra-objects.yaml.gotmpl` | fleet, registry-oci, observability-ui/admin, vault, keycloak, cluster-orch-node, logs-node, metrics-node, gitea + CSP/CORS |
| `infra-managers.tpl` | `infra-managers.yaml.gotmpl` | infra-node, update-node, telemetry-node, attest-node |
| `infra-onboarding.tpl` | `infra-onboarding.yaml.gotmpl` | All 17+ edge-node-facing hostnames |
| `web-ui-root.tpl` | `web-ui-root.yaml.gotmpl` | web-ui, root domain, keycloak, api |
| `platform-keycloak.tpl` | `platform-keycloak-realm.yaml.gotmpl` | All OAuth clients |
| `platform-autocert.tpl` | `platform-autocert.yaml.gotmpl` | certDomain |
| `self-signed-cert.tpl` | `self-signed-cert.yaml.gotmpl` | certDomain |
| `edgenode-observability.tpl` | (not present in helmfile) | Grafana root_url, Keycloak auth_url |
| `orchestrator-observability.tpl` | (not present in helmfile) | Grafana root_url, Keycloak auth_url |
| `kube-prometheus-stack.tpl` | (not present in helmfile) | Duplicate Grafana + Keycloak URLs |
| `gitea.tpl` (via `orch-configs/`) | (not present in helmfile) | Gitea DOMAIN, ROOT_URL |

### A.2 Mage Helpers (`mage/`)

Go-based dev tooling with hardcoded subdomain construction:

| File | Lines | Pattern |
|---|---|---|
| `mage/dev_utils.go` | 520, 585 | `"https://keycloak." + serviceDomainWithPort + "/realms/..."` |
| `mage/app.go` | 52 | `"https://api." + domain` |
| `mage/tenant_utils.go` | 481 | `"https://keycloak." + serviceDomainWithPort` |

These do not exist in the helmfile branch.

### A.3 External Router (`tools/router/traefik.template`)

Dev-mode Traefik template with subdomain-based HTTP routes (lines 66-70), domain-rewriting
middleware (lines 93-107), and TCP `HostSNI` enumeration (lines 112-144). Does not exist
in the helmfile branch.

### A.4 Terraform (`terraform/`)

| File | Lines | What |
|---|---|---|
| `terraform/edge-network/variables.tf` | 41-86 | 36-entry `dns_hosts` subdomain inventory. **Pre-existing bug:** line 58 `device-manager-node.onprem` missing `cluster.` prefix. |
| `terraform/orchestrator/variables.tf` | 233-236 | `cluster_domain` default `cluster.onprem` |

These do not exist in the helmfile branch.

### A.5 E2E Tests (`e2e-tests/`)

| File | Lines | What |
|---|---|---|
| `e2e-tests/orchestrator/orchestrator_test.go` | 333-348 | `https://vnc.DOMAIN`, `https://web-ui.DOMAIN` URL construction |
| `e2e-tests/orchestrator/orchestrator_test.go` | 992-993 | Hardcoded CSP expectations with 9 subdomain references |

These do not exist in the helmfile branch.

### A.6 CoreDNS Dev/Test Configs (`node/`)

| File | Lines | What |
|---|---|---|
| `node/kind/coredns-config-map.template` | 23, 38-41 | CoreDNS zone file with `fleet.kind.internal`, `app-orch.kind.internal` A records |
| `node/capi/coredns-config.yaml` | 37-40 | CoreDNS hosts plugin for `connect-gateway.kind.internal`, `fleet.kind.internal` |

These do not exist in the helmfile branch.

### A.7 On-Prem Installers (`on-prem-installers/`)

`on-prem-installers/onprem/onprem.env:39` — `CLUSTER_DOMAIN=cluster.onprem` and
`generate_cluster_yaml.sh`. Replaced by `helmfile-deploy/post-orch/post-orch.env` and
the helmfile environment templates.
