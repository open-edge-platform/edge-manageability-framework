# Design Proposal: DNS Simplification

Author(s): Scott Baker

Last updated: 2026-3-31

Revision: 1.0

## Abstract

The Edge Orchestrator currently requires 35+ DNS records (one per subdomain) to be created
before installation can proceed. This ADR proposes consolidating to a single domain name
with path-based routing, eliminating the DNS setup burden and simplifying deployment.

## Problem Statement

Every EMF deployment requires the operator to create individual DNS records for each
service subdomain. A typical on-prem deployment needs records for `web-ui`, `api`,
`keycloak`, `vault`, `registry-oci`, `observability-ui`, `observability-admin`,
`gitea`, `fleet`, `connect-gateway`, `app-orch`, `app-service-proxy`, `vnc`,
`metadata`, `alerting-monitor`, `docs-ui`, `cluster-management`, and 18+ edge-node-facing
subdomains. All of these resolve to the same IP address (or the same small set of IPs).

This creates several problems:

1. **Deployment friction.** Operators must create 35+ DNS records before installation.
   For on-prem deployments without automated DNS, this means 35+ entries in `/etc/hosts`
   or dnsmasq on every machine that needs to access the orchestrator.

2. **Wildcard DNS as a workaround.** Most deployments use a wildcard record
   (`*.cluster.onprem`) to avoid creating individual records. If a wildcard works for
   everyone, the individual subdomains serve no technical purpose at the DNS layer.

3. **TLS certificate complexity.** Each subdomain either needs its own certificate or
   a wildcard certificate. Wildcard certs are the norm, which again demonstrates that
   the per-subdomain distinction adds no value at the TLS layer.

4. **Edge node configuration burden.** Edge nodes are configured with 15+ hostnames
   for orchestrator services. All resolve to the same Traefik ingress IP.

## Current Subdomain Inventory

### User-Facing (Browser) Subdomains

These subdomains are accessed by operators via web browser or CLI tools.

| Subdomain | Service | Protocol | Notes |
|-----------|---------|----------|-------|
| `web-ui` | Web UI | HTTPS | Main dashboard |
| `api` | API Gateway (nexus-api-gw) | HTTPS + gRPC | Multiplexed via PathPrefix already |
| `keycloak` | Authentication | HTTPS | OAuth/OIDC provider |
| `vault` | Secrets Management | HTTPS | |
| `registry-oci` | Harbor OCI Registry | HTTPS | Docker registry API |
| `observability-ui` | Grafana | HTTPS | |
| `observability-admin` | Grafana Admin | HTTPS | |
| `alerting-monitor` | Alert Manager | HTTPS | |
| `gitea` | Git Server | HTTPS | |
| `app-orch` | App Orchestrator | HTTPS | |
| `app-service-proxy` | App Service Proxy | HTTPS + WSS | |
| `vnc` | Remote Console | WSS | WebSocket for KVM |
| `docs-ui` | Documentation Portal | HTTPS | |
| `cluster-management` | Cluster Management UI | HTTPS | |
| `connect-gateway` | Cluster Connect | WSS | WebSocket gateway |
| `fleet` | Fleet Server | HTTPS | |
| *(root domain)* | Redirect to web-ui | HTTPS | |

### Edge-Node-Facing (gRPC/Machine) Subdomains

These subdomains are accessed by edge node agents over gRPC or specialized protocols.

| Subdomain | Service | Protocol | Notes |
|-----------|---------|----------|-------|
| `infra-node` | Host Manager | gRPC | |
| `update-node` | Maintenance Manager | gRPC | |
| `telemetry-node` | Telemetry Manager | gRPC | |
| `attest-node` | Attestation Manager | gRPC | |
| `cluster-orch-node` | Cluster Orchestrator | gRPC | |
| `onboarding-node` | Onboarding Manager | gRPC | |
| `onboarding-stream` | Onboarding Stream | gRPC | |
| `device-manager-node` | Device Manager | gRPC | |
| `logs-node` | Log Collector | HTTPS | |
| `metrics-node` | Metrics Collector | HTTPS | |
| `release` | Release Service | HTTPS | Package distribution |
| `tinkerbell-server` | Tinkerbell | HTTPS | Provisioning |
| `tinkerbell-haproxy` | Tinkerbell LB | HTTPS | PXE boot |
| `mps` | Managed Presence Server | CIRA (port 4433) | Intel AMT |
| `mps-wss` | MPS WebSocket | WSS | |
| `rps` | Remote Provisioning Server | gRPC | Intel AMT |
| `rps-wss` | RPS WebSocket | WSS | |

## Proposal: Consolidate to a Single Domain

Replace all subdomains with path-based routing under a single domain name.

### Path Mapping

**User-facing services** become path prefixes:

| Current | Proposed | Complexity |
|---------|----------|------------|
| `web-ui.example.com` | `example.com/` | Straightforward |
| `api.example.com` | `example.com/api/` | Already uses PathPrefix |
| `keycloak.example.com` | `example.com/auth/` | Moderate (see below) |
| `vault.example.com` | `example.com/vault/` | Straightforward |
| `registry-oci.example.com` | `example.com/registry/` | Complex (see below) |
| `observability-ui.example.com` | `example.com/grafana/` | Straightforward - Grafana supports root_url |
| `observability-admin.example.com` | `example.com/grafana-admin/` | Straightforward |
| `alerting-monitor.example.com` | `example.com/alerts/` | Straightforward |
| `gitea.example.com` | `example.com/gitea/` | Straightforward - Gitea supports ROOT_URL |
| `app-orch.example.com` | `example.com/app-orch/` | Straightforward |
| `app-service-proxy.example.com` | `example.com/app-proxy/` | Straightforward |
| `vnc.example.com` | `example.com/vnc/` | Straightforward (WebSocket upgrade on path) |
| `docs-ui.example.com` | `example.com/docs/` | Straightforward |
| `cluster-management.example.com` | `example.com/cluster-mgmt/` | Straightforward |
| `connect-gateway.example.com` | `example.com/connect/` | Straightforward (WebSocket upgrade on path) |
| `fleet.example.com` | `example.com/fleet/` | Straightforward |

**Edge-node-facing gRPC services** become path prefixes using gRPC's native path routing:

| Current | Proposed |
|---------|----------|
| `infra-node.example.com` | `example.com` with gRPC service routing |
| `update-node.example.com` | `example.com` with gRPC service routing |
| `telemetry-node.example.com` | `example.com` with gRPC service routing |
| *(all other gRPC services)* | `example.com` with gRPC service routing |

gRPC requests already include a service path (`/package.ServiceName/MethodName`) in every
request. Traefik can route on these paths without needing separate hostnames, provided the
gRPC service names are unique (they are).

**AMT services** (MPS/RPS) are a special case discussed below.

### Services Requiring Detailed Analysis

#### Keycloak - Moderate Complexity

Keycloak is deeply integrated as the OIDC provider. Moving it to a path prefix requires:

- Setting `http-relative-path` to `/auth` (Keycloak supports this natively).
- Updating all OIDC client configurations (`redirectUris`, `rootUrl`) in the realm import.
  There are 15+ Keycloak clients defined in `platform-keycloak.tpl` that reference
  the domain. These would need updated redirect URIs (e.g., `https://example.com/auth/...`
  instead of `https://keycloak.example.com/...`).
- Updating the internal JWKS URL used by all services for JWT validation
  (`keycloakJwksUrl` in `traefik-extra-objects.tpl`). This is currently an internal
  Kubernetes service URL (`http://platform-keycloak.orch-platform.svc`) and would
  not change.
- Updating CORS `allowedOrigins` - currently references `web-ui.example.com` and the
  root domain separately. With a single domain, CORS configuration simplifies.
- Browser cookie domain scoping: with all services on one domain, cookies set by
  Keycloak are automatically available to all services. This is actually a
  **simplification** - the current multi-subdomain setup requires careful cookie
  domain configuration.

**Verdict:** Viable. Keycloak has built-in support for path-based deployment.

#### Harbor OCI Registry - Complex

Harbor exposes a Docker Registry V2 API that Docker and container runtimes expect at
specific paths (`/v2/`). Moving Harbor behind a path prefix is problematic:

- Docker clients expect the registry API at the root path. `docker pull
  example.com/registry/myimage:latest` would not work correctly - Docker interprets
  `registry` as an image path component, not a prefix.
- Harbor's internal routing assumes it owns the root path.
- The Notary and Chartmuseum sub-components add further path conflicts.

**Verdict:** Keep `registry-oci` as a separate subdomain, or use a non-standard port
on the main domain (e.g., `example.com:5443`). Container registries are a known
exception to path-based consolidation. Alternatively, since Harbor is only accessed
by operators (not edge nodes), the single additional DNS record is acceptable.

**Alternative:** Drop harbor from the product and it is no longer an issue. Harbor
is part of the application orchestration layer and is not a priority.

#### MPS/RPS (Intel AMT) - Straightforward

MPS CIRA already uses port 4433, so it naturally differentiates by port and needs
no routing changes - it just uses the single domain on its existing port. The
remaining AMT endpoints are standard protocols:

- `mps` (CIRA, port 4433): Already on its own port. No change needed.
- `mps-wss` (WSS, port 443): Can be path-routed to `example.com/mps/`.
- `rps` (gRPC, port 443): Can use gRPC service path routing.
- `rps-wss` (WSS, port 443): Can be path-routed to `example.com/rps/`.

**Verdict:** Viable. No complexity here.

#### gRPC Services - Straightforward but Requires Agent Changes

All gRPC services (infra-node, update-node, telemetry-node, attest-node,
cluster-orch-node, onboarding-node, onboarding-stream, device-manager-node, rps)
can be consolidated because gRPC routes on `/:authority` and service path.

Traefik already supports gRPC routing with `PathPrefix` rules matching gRPC
service paths. Since each gRPC service has a unique protobuf package and service name,
there are no path collisions.

The change requires updating the `infra-config` ConfigMap
(`infra-onboarding.tpl`) that tells edge nodes which hostnames to connect to.
Instead of 10+ hostnames, edge nodes would receive a single hostname with
different gRPC service paths. The edge node agents must be updated to connect to
the new single hostname.

**Verdict:** Viable. Requires coordinated changes to edge node agent configuration
and the infra-config ConfigMap. This is the highest-impact simplification since it
eliminates 10+ DNS records and simplifies edge node provisioning.

### What Does NOT Change

- **Internal Kubernetes service DNS** (e.g., `platform-keycloak.orch-platform.svc`)
  remains unchanged. These are cluster-internal and not affected by external DNS.
- **Release service** (`registry-rs.edgeorchestration.intel.com`) is an external
  Intel-hosted service, not part of the orchestrator's DNS.

## Impact Assessment

### DNS Records: Before and After

| Scenario | Current | Proposed |
|----------|---------|----------|
| Minimal (vPro profile) | ~20 records | 1 record |
| Full deployment | ~35 records | 1 record |
| Full deployment with Harbor | ~35 records | 2 records (1 + registry-oci) |

### Components Requiring Changes

| Component | Change Required | Effort |
|-----------|----------------|--------|
| Traefik IngressRoute rules | Host() to PathPrefix() | Medium |
| traefik-extra-objects chart | Rewrite routing rules | Medium |
| Keycloak realm configuration | Update redirect URIs and paths | Medium |
| Edge node agent configs (infra-config) | Single hostname instead of many | Low |
| Edge node agents (8+ agents) | Connect to new hostname | Medium |
| Web UI | Update API endpoint URLs | Low |
| CORS configuration | Simplifies (single origin) | Low |
| CSP headers | Simplifies (single origin) | Low |
| Installer scripts (onprem.env) | Remove per-service domain config | Low |
| Documentation | Update all hostname references | Low |
| Harbor/OCI registry | Keep as subdomain (exception) | None |

### Benefits

1. **Deployment setup reduces from 35+ DNS records to 1-2.** On-prem deployments
   need one DNS record (or `/etc/hosts` entry) instead of a wildcard or 35 individual records.

2. **TLS simplifies to a single certificate.** No wildcard cert needed. A standard
   single-domain certificate covers everything.

3. **Edge node provisioning simplifies.** The infra-config ConfigMap shrinks from
   15+ hostnames to one. Edge nodes need to trust one hostname.

4. **CORS and CSP policies simplify.** All services share one origin, eliminating
   cross-origin complexity.

5. **Cookie sharing works automatically.** Authentication cookies from Keycloak are
   available to all services without domain configuration.

6. **Proxy configuration simplifies.** The `no_proxy` list for internal services
   shrinks to one entry.

## Migration Strategy

### Phase 1: Support Both (Non-Breaking)

Add path-based routing alongside existing subdomain routing. Traefik can match on
both `Host(keycloak.example.com)` and `Host(example.com) && PathPrefix(/auth/)`
simultaneously. This allows gradual migration without breaking existing deployments.

### Phase 2: Default to Single Domain

New deployments default to single-domain mode. Subdomain mode remains available
as a configuration option for customers with existing DNS infrastructure.

### Phase 3: Deprecate Subdomain Mode

After one release cycle, subdomain mode is deprecated. The routing configuration
simplifies to path-based only.

## Recommendation

Adopt single-domain path-based routing as the default for new deployments in 2026.2,
with the following exceptions:

- **Harbor OCI registry** retains a dedicated subdomain (`registry-oci`) due to
  Docker registry API constraints.
- **MPS CIRA** shares the main domain but uses port 4433 for protocol differentiation.

This reduces DNS setup from 35+ records to 2 (one for the main domain, one for the
registry), eliminates wildcard certificate requirements, and significantly simplifies
both operator and edge node configuration.

## Open Questions

1. **Should path prefixes be configurable?** For example, allowing operators to change
   `/auth/` to `/keycloak/` if it conflicts with existing infrastructure.

2. **Backward compatibility duration.** How long should subdomain routing remain
   supported alongside path-based routing?

3. **Edge node agent rollout.** Changing the hostnames that edge nodes connect to
   requires updating deployed agents. What is the upgrade path for existing edge nodes?

## Effort Assessment

The majority of this work is mechanical transformation of configuration files and
templates within a single repository. The changes follow repeatable patterns
(hostname-to-path rewrites) across a known set of files.

### Manual Effort Estimate

Without automation, this work would require approximately 2-3 engineer-weeks:

| Task | Effort | Notes |
|------|--------|-------|
| Traefik routing rules (~15 template files) | 2-3 days | Repetitive Host() to PathPrefix() rewrites |
| Keycloak realm configuration | 1-2 days | 15+ OAuth clients with redirect URIs to update |
| infra-config ConfigMap + edge agent configs | 1-2 days | Straightforward but requires cross-repo coordination |
| Web UI API endpoint URLs | 1 day | Depends on how endpoints are centralized |
| Grafana/Gitea/Harbor sub-path configuration | 1 day | Well-documented settings in upstream charts |
| Installer script updates | 0.5 days | Remove per-subdomain config from onprem.env and generate_cluster_yaml.sh |
| CORS, CSP, cookie policy simplification | 0.5 days | Mostly deletion of now-unnecessary config |
| Automated test updates | 1-2 days | Update hostname assertions, fixtures, and test configs |
| End-to-end testing and validation | 2-3 days | Deploy to a real cluster and verify all routing paths |
| Documentation updates | 1-2 days | Deployment guide, API reference, edge node setup docs |

### AI-Assisted Effort Estimate

With AI code assistance (e.g., Claude Code), the implementation portion compresses
significantly because the changes are pattern-based and contained within well-structured
template files:

- **Traefik templates, Keycloak realm, infra-config, CORS/CSP, installer scripts:**
  These are all in this repository and follow clear, repeatable patterns. An AI assistant
  can execute these changes in a single session with human review. This covers roughly
  70-80% of the code changes.

- **Web UI endpoint URLs and upstream chart sub-path settings:** These require some
  exploration to locate the right configuration points, but once found, the changes
  are straightforward. AI can handle this with light guidance.

- **Edge node agent hostname changes:** These live in separate repositories and require
  understanding the agent codebase. AI can make the changes but a developer needs to
  identify the correct repositories and review the results.

- **Automated test updates:** Tests that assert on hostnames, construct URLs, or
  mock routing behavior will need updating. These follow the same hostname-to-path
  pattern as the production code and are well-suited to AI-assisted rewriting.

- **End-to-end testing and validation:** Requires deploying to a real cluster and
  verifying routing, authentication flows, gRPC connectivity, and edge node
  communication. AI cannot substitute for this.

- **Documentation:** Deployment guides, API references, and edge node setup docs
  all reference subdomains. AI can rewrite these but a developer should review
  for accuracy.

| Task | Manual | AI-Assisted |
|------|--------|-------------|
| Configuration and template changes | 5-7 days | 1 day (AI) + 1 day (review) |
| Cross-repo agent changes | 1-2 days | 0.5 days (AI) + 0.5 days (review) |
| Automated test updates | 1-2 days | 0.5 days (AI) + 0.5 days (review) |
| End-to-end testing and validation | 2-3 days | 2-3 days (no change) |
| Documentation | 1-2 days | 0.5 days (AI) + 0.5 days (review) |
| **Total** | **~2.5-3.5 weeks** | **~1-1.5 weeks** |

The bottleneck shifts from writing code to reviewing and testing it. The configuration
and test changes that would take an engineer over a week of tedious editing become a
review exercise.

## Affected Components and Teams

- Platform Team (Traefik routing, installer, Keycloak configuration)
- Edge Infrastructure Team (agent hostname configuration, infra-config)
- UI Team (API endpoint URL updates)
- Documentation Team (hostname reference updates)

## Decision

- Defer until 2026.2
