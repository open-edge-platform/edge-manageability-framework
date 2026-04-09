# Design Proposal: Nexus Tenancy Replacement

Author(s): Scott Baker

Last updated: 4/9/26

## Abstract

The EMF tenancy data model (orgs, folders, projects) is currently implemented
using the Nexus framework: CRDs in Kubernetes etcd, a generated Go SDK, and an
event-driven watcher/acknowledgment protocol coordinated by the tenancy-manager.
Seven tenant controllers subscribe to CRD events to provision resources in
external systems (Keycloak, Harbor, K8s namespaces, inventory, etc.).

This proposal replaces the Nexus-based tenancy implementation with a
Postgres-backed design using the Ent ORM and a new Tenant Manager REST API.
The Nexus SDK, tenancy CRDs, and the watcher protocol are removed from all
components.

This builds on the direction established in the
[Multi-tenancy Simplification](edge-manageability-framework/design-proposals/platform-multi-tenancy-simplification.md)
proposal (Moon, Togashi), which introduced the Tenant Manager REST API and
Traefik IngressRoutes. This proposal extends that work by replacing the
underlying data store and event propagation mechanism.

## Proposal

### Data Store

Replace the Nexus CRD/etcd data model with five Postgres tables managed by
the Ent ORM framework (same pattern as `infra-core/inventory/`):

| Table | Purpose |
| --- | --- |
| `orgs` | Organization with soft-delete (`deleted_at`) |
| `folders` | Org→Folder hierarchy (one "default" folder per org, no CRUD API) |
| `projects` | Folder→Project with soft-delete |
| `tenancy_events` | Event log for org/project create and delete |
| `controller_statuses` | Per-controller, per-resource status tracking |

Hash names are eliminated -- Postgres uses the user-friendly name directly,
with UUID primary keys.

### Event Propagation

Replace the Nexus callback/watcher protocol with a transactional event table
and polling:

1. **Transactional writes:** When the Tenant Manager creates or deletes an org
   or project, it writes the data change and a `tenancy_events` row in the
   same database transaction. If the data committed, the event committed.

2. **Controller polling:** Each controller polls the Tenant Manager REST API
   on startup (replay from current DB state) and then at a regular interval
   (incremental events). Controllers report per-resource status back to the
   Tenant Manager, which derives overall org/project status.

3. **Delete lifecycle:** Controllers delete their `controller_statuses` row
   after successfully processing a delete event (mirroring the current
   `DeleteActiveWatchers()` pattern). The Tenant Manager hard-deletes
   soft-deleted resources when no status rows from registered controllers
   remain.

This approach requires no new infrastructure (uses existing Postgres), no
push-based mechanisms, and is compatible with cloud-hosted databases like
Amazon Aurora (standard SQL only).

### Tenant Manager

The tenancy-manager is rewritten as a standalone HTTP server that:

- Owns the Postgres tenancy schema (sole database accessor)
- Exposes REST endpoints for org/project CRUD (`/v1/orgs`, `/v1/projects`)
- Exposes internal endpoints for controller event polling and status reporting
- Derives resource status from the `controller_statuses` table against a
  Helm-managed config of registered controllers
- Bootstraps default org/project on first startup (absorbing `tenancy-init`)

Controllers interact exclusively via the REST API -- they have no database
dependency and no Ent schema dependency.

### Controller Refactoring

All seven tenant controllers are refactored:

| Controller | Canonical ID | Events |
| --- | --- | --- |
| app-orch | `app-orch-tenant-controller` | project |
| keycloak | `keycloak-tenant-controller` | **org + project** |
| infra-core | `infra-tenant-controller` | project |
| cluster-manager | `cluster-manager` | project |
| app-deployment-mgr | `app-deployment-manager` | project |
| observability | `observability-tenant-controller` | project |
| metadata-broker | `metadata-broker` | project |

Each controller removes its Nexus SDK integration and replaces it with a
shared library (`orch-library/go/pkg/tenancy/`) that implements the
replay-then-poll lifecycle. Business logic (provisioning, cleanup) is
unchanged.

### API Exposure

The Tenant Manager gets a Traefik IngressRoute for external CRUD endpoints.
Internal event/status endpoints are ClusterIP-only. The orch-library project
context middleware is updated to resolve project UUIDs via the Tenant Manager.

### Components Removed

- Tenancy data model CRDs (`orch-utils/tenancy-datamodel/`)
- `tenancy-init` job (`orch-utils/tenancy-init/`)
- Nexus SDK dependency from all seven controllers, the tenancy-manager, and
  the nexus-api-gw

## Rationale

### Why not keep Nexus CRDs?

- Tenancy data in etcd is not backed up alongside other data in Postgres,
  creating a split-brain risk for disaster recovery.
- The Nexus framework adds significant complexity (CRD generation, hash names,
  watcher protocol, finalizer-based deletion) for what is a simple
  hierarchical data model.
- The Nexus SDK is tightly coupled to Kubernetes, making cloud-hosted database
  migration impossible.

### Why not a message queue / event bus?

The requirements explicitly exclude additional infrastructure. Postgres is
already deployed. The transactional event table provides the same durability
guarantees as a queue for this use case (low event volume, seconds-level
latency acceptable).

### Why polling instead of LISTEN/NOTIFY?

LISTEN/NOTIFY is Postgres-specific and not supported by Amazon Aurora or other
cloud database services. Polling at 5-second intervals is negligible overhead
for tenancy operations, which involve slow external provisioning (Harbor,
Keycloak, K8s namespace creation).

### Why Ent ORM?

Already in use in `infra-core/inventory/` with 28 schemas, versioned
migrations, and a store layer. Using the same framework avoids introducing a
second data access pattern.

### Why controllers access the REST API, not Postgres directly?

Avoids database technology coupling, schema dependency proliferation across
seven controllers, and startup ordering issues (controller starting before
migrations run).

## Affected Components and Teams

| Component | Repository | Change |
| --- | --- | --- |
| tenancy-manager | `orch-utils` | Full rewrite (Nexus → HTTP server + Ent) |
| keycloak-tenant-controller | `orch-utils` | Replace Nexus SDK with shared library |
| app-orch-tenant-controller | `app-orch-tenant-controller` | Replace Nexus SDK with shared library |
| infra-core tenant-controller | `infra-core` | Replace Nexus SDK with shared library |
| cluster-manager | `cluster-manager` | Replace Nexus SDK with shared library |
| app-deployment-manager | `app-orch-deployment` | Replace Nexus SDK with shared library |
| observability-tenant-controller | `o11y-tenant-controller` | Replace Nexus SDK with shared library |
| metadata-broker | `orch-metadata-broker` | Replace Nexus SDK with shared library |
| orch-library | `orch-library` | New shared tenancy polling library + middleware update |
| tenancy-datamodel | `orch-utils` | Removed |
| tenancy-init | `orch-utils` | Removed (absorbed by Tenant Manager) |
| CLI | `orch-cli` | Regenerate API client |
| UI | `orch-ui` | Regenerate API client |
| Helm charts | various | Add Ent migrations, update controller config, remove CRDs |

All work to be done by platform team.

## Implementation Plan

This is a big-bang replacement -- all components must be updated together.
The detailed implementation plan, including Ent schemas, REST API contracts,
per-controller migration details, and a 21-step phased implementation order,
is available.

## Open Issues

1. **Existing data migration:** Live deployments have tenancy state in etcd
   CRDs. A one-time migration tool or procedure is needed to seed the Postgres
   tables from the existing CRDs before cutting over.
2. **Nexus API gateway removal scope:** The nexus-api-gw also serves as an API
   remapper for non-tenancy services. Its full removal depends on the separate
   API remapper migration and is out of scope for this proposal.
