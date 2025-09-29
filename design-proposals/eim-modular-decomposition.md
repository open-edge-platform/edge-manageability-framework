# Design Proposal: Edge Infrastructure Manager Modular Decomposition

Author(s): Edge Manageability Architecture Team

Last updated: 2025-09-29

## Abstract

Edge Infrastructure Manager (EIM) today ships as an integrated collection of services that are deployed together by Argo CD. Customers have asked for the ability to consume only the subsets of functionality they need—such as device onboarding or out-of-band device management—without inheriting the full solution footprint. This proposal defines how to decompose EIM into modular building blocks with clear consumables (Helm charts, container images, APIs, scripts) while still enabling a full-stack deployment for customers that want the entire framework.

## Background and Context

Edge Manageability Framework (EMF) spans seven domains that are orchestrated through Argo CD and Helm charts: Edge Infrastructure Manager, Edge Cluster Orchestration, Edge Application Orchestration, UI, CLI, Observability, and Platform Services. Each domain is composed of microservices that are deployed via a GitOps flow rooted in this repository. Within that ecosystem, EIM focuses on policy-driven lifecycle management of distributed edge fleets and collaborates with adjacent domains for shared services such as identity, telemetry, and higher-layer orchestration.

Key API specifications are published in the `orch-utils` repository:

* EIM northbound APIs: <https://github.com/open-edge-platform/orch-utils/blob/main/tenancy-api-mapping/openapispecs/generated/amc-infra-core-edge-infrastructure-manager-openapi-all.yaml>
* Edge Cluster Orchestration APIs: <https://github.com/open-edge-platform/orch-utils/blob/main/tenancy-api-mapping/openapispecs/generated/amc-cluster-manager-openapi.yaml>
* Edge Application Orchestration APIs: <https://github.com/open-edge-platform/orch-utils/blob/main/tenancy-api-mapping/openapispecs/generated/amc-app-orch-deployment-app-deployment-manager-openapi.yaml>

The EIM software supply chain spans multiple repositories—`infra-core`, `infra-managers`, `infra-onboarding`, `infra-external`, and `infra-charts`—that jointly deliver the APIs, resource managers, onboarding flows, and Helm packaging required to operate the framework.

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

### Consumer Personas and Reference Workflows

#### Independent Software Vendor / OS Vendor

* Fully automated, dynamic out-of-band device management using Intel® vPro™ AMT and ISM to manage fleets of edge devices.
* Edge node hardware and software observability surfacing Intel silicon metrics for higher-level management stacks.
* Secure device onboarding and OS provisioning with reusable scripts and Helm values.
* Day-two device lifecycle management covering immutable OS updates, firmware updates, and CVE remediation.
* Day-one CPU, GPU, and NPU configuration workflows that expose resource partitioning APIs to upper-layer applications.
* Device configuration and northbound APIs that integrate with trusted compute, cluster orchestration, and customer control planes.

#### Original Equipment Manufacturer

* Fully automated device onboarding, OS provisioning, Kubernetes deployment, and add-on installation at fleet scale in factories or warehouses, with hooks for QA validation.
* Automated device upgrades (OS and firmware) for field deployments with staged rollouts and rollback controls.
* Automated device activation and out-of-band management using Intel® vPro™ AMT and ISM across large fleets.

#### End Customer / Systems Integrator

* End-to-end multi-tenant solution covering Day 0 (onboarding and provisioning), Day 1 (configuration and operations), and Day 2 (lifecycle management) that can deploy on-premises or in the cloud.
* Use EIM as a validation bridge for customer orchestration layers against new Intel CPU and GPU platforms prior to production rollout.

## Rationale

### Alternative 1 – Maintain monolithic chart

This option keeps the current `infra-charts` umbrella chart intact and continues shipping all services together. EIM remains a single Argo CD Application with hard-wired dependencies.

* **Pros**
   * Minimal change to existing GitOps repositories and automation.
   * Straightforward version matrix—one chart version maps to one platform release.
   * Existing documentation and support processes remain unchanged.
* **Cons**
   * Customers who only need Device Onboarding or vPro tooling must install the entire stack (PostgreSQL, resource managers, observability exporters, etc.).
   * Upgrades force synchronized downtime windows across all services and increase risk of regression in unrelated components.
   * Contributor teams cannot iterate independently; even a small fix in `infra-onboarding` requires full regression testing of all modules.

**Example:** An ISV seeking only the vPro Device Management Toolkit must deploy the entire suite (inventory, onboarding, resource managers, telemetry) even if those services conflict with their existing stack, leading to duplicated infrastructure and operational overhead.

### Alternative 2 – Break into separate repositories per module

This approach would split each EIM capability into its own Git repository (for example, `infra-onboarding`, `infra-vpro`, `infra-resource-managers`) with discrete Helm charts, pipelines, and release schedules.

* **Pros**
   * Strong isolation between modules—teams can release without cross-repo coordination.
   * Each repository can tailor CI pipelines, code owners, and branching models to its needs.
   * Easier for external contributors to focus on a single capability without onboarding to the whole stack.
* **Cons**
   * Rapid repository proliferation increases maintenance cost (CI runners, issue tracking, security scanning).
   * Coordinating platform-wide releases becomes harder; consumers must stitch together compatible versions manually.
   * Shared assets (SDKs, API specs, documentation) risk divergence or duplication across repos.

**Example:** Shipping `eim-onboarding` from a dedicated repo accelerates onboarding innovation, but release engineering must coordinate simultaneous tags across five repositories when producing a full-suite build—raising risk of mismatched versions in customer environments.

### Alternative Comparison

| Dimension | Alternative 1: Monolithic Chart | Alternative 2: Separate Repos per Module |
|-----------|---------------------------------|-------------------------------------------|
| Customer modular adoption | Very limited—entire suite required | High—modules can be consumed independently |
| Release coordination | Centralized, simple | Decentralized, complex |
| Operational overhead | Single pipeline, lower infra cost | Multiple pipelines, higher infra cost |
| Developer autonomy | Low—shared release train | High—repo-level ownership |
| Documentation/SDK maintenance | Unified | Risk of duplication and drift |
| Backward compatibility guarantees | Strong but rigid | Flexible but harder to enforce |

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
