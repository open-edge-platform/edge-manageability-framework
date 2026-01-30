# Design Proposal: Scenario-Specific Northbound APIs and CLI Commands for EIM Decomposition

Author(s) Edge Infrastructure Manager Team

Last updated: 30/01/26

## Abstract

In the context of EIM decomposition, the North Bound API service should be treated as an
independent interchangeable module.
The [EIM proposal for modular decomposition](https://github.com/open-edge-platform/edge-manageability-framework/blob/main/design-proposals/eim-modular-decomposition.md)
calls out a need for exposing both a full set of EIM APIs, and a need for exposing only a subset of EIM API
as required by individual workflows taking advantage of a modular architecture.
This proposal explores how the exposed APIs can be decomposed
and adjusted to reflect only the supported EIM services per particular scenario.
It defines how different scenarios can be supported by API versions that match only the services
and features required per scenario, while keeping the full API support in place.

## Background and Context

### Levels of API Decomposition and Key Questions

There are multiple levels of APIs currently available within EMF, with individual specs available for
each domain in
[orch-utils](https://github.com/open-edge-platform/orch-utils/tree/main/tenancy-api-mapping/openapispecs/generated).

The list of domain APIs includes:

- Catalog and Catalog utilities APIs
- App deployment manager and app resource manager APIs
- Cluster APIs
- EIM APIs
- Alert Monitoring APIs
- MPS and RPS APIs
- Metadata broker and Tenancy APIs

There are two levels to the API decomposition:

- **Cross-domain decomposition**: Separation of the above domain-level APIs
(e.g., only exposing EIM APIs - without Cluster APIs, App Orchestrator APIs and others).
- **Intra-domain decomposition**: Separation within a domain (e.g., at the EIM domain level,
where the overall set of APIs may include onboarding/provisioning/Day 2 APIs,
but another workflow may support only onboarding/provisioning without Day 2 support).

The following questions must be answered and investigated:

- **Q**: How is the API service built currently?
  - **Ans**: The protobuf definition file is used to generate the REST API spec, and go code for the REST to gRPC gateway,
  gRPC client and server.
  See [how NB API is currently built](#how-it-is-built)
- **Q**: What level of decomposition is needed for the required workflows?
  - **Ans**: See the [scenarios descriptions](#scenarios-to-be-supported-by-the-initial-decomposition)
- **Q**: How to decompose APIs at the domain level?
  - **Ans**: Domain-level decomposition is achieved through independent service deployment. Each domain
  (EIM, Cluster Orchestrator, App Orchestrator, etc.) exposes its own API service deployed as a separate microservice with
  its own Helm chart and ArgoCD application.
- **Q**: How to decompose APIs within the domain level?
  - **Ans**: Only selected API services will get their handlers registered.
- **Q**: How to build various API service versions as per desired workflows using the modular APIs?
  - **Ans**: apiv2 will be always built to support the full set of EIM NB APIs.
- **Q**: How to deliver the various API service versions as per desired workflows?
  - **Ans**: the workflow will be set through the config file at apiv2 initialization.
- **Q**: How to expose the list of available APIs for client consumption (orch-cli)?
  - **Ans**: The REST client for orch-cli will be always built from the REST API spec including all EIM NB APIs.

### Scenarios to be Supported by the Initial Decomposition

Currently planned decomposition tasks is focused on the EIM layer. The following is the list of deployment scenarios:

- **Full EMF** - Full EMF supporting all existing levels of APIs.
- **EIM Only** - EIM installed on its own, supports only the existing EIM NB APIs.
- **EIM vPRO Only** - EIM installed on its own, supporting only the EIM NB APIs required to support vPRO use cases.
- **EIM without Observability** - EIM installed on its own, supporting all NB EIM APIs except
the APIs related to observability.

### EIM API Service (apiv2)

In Edge Infrastructure Manager (EIM) the apiv2 service represents the North Bound API service that exposes
the EIM operations to the end user, who uses Web UI, Orch-CLI or direct API calls. Currently,
the end user is not allowed to call the EIM APIs directly. The API calls reach first the API gateway, external
to EIM (Traefik gateway), they are mapped to EIM internal API endpoints and passed to EIM.

**Note**: The current mapping of external APIs to internal APIs is 1:1, with no direct mapping to SB APIs.
The mapping is going to be removed so user can call internal APIs directly.
The API service communicates with Inventory via gRPC, which then manages the SB API interactions.

**Apiv2** is just one of EIM Resource Managers that talk to one EIM internal component - the Inventory - over gRPC.
Similar to other RMs, it updates status of the Inventory resources and retrieves their status allowing user
performing operations on the EIM resources for manipulating Edge Nodes.
In EMF 2025.2, the apiv2 service is deployed via a helm chart deployed by Argo CD as one of its applications.
The apiv2 service is deployed as a container using the apiv2 container image.

#### How It Is Built

Currently, apiv2 (infra-core repository) holds the definition of REST API services in protocol buffer files
(`api/proto/`) and uses protoc-gen-connect-openapi to generate the OpenAPI spec - openapi.yaml.

The input to protoc-gen-connect-openapi comes from:

- `api/proto/services` directory - one file (services.proto) containing API operations on
all the available resources (Service Layer).
- `api/proto/resources` directory - multiple files with data models - separate file with data
model per single inventory resource.

Protoc-gen-connect-openapi is the tool that is indirectly used to build the openapi spec.
It is configured as a plugin within buf (buf.gen.yaml).

**Buf** is a replacement for protoc (the standard protocol buffers compiler). It makes working with
.proto files easier as it replaces messy protoc commands with clean config file.
It is an all-in-one tool as it provides compiling, linting, breaking change detection, and dependency management.

In infra-core/apiv2, **buf generate** command is executed within the **generate** or
**buf-gen** make target to generate the OpenAPI 3.0 spec directly from .proto files.

Key Items:

- Input: api/proto/**/*.proto
- Config: buf.gen.yaml, buf.work.yaml, buf.yaml
- Output: openapi.yaml
- Tool: protoc-gen-connect-openapi

Based on the provided protobuf definitions, buf also generates:

- The Go code ( Go structs, gRPC clients/services) (`internal/pbapi`).
- gRPC gateway: REST to gRPC proxy code - HTTP handlers that proxy REST calls to gRPC (`internal/pbapi/**/*.pb.gw.go`).
- Documentation (`docs/proto.md`).

## Decomposing the API Service - Investigation

An investigation needs to be conducted into how the API service can be decomposed to be rebuilt as various
flavors of the same API service providing different sets of APIs.

**Design Principles:**

1. **Single Source of Truth**: The total set of APIs serves as the main source of the API service,
This makes maintenance simple and centralized.

2. **Domain-Level Decomposition**: The API service should be decomposed at the domain level,
meaning that all domains or a subset of domains should be available as part of the EMF.
   - At this level, APIs are already decomposed/modular and deployed as separate services
   (e.g., EIM APIs, Cluster APIs, App Orchestrator APIs).
   - **For EIM-focused scenarios**: Only the EIM domain APIs would be included.

3. **Intra-Domain Decomposition**: The API service should be decomposed within the domain level, meaning
that only a subset of available APIs may need to be released and/or exposed at the API service level.
   - **Example**: Within the EIM domain, we may not want to expose Day 2 functionality for some workflows,
   even though Day 2 operations are part of the full EIM OpenAPI spec.
   - This allows workflows focused on onboarding/provisioning to omit upgrade, maintenance, and troubleshooting APIs.

4. **Resource-Level Decomposition**: The API service may also need to be decomposed at the individual internal service level.
   - **Example**: Host resource might need different data models across use cases.
   - **Note**: This would require separate data models and may increase complexity significantly.

The following are the investigated options to decomposing or exposing subsets of APIs.

- ~~API Gateway that would only expose certain endpoints to user~~ -
It does not actually solve the problem of releasing only specific flavours of EMF.
- ~~Maintain multiple OpenAPI specification~~ - While possible to create multiple OpenAPI specs,
the maintenance of same APIs across specs will be a large burden.
- ~~Authentication & Authorization Based Filtering~~ - We do not control the
end users of the EMF, and we want to provide tailored modular product for each workflow.
- ~~API Versioning strategy~~ - Creating different API versions for each use-case means too much overhead,
 similar to maintaining multiple OpenAPI specs.
- ~~Proxy/Middleware Layer~~ - Similar to API Gateway, does not fit our use cases.
- ~~OpenAPI Spec Manipulation~~ - This approach uses OpenAPI's extension mechanism (properties starting with x-)
to add metadata that describes which audiences, use cases, or clients should have access to specific endpoints,
operations, or schemas. This approach is worth investigating to see if it can give us the automated approach for
creating individual OpenAPI specs for workflows based on labels. (Update: not valid as it was decided that the openapi
spec will be always generated for the full EI API set only.)
- ~~Break the protobuf definition file `services.proto` into multiple files—one per service~~ -
Use buf to select services based on scenario manifests, which would generate scenario-specific API specs.
(Update: not valid as it was decided that the openapi spec will be always generated for the full EIM.)
- **Selective Handler Registration (Selected Approach)** - Generate the complete REST API specification
supporting all EIM NB APIs, but register only the scenario-specific gRPC service handlers within the apiv2
gRPC server at runtime.

### Proposal

#### Build and Release Strategy for the Decomposed API Service Module

Build the EIM API Service per Scenario

- The protobuf definitions in `apiv2/proto` remain unchanged.
- The code generation process for protobuf definitions remains unchanged:
  - The `openapi.yaml` specification will continue to include all EIM API services.
  - All generated Go code will support the complete set of EIM NB API services.
- Scenario manifests will define the subset of API services supported for each scenario.
- The `make generate` target will parse these manifests to generate Go code mappings
between scenario names and their required API services.
- The apiv2 application will accept a `scenario` argument specifying the chosen EIM scenario.
This argument will be supplied via the Helm chart's `scenario` value.

Recommended Release Approach:

- Build and release a single apiv2 container image that will support all scenarios.
- Single Helm chart for all scenarios will use a specific value to set additional apiv2 flag to choose scenario.
- Argo profiles can specify different scenarios - depending on the profiles
enabled (e.g., `orch-configs/profiles/eim-only-vpro.yaml` it sets `eimScenario: vpro`).

#### Define Scenario Manifests

Maintain scenario manifests that list the API services supported by each scenario.
Scenario manifest files will be kept in `infra-core/apiv2`. The following are the examples of the manifests:

```yaml
   # manifests/fulleim.yaml
   name: fulleim
   description: All EIM services
   services:
   - HostService
   - InstanceService
   - OSUpdateRunService
   (..)

   # manifests/vpro.yaml
   name: vpro
   description: EIM vPRO only
   services:
   - host
```

Why manifest files:

- Manifest files content serves as configuration consumed by the apiv2 application at initialization.
- Manifests define which gRPC service handlers should be registered within apiv2 for each scenario.
- Non-developers can easily locate, read, and modify manifests without navigating complex codebases.
- Service names in manifests can be validated against handlers auto-generated by buf from proto definitions.
- Manifests are tracked in the git repository, providing full change history.

#### Modify REST-gRPC Gateway Implementation

The apiv2 proxy server (`internal/proxy/server.go`) for translating the REST calls to gRPC
requests will register only the handlers related to the API services to be supported by configured scenario.

#### Modify gRPC Server Implementation

The apiv2 gRPC server (`internal/server/server.go`) will register only the gRPC service handlers related to
the API services supported by the configured scenario.

### Consuming the Scenario Specific APIs from the CLI

EMF will provide a service that communicates which endpoints/APIs are
currently supported by the deployed API service - Component Status Service.
Proposed in [Design Proposal: Orchestrator Component Status Service](https://github.com/open-edge-platform/edge-manageability-framework/blob/main/design-proposals/platform-component-status-service.md).
Development of such service is outside of this ADR's scope.

#### CLI Workflow

1. **Build**: CLI is built based on the full REST API spec.
2. **Capability Discovery on Login**: The CLI queries the new capabilities service endpoint, upon user login,
to request API capability information.
3. **Configuration Caching**: The CLI saves the supported API configuration locally.
4. **Command Validation**: Before executing commands, the CLI checks the cached configuration and executes
only the commands supported by the currently deployed scenario.
5. **Error Handling**:
   - For CLI commands: Display user-friendly error message.
   - For direct curl calls: API returns HTTP 404 (endpoint not found).

## Summary of Action Items

### 1. Traefik Gateway Compatibility

- Traefik gateway will be removed for all workflows. NB API calls will access EIM endpoints directly.
Investigate the impact.

### 2. Scenario Definition and API Mapping

- Define all supported scenarios:
  - Full EIM
  - EIM vPRO
  - Full EIM without observability
- For each scenario, document:
  - Required services (which resource managers are needed)
  - Required API endpoints (which operations are exposed)
  - Deployment configuration (Helm values, profiles)

### 3. Data Model Changes

- Collaborate with teams/ADR owners to establish:
  - Required changes at Inventory level
  - Impact on APIs from Inventory level changes

## Summary of Current Requirements

- Provide scenario-based support for EIM NB API sets (both full and subsets).
- Preserve API compatibility with Inventory.
- Maintain a single source of truth for API definitions.
- Deliver apiv2 service that supports a particular EIM scenario based on
the configuration provided at initialization time.
- Keep CLI operable against any scenario via discovery, caching, and command validation.
- Provide error handling for missing APIs per scenario.
- Support Helm-driven configuration (scenario selection).
- Support API selection per scenario through Mage/ArgoCD.

## Rationale

The approach aims to narrow the operational APIs surface to the specific scenarios being targeted,
while ensuring the full EMF remains available for deployments.
The proposed solution to APIs decomposition enables incremental decomposition that can be adopted
progressively without breaking existing integrations or workflows.

## Investigation Needed

The following investigation tasks will drive validation of the decomposition approach:

1. Validate feasibility of splitting services.proto and generating per-scenario specs.
via buf/protoc-gen-connect-openapi. (dropped)
2. Evaluate Inventory data model variations per scenario.
3. Verify impact of **1** and **2** on gRPC gateway generation and handler registration per scenario.
4. Validate Argo CD application configs or Mage targets for scenario-specific deployments.

## Implementation Plan for Orch CLI

1. Add login-time scenario discovery: retrieve scenario supported APIs from the new service.
2. Cache discovered capabilities in orch-cli config.
3. Validate user commands against supported APIs.
4. Implement error handling for unsupported APIs.
5. Adjust help to hide unsupported commands/options.
6. Define E2E tests targeting all scenarios.

## Implementation Plan for EIM API

1. ~~Restructure Proto Files. Split monolithic `services.proto` into service-scoped folders~~
(dropped based on inventigation).
2. Create Scenario Manifests.
3. Implement manifest parsing logic.
4. Update Makefile: modify the `generate` target to parse manifests and generate Go mappings.
5. Modify apiv2 proxy and server implementation to register API service handlers based on the chosen scenario.
6. Update apiv2 Helm chart to take additional value to select the EIM scenario.
7. ArgoCD Integration - update/provide profiles for known scenarios.

## Test plan

Tests will verify that minimal and full deployments work as expected, that clients can discover
supported features, and that errors are clear.

- CLI integration with new service: CLI can discover supported services, absence returns descriptive messages.
- CLI E2E: Login discovery, caching, command blocking, error messaging.
- Deployment E2E: Deploy each scenario via mage and verify that expected endpoints exist and work.
- Regression: Verify the full EMF scenario behaves identically to pre-decomposition.

## Open Issues

- **Q**: Traefik gateway removal and impacts.
  - **Ans**: No impact on the API decomposition tasks.
- **Q**: What happens when the service does not exist and CLI expects it to exist?
  - **Ans**: CLI displays an error.
- **Q**: Are there any changes on the Inventory level in regards to scenario definitions?
(NB APIs should be aligned).
  - **Ans**: No
- **Q**: How will managing apiv2 image version used by infra-core argo application look on
the deployment level?
  - **Ans**: One docker image covers all scenarios.
- **Q**: How scenario deployment through ArgoCD will look like?
  - **Ans**: Scenarios other than the full EIM will require specifying
  the corresponding profile in the deployment configuration.
- **Q**: What will be the image naming convention (per scenario)?
  - **Ans**: No change - one docker image covers all scenarios.
