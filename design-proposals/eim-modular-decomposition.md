# Design Proposal: Edge Infrastructure Manager Modular Decomposition

Author(s): 

Last updated: 2025-09-29

## Abstract

Edge Infrastructure Manager (EIM) today ships as an integrated collection of services that are deployed together by Argo CD. Customers have asked for the ability to consume only the subsets of functionality they need—such as device onboarding or out-of-band device management—without inheriting the full solution footprint. This proposal defines how to decompose EIM into modular building blocks with clear consumables (Helm charts, container images, APIs, scripts) while still enabling a full-stack deployment for customers that want the entire framework.

## Proposal

### Scope

* The proposal covers only the Edge Infrastructure Manager domain of the Edge Manageability Framework.
* Edge Cluster Orchestration, Edge Application Orchestration, UI, CLI, Observability, and Platform Services are included only where they integrate with EIM modules (for example, shared identity or telemetry connectors).
* Changes are limited to packaging, deployment flows, API surfacing, and build processes; no core business logic changes are in-scope unless explicitly required to achieve modular boundaries.

### Objectives and Requirements

1. **Modular consumption** – Customers must be able to deploy the whole EIM stack or any subset of modules (for example, Device Onboarding, vPro device management) with minimal dependency drag.
2. **Clear consumables** – Each module publishes:
   * A Helm chart (with documented values, dependencies, and upgrade paths).
   * Versioned container images stored in the public registry.
   * API specifications (OpenAPI) and usage samples.
   * Optional scripts or Terraform modules that automate prerequisite configuration.
3. **Build flexibility** – CI/CD pipelines must produce artifacts for the whole stack and for each module. Consumers can build locally or pull pre-built images.
4. **Operational coherence** – Observability, IAM, and shared services must remain pluggable so that partial deployments still receive security and monitoring coverage.
5. **Backward compatibility** – Existing full-stack deployment flows continue to work (Argo Application of Applications) with new composable sub-charts.

### Definition of Decomposition

Decomposition in this context is the process of slicing EIM into bounded domains that expose stable interfaces and can be built, deployed, and life-cycled independently. Each module is defined by:

* A clear domain responsibility (e.g., Inventory, Resource Managers, Onboarding, Device Management Toolkit).
* Explicit upstream and downstream dependencies declared via Helm chart requirements, API contracts, and event topics.
* A versioned interface surface (OpenAPI, gRPC, message schemas) that allows consumers to integrate without depending on internal implementation details.
* Independent release cadence that can iterate without forcing upgrades in sibling modules, provided interface contracts are respected.

The modular blueprint introduces three tiers:

1. **Foundational services** – Identity, tenancy, inventory database, common event bus.
2. **Service bundles** – Device Onboarding, Resource Management, Observability exporters, Device Management Toolkit.
3. **Integration adapters** – External integrations (e.g., infra-external, partner connectors) that can be plugged in based on customer needs.

### Architectural Design Pattern

The proposal adopts a **Domain-Driven, Helm-Packaged Microservice Mesh** pattern:

* **Domain-driven design (DDD)** provides bounded contexts that map to Helm sub-charts and release artifacts.
* **Microservice mesh with sidecar-enabled observability** ensures each module can be composed via service discovery and policy without tight coupling.
* **Strangler Fig modernization** approach is used to gradually peel existing monolithic Helm definitions into independent sub-charts while keeping the legacy entry points alive until migration is complete.

### Reference Solutions in the Industry

* **Red Hat Advanced Cluster Management (RHACM)** exposes modular operators (cluster lifecycle, application lifecycle, policy) packaged as discrete Helm/OLM operators, enabling customers to install only what they need.
* **VMware Edge Compute Stack** provides optional services (device onboarding, secure access, observability) that can be deployed individually via Helm charts and APIs.
* **Azure Arc-enabled services** shows a pattern where core control plane is optional and capabilities such as Kubernetes configuration, data services, or VM management can be onboarded independently.

These solutions demonstrate that modular edge management platforms rely on clear packaging, API-first integration, and layered observability—the same principles applied here.

### Modular Packaging Model

* **Helm chart hierarchy**
  * `eim-core` (inventory API, tenancy controller, identity integration).
  * `eim-resource-managers` (resource-specific controllers, agent orchestration).
  * `eim-onboarding` (remote onboarding services, DKAM, Tinkerbell workflows).
  * `eim-device-management-toolkit` (vPro/AMT services and gateway).
  * `eim-observability-exporters` (inventory exporter, custom metrics, alert rules).
  * `eim-foundation` (shared postgres/redis, IAM sidecars) – optional if supplied externally.
  * `eim-suite` (meta-chart that depends on all of the above to deliver the full stack).
* **Container images**
  * Each module houses its own `Dockerfile` and GitHub Actions workflow to publish tagged images.
  * Images include SBOMs and signatures to satisfy supply chain requirements.
* **APIs and SDKs**
  * OpenAPI specs remain in `orch-utils`. Each module references the relevant spec in its chart README and provides client snippets.
  * Generate language bindings (Go, Python, TypeScript) during CI to accelerate integration.
* **Automation scripts**
  * `scripts/eim/<module>/install.ps1|sh` for day-0 setup.
  * Terraform modules for cloud prerequisites (IAM roles, load balancers) embedded in the module repository or published to the Terraform registry.

### Build and Release Strategy

1. **Repo alignment**
   * Continue using existing repositories (`infra-core`, `infra-managers`, etc.) but add module-specific `helm/` directories hosting the decomposed charts.
   * `infra-charts` becomes the authoritative aggregation point; each module chart is managed there and versioned via semantic release.
2. **CI pipeline**
   * Multi-stage pipeline generates container images, runs integration tests, and packages Helm charts.
   * Matrix builds allow `--module=<name>` to build and test individual modules, while `--module=all` produces the full suite.
3. **Artifact publication**
   * Helm charts pushed to OCI registry `ghcr.io/open-edge-platform/eim/<module>`.
   * Images pushed to `ghcr.io/open-edge-platform/eim/<module-service>`.
   * Release notes list module versions, dependencies, and migration steps.
4. **Argo CD integration**
   * Introduce Argo Application sets per module, referencing the new charts.
   * The existing umbrella application (`edge-manageability-framework`) depends on module apps; customers can disable modules by omitting specific applications in their GitOps repo.

### Customer Consumption Flows

* **Full Stack** – Install `eim-suite` meta-chart via Argo CD for a turn-key deployment; all modules enabled with default profiles.
* **Device Onboarding Only** – Deploy `eim-onboarding` chart; optionally depend on `eim-foundation` for shared services or point to existing IAM/DB endpoints via Helm values.
* **Out-of-band Device Management** – Deploy `eim-device-management-toolkit` plus `eim-core` (for tenant APIs) and the relevant resource manager connectors.
* **Custom bundle** – Compose desired module charts in a customer-owned GitOps repo, leveraging documented dependencies and values.

## Rationale

### Alternative 1 – Maintain monolithic chart

* **Pros**: Minimal change, easy to manage version matrix.
* **Cons**: Customers cannot consume subsets; upgrades require synchronized releases; difficult to scale contributor teams.

### Alternative 2 – Break into separate repositories per module

* **Pros**: Strong isolation, independent lifecycles.
* **Cons**: Increased repo sprawl, duplicated CI/CD infrastructure, harder to coordinate cross-module releases.

### Chosen Approach Benefits

* Balanced modularity with centralized governance through `infra-charts` and `edge-manageability-framework`.
* Reuses existing GitOps patterns (Argo CD Application of Applications) with minimal disruption.
* Enables customer choice while keeping shared observability and IAM pluggable.

## Affected Components and Teams

* `infra-core`, `infra-managers`, `infra-onboarding`, `infra-external`, `infra-charts` repositories.
* Edge Manageability GitOps configurations (Argo CD apps, Helm charts).
* Release Engineering (new pipelines and artifact publishing).
* Developer Experience (documentation, SDK generation).
* Observability and IAM teams (ensuring optional integrations remain supported).

## Implementation Plan

| Phase | Timeline (est.) | Milestones |
|-------|-----------------|------------|
| Phase 0 – Planning & Inventory | Q4 2025 | Identify module boundaries, audit dependencies, document APIs and helm values per service. |
| Phase 1 – Chart Extraction | Q1 2026 | Extract module charts into `infra-charts`; create CI for module builds; produce alpha releases of `eim-core`, `eim-onboarding`, `eim-device-management-toolkit`. |
| Phase 2 – Artifact & API Hardening | Q2 2026 | Publish versioned APIs, SBOMs, signed images; update docs; release beta for customer pilots. |
| Phase 3 – Production Rollout | Q3 2026 | General availability of modular charts; deliver `eim-suite` meta-chart; deprecate legacy monolithic chart with migration guide. |
| Phase 4 – Optimization | Q4 2026 | Automate dependency validation, implement per-module usage analytics, iterate on reference automation scripts. |

## Open Issues

* **Dependency Minimization** – Additional work is required to refactor runtime assumptions (for example, shared PostgreSQL schemas) so modules can operate with external managed services.
* **Licensing Attribution** – Helm chart decomposition must ensure third-party license notices remain accurate when modules are installed standalone.
* **Version Compatibility Matrix** – Need tooling to surface compatible module version sets (e.g., `eim-core` v1.3 works with `eim-onboarding` v1.2+).
* **Edge Agent Coordination** – Define how edge node agent versions map to modular Resource Manager releases to avoid drift.
