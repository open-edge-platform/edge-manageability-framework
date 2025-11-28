# Design Proposal: Exposing only the required North Bound APIs and CLI commands for the workflow as part of EIM decomposition

Author(s) Edge Infrastructure Manager Team

Last updated: 7/11/25

## Abstract

In the context of EIM decomposition, the North Bound API service should be treated as an independent interchangeable module.
The [EIM proposal for modular decomposition](https://github.com/open-edge-platform/edge-manageability-framework/blob/main/design-proposals/eim-modular-decomposition.md) calls out a need for exposing both a full set of EIM APIs, and a need for exposing only a subset of EIM APIs as required by individual workflows taking advantage of a modular architecture. This proposal explores how the exposed APIs can be decomposed and adjusted to reflect only the supported EIM services per particular scenario. It defines how different scenarios can be supported by API versions that match only the services and features required per scenario, while keeping the full API support in place.

## Background and Context

In Edge Infratructure Manager (EIM) the apiv2 service represents the North Bound API service that exposes the EIM operations to the end user, who uses Web UI, Orch-CLI or direct API calls. Currently, the end user is not allowed to call the EIM APIs directly. They API calls reach first the API gateway external to EIM (Traefik gateway) which are mapped to EIM internal API endpoints and passed to EIM.
**Note**: The current mapping of external APIs to internal APIs is 1:1, with no direct mapping to SB APIs. The API service communicates with Inventory via gRPC, which then manages the SB API interactions.

### About APIV2

**Apiv2** is just one of EIM resource managers who talks to one EIM internal component, the Inventory, over gRPC. Same as other RMs it  updates status of resources and retrieves the status allowing user performing operations on the EIM resources for manipulating Edge Nodes.
In EMF 2025.2 the apiv2 service is deployed via a helm chart deployed by Argo CD as one of its applications. The apiv2 service is run and deployed in a container kick-started from the apiv2 service container image.

### How NB API is Currently Built

Currently, apiv2 (infra-core repository) holds the definition of REST API services in protocol buffer files (.proto) and uses protoc-gen-connect-openapi to autogenerate the OpenAPI spec - openapi.yaml.

The input to protoc-gen-connect-openapi comes from:
- `api/proto/services` directory - one file (services.proto) containing API operations on all the available resources (Service Layer)
- `api/proto/resources` directory - multiple files with data models - separate file with data model per single inventory resource

Protoc-gen-connect-openapi is the tool that is indirectly used to build the openapi spec. It is configured as a plugin within buf (buf.gen.yaml). 

### What is Buf

Buf is a replacement for protoc (the standard Protocol Buffers compiler). It makes working with .proto files easier as it replaces messy protoc commands with clean config file. It is a all-in-one tool as it provides compiling, linting, breaking change detection, and dependency management.

In infra-core/apiv2, "buf generate" command is executed within the "make generate" or "make buf-gen" target to generate the OpenAPI 3.0 spec directly from .proto files in api/proto/ directory.

Protoc-gen-connect-openapi plugin takes as an input one full openapi spec that includes all services (services.proto) and outputs the openapi spec in api/openapi.

Key Items:
- Input: api/proto/**/*.proto
- Config: buf.gen.yaml, buf.work.yaml, buf.yaml
- Output: openapi.yaml
- Tool: protoc-gen-connect-openapi

Based on the content of api/proto/ , buf also generates:
- the Go code ( Go structs, gRPC clients/services) in internal/pbapi
- gRPC gateway: REST to gRPC proxy code - HTTP handlers that proxy REST calls to gRPC (in internal/pbapi/**/*.pb.gw.go )
- documentation: docs/proto.md


### About CLI

There are multiple levels of APIs currently available, with individual specs available for each domain in [orch-utils](https://github.com/open-edge-platform/orch-utils/tree/main/tenancy-api-mapping/openapispecs/generated).

The list of domain APIs includes:

- Catalog and Catalog utilities APIs
- App deployment manager and app resource manager APIs
- Cluster APIs
- EIM APIs
- Alert Monitoring APIs
- MPS and RPS APIs
- Metadata broker and Tenancy APIs

There are two levels to the API decomposition:

- **Cross-domain decomposition**: Separation of the above domain-level APIs (e.g., only exposing EIM + Cluster APIs without App Orchestrator APIs)
- **Intra-domain decomposition**: Separation within a domain (e.g., at the EIM domain level, where the overall set of APIs includes onboarding/provisioning/Day 2 APIs, but another workflow may support only onboarding/provisioning without Day 2 support)

The following questions must be answered and investigated:

- How is the API service built currently?
  - It is built from a proto definition and code is autogenerated by the "buf" tool - [See How NB API is Currently Built](#how-nb-api-is-currently-built)
- How is the API service container image built currently?
- How are the API service Helm charts built currently?
- What level of decomposition is needed for the required workflows?
- How to decompose APIs at the domain level?
  - At the domain level, the APIs are deployed as separate services
- How to decompose APIs within the domain level?
- How to build various API service versions as per desired workflows using the modular APIs?
- How to deliver the various API service versions as per desired workflows?
- How to expose the list of available APIs for client consumption (orch-cli)?

## Decomposing the release of API service as a module

Once the investigation is completed on how the API service is created today, decisions must be made on how the service will be built and released as a module.

**Build and Release Strategy:**

- The build of the API service itself will depend on the results of "top-to-bottom" and "bottom-to-top" decomposition investigations.
- Individual versions of the API service can be packaged as versioned container images:
  - `apiv2-emf:x.x.x` (full EMF with all APIs)
  - `apiv2-workflow1:x.x.x` (e.g., onboarding + provisioning only)
  - `apiv2-workflow2:x.x.x` (e.g., minimal edge node management)

**Recommended Approach: Multiple Container Images (One per Scenario)**

**Why this is the only viable option:**

`buf generate` doesn't just create OpenAPI specs—it generates the entire Go codebase including:
- Go structs from proto messages
- gRPC client and server code
- HTTP gateway handlers (REST to gRPC proxy)
- Type conversions and validators

**This means:** You cannot have a single image with "all code" and selectively enable services at runtime. If a service isn't generated by `buf`, the Go code doesn't exist and handlers can't be registered.

**Build Strategy:**
- Build **separate container images per scenario**, each containing only the required API subset
  - `apiv2:eim-full-2025.2` (full EMF with all APIs)
  - `apiv2:eim-minimal-2025.2` (onboarding + provisioning only)
  - `apiv2:eim-provisioning-2025.2` (provisioning workflow only)
- Each image is built from scenario manifests via Makefile
- Each build runs `buf generate` with only the proto files for that scenario's services

**Build Process Per Scenario:**
```bash
# For eim-minimal scenario
1. Read scenarios/eim-minimal.yaml → services: [onboarding, provisioning]
2. Run: buf generate api/proto/services/onboarding/v1 api/proto/services/provisioning/v1
3. Compile Go code (only onboarding and provisioning code exists)
4. Build Docker image: apiv2:eim-minimal-2025.2
```

**Pros:**
- ✅ Only compiles and includes needed services (smaller images)
- ✅ Explicit API surface per image
- ✅ Clear separation between scenarios
- ✅ Better security (reduced attack surface—unused code doesn't exist)
- ✅ Faster startup (fewer services to initialize)
- ✅ Aligns with how `buf generate` actually works

**Cons:**
- Multiple images to build and maintain in CI/CD
- More storage in container registry
- Need to rebuild all images for common code changes

**Helm Chart:**

Single Helm chart for all scenarios will use a flag to use scenariospecific image

**Benefits:**
- Single Helm chart to maintain
- Image selection controlled by tag that includes scenario name
- Easy to switch scenarios by changing one value
- Argo profiles can specify different scenarios (e.g., `orch-configs/profiles/minimal.yaml` sets `eimScenario: eim-minimal` set in deployment configuration)

### Decomposing the API service

An investigation needs to be conducted into how the API service can be decomposed to be rebuilt as various flavors of the same API service providing different sets of APIs.

**Design Principles:**

1. **Single Source of Truth**: The total set of APIs serves as the main source of the API service, and other flavors/subsets are automatically derived from this based on required functionality. This makes maintenance simple and centralized.

2. **Domain-Level Decomposition**: The API service should be decomposed at the domain level, meaning that all domains or a subset of domains should be available as part of the EMF.
   - At this level, APIs are already decomposed/modular and deployed as separate services (e.g., EIM APIs, Cluster APIs, App Orchestrator APIs)
   - **For EIM-focused scenarios**: Only the EIM domain APIs would be included

3. **Intra-Domain Decomposition**: The API service should be decomposed within the domain level, meaning that only a subset of available APIs may need to be released and/or exposed at the API service level.
   - **Example**: Within the EIM domain, we may not want to expose Day 2 functionality for some workflows, even though Day 2 operations are part of the full EIM OpenAPI spec
   - This allows workflows focused on onboarding/provisioning to omit upgrade, maintenance, and troubleshooting APIs

4. **Resource-Level Decomposition** (Under Investigation): The API service may also need to be decomposed at the individual internal service level.
   - **Example**: Host resource might need different data models across use cases
   - **Note**: This would require separate data models and may increase complexity significantly 

The following are the usual options to decomposing or exposing subsets of APIs.

- ~~API Gateway that would only expose certain endpoints to user~~ - this is a no go for us as we plan to remove the existing API Gateway and it does not actually solve the problem of releasing only specific flavours of EMF.
- Maintain multiple OpenAPI specification - while possible to create multiple OpenAPI specs, the maintenance of same APIs across specs will be a large burden - still let's keep this option in consideration in terms of auto generating multiple specs from top spec.
- ~~Authentication & Authorization Based Filtering~~ - this is a no go for us as we do not control the end users of the EMF, and we want to provide tailored modular product for each workflow.
- ~~API Versioning strategy~~ - Creating different API versions for each use-case - too much overhead without benefits similar to maintaining multiple OpenAPI specs.
- ~~Proxy/Middleware Layer~~ - Similar to API Gateway - does not fit our use cases
- OpenAPI Spec Manipulation - This approach uses OpenAPI's extension mechanism (properties starting with x-) to add metadata that describes which audiences, use cases, or clients should have access to specific endpoints, operations, or schemas. This approach is worth investigating to see if it can give us the automated approach for creating individual OpenAPI specs for workflows based on labels.
- Other approach to manipulate how a flavour of OpenAPIs spec can be generated from main spec, or how the API service can be build conditionally using same spec.

### Consuming the APIs from the CLI

The best approach would be for the EMF to provide a service that communicates which endpoints/APIs are currently supported by the deployed API service. Proposed in ADR https://github.com/open-edge-platform/edge-manageability-framework/pull/1106

**CLI Workflow:**

1. **Discovery on Login**: The CLI requests API capability information upon user login
2. **Configuration Caching**: The CLI saves the supported API configuration locally
3. **Command Validation**: Before executing commands, the CLI checks the cached configuration
4. **Graceful Degradation**: If a command maps to an unsupported API, the CLI displays a clear error message

**Implementation Approach:**

```
CLI Command Flow:
┌─────────────────┐
│ User runs       │
│ orch-cli login  │
└────────┬────────┘
         │
         ▼
┌─────────────────────────┐
│ GET /../capabilities│  ← New service endpoint
└────────┬────────────────┘
         │
         ▼
┌─────────────────────────┐
│ Response:               │
│ {                       │
│   "scenario": "eim-min",│
│   "apis": [             │
│     "onboarding",       │
│     "provisioning"      │
│   ]                     │
│ }                       │
└────────┬────────────────┘
         │
         ▼
┌─────────────────────────┐
│ CLI caches config       │
│ in ~/.orch-cli/config   │
└─────────────────────────┘
```

**Command Execution:**
- The CLI checks the capability configuration before executing a command
- If the required API is not in the supported list, display:
  ```
  Error: This orchestrator deployment does not support <feature-name>
  Available features: onboarding, provisioning
  ```
- For direct curl calls to unsupported endpoints, the API service returns a standard 404 or 501 (Not Implemented) with a descriptive message

## Summary of Action Items

### 1. Traefik Gateway Compatibility

**Status**: Phase 1 retains Traefik for all workflows

**Action Items**:
- Investigate how the Traefik→EIM mapping behaves when EIM only supports a subset of APIs
- Determine error handling when Traefik routes to non-existent EIM endpoints

### 2. Data Model and API Compatibility

**Action Items**:
- Ensure APIs support specific use cases while maintaining compatibility with other workflows
- Review and potentially modify data models to accommodate multiple scenarios
- **Example**: Instance creation requires OS profile for general use case, but this may not be true for self-installed OSes/Edge Nodes
  - Make fields conditionally required based on scenario
  - Document field requirements per scenario in OpenAPI spec
- Collaborate with teams/ADR owners to establish:
  - Required changes at Resource Manager level
  - Required changes at Inventory level  
  - Impact on APIs from these changes

**Timeline**: Investigation required once the set of services is known per each scenario

### 3. Scenario Definition and API Mapping

**Action Items**:
- Define all supported scenarios (e.g., full EMF, minimal onboarding, edge provisioning only)
- For each scenario, document:
  - Required services (which resource managers are needed)
  - Required API endpoints (which operations are exposed)
  - Data model variations (if any)
  - Deployment configuration (Helm values, feature flags)
- Create a scenario-to-API mapping matrix
- Define API specifications per scenario

**Status**: Investigation in progress

## Building REST API Spec per Scenario

The following is the proposed solution (draft) to the requirement for decomposition of EMF, where the exposed REST API is limited to support a specific scenario and maintains compatibility with other scenarios.

### Implementation Steps

#### Step 1: Restructure Proto Definitions

Split the monolithic `services.proto` file into multiple folders/files per service:

```
api/proto/services/
├── onboarding/
│   └── v1/
│       └── onboarding_service.proto
├── provisioning/
│   └── v1/
│       └── provisioning_service.proto
├── maintenance/
│   └── v1/
│       └── maintenance_service.proto
└── telemetry/
    └── v1/
        └── telemetry_service.proto
```

#### Step 2: Define Scenario Manifests

Maintain scenario manifests that list the REST API services supported by each scenario.

**Recommended Approach: Scenario manifest files in repository**

```yaml
# scenarios/eim-minimal.yaml
name: eim-minimal
description: Minimal EIM for onboarding and provisioning only
services:
  - onboarding
  - provisioning

# scenarios/eim-full.yaml
name: eim-full
description: Full EIM with all capabilities
services:
  - onboarding
  - provisioning
  - maintenance
  - telemetry
```

**Why manifest files:**
- Makefile-driven builds read the manifest to determine which services to compile
- Version controlled in git repository
- No runtime database dependencies
- Each scenario gets its own container image
- Clear, declarative configuration

#### Step 3: Expose API Capabilities Endpoint

Add a new service that lists supported services in the current scenario as part of other ADR.

#### Step 4: Modify Build Process

Modify "buf-gen" make target to build the openapi spec for suported services as per scenario manifest. (Later Tag the image with scenario name and version).

#### Step 5: Generate Scenario-Specific OpenAPI Specs

Step 4 generates the `openapi.yaml` file containing only the services supported by the particular scenario. The output file can be named per scenario. An image is build per scenario and pushed seperately.

#### Step 6: CLI Integration

The CLI handles scenario variations through dynamic capability discovery:

1. **Build**: CLI is built based on the full REST API spec (generated with `SCENARIO=eim-full`)
2. **Runtime**: CLI queries the `/api/v2/capabilities` endpoint on login
3. **Validation**: Before executing commands, CLI checks if the required service is in the `supported_services` list
4. **Error Handling**: 
   - For CLI commands: Display user-friendly error message
   - For direct curl calls: API returns HTTP 404 or 501 with descriptive message

### Summary of Current Requirements
- Provide scenario-based API exposure for EIM (full and subsets like onboarding/provisioning).
- Deliver per-scenario OpenAPI specs and container images or runtime-config flags.
- Add a capabilities endpoint that advertises supported services and scenario.
- Ensure Traefik routing and error handling work for missing endpoints.
- Maintain single source of truth for API definitions with automated generation.
- Keep CLI operable against any scenario via discovery, caching, and validation.
- Preserve compatibility with Inventory and SB APIs; no 1:1 mapping changes required.
- Support Helm-driven configuration (image/tag, feature flags, scenario selection).
- Include SPDX headers and follow EMF Mage/ArgoCD workflows.

### Rationale
The approach aims to narrow the operational surface to the specific workflows being targeted, while ensuring the full EMF remains available for comprehensive deployments. Rather than maintaining multiple divergent API specifications, it generates tailored subsets from a single master definition to minimize duplication and drift. User experience is improved by enabling the CLI to detect capabilities at login and prevent unsupported commands upfront. Deployment remains flexible through GitOps profiles, ArgoCD application values, and Helm overrides so teams can switch scenarios without rebuilding. This enables incremental decomposition that can be adopted progressively without breaking existing integrations or workflows.

### Investigation Needed

The following investigation tasks will drive validation of the decomposition approach:

- Validate feasibility of splitting services.proto and generating per-scenario specs via buf/protoc-gen-connect-openapi.
- Evaluate Inventory data model variations and conditional field requirements per scenario.
- Confirm deployment pipeline changes (mage targets) and ArgoCD app configs integration.
- Measure impact on gRPC gateway generation and handler registration per scenario.

### Implementation Plan for Orch CLI

A concise plan to enable scenario-aware CLI behavior with capability discovery and graceful command handling.

- Add login-time discovery: GET capabilities (new service) to retrieve scenario, version, supported_services.
- Cache capabilities in ~/.orch-cli/config with TTL and manual refresh.
- Validate commands against supported_services; show clear errors and available features.
- Hide unsupported help entries when possible.
- Ensure full-spec build while runtime limits features based on capabilities.
- Add E2E tests targeting all scenarios.

### Implementation Plan for EIM API

Here is a short plan to implement scenario-based EIM API decomposition:

1. **Restructure Proto Files**
   - Split monolithic `services.proto` into service-scoped folders (onboarding, provisioning, maintenance, telemetry)
   - Each service in its own directory: `api/proto/services/<service>/v1/<service>_service.proto`

2. **Create Scenario Manifests**
   - Add `scenarios/` directory with YAML files for each scenario
   - Define service lists per scenario (eim-full, eim-minimal, etc.)

3. **Update Makefile Build Process**
   - Modify `buf-gen` target to read scenario manifest and generate only specified services
   - Run `buf generate` with only the proto paths for services in that scenario
   - Build per-scenario images with `docker-build` target
   - Add `build-all-scenarios` target to build all images (with `clean` between each)
   - Image naming convention: `apiv2:<scenario>-<version>`
   - Critical: Must clean generated code between scenarios to avoid conflicts

4. **Implement Capabilities Service**
   - Add `CapabilitiesService` protobuf definition
   - Implement handler to return scenario name, supported services, and version
   - Endpoint reads scenario from build-time embedded configuration

5. **Handler Registration**
   - Conditionally register gRPC and HTTP gateway handlers based on compiled services
   - Only services included in buf-gen for that scenario will have gRPC handlers

6. **OpenAPI Generation**
   - Produce per-scenario OpenAPI outputs: `api/openapi/<scenario>-openapi.yaml`
   - Each image embeds its own OpenAPI spec

7. **Update Helm Chart**
   - Use single, common Helm chart for all scenarios
   - Add `image.tag` value to select which scenario image to deploy
   - Add `scenario.name` value for documentation/validation
   - Expose capabilities endpoint in service definition

8. **ArgoCD Integration**
   - Update ArgoCD application templates to use scenario-based image tags
   - Add `argo.eimScenario` value to cluster configs
   - Profiles specify which scenario to deploy (e.g., minimal profile uses `eim-minimal`)

9. **CI/CD Pipeline**
   - Build all scenario images in CI
   - Tag with both scenario name and version
   - Push all images to registry

### Test plan

This section describes a practical test plan to verify EIM’s scenario-based APIs. It checks that minimal and full deployments work as expected, that clients can discover supported features, and that errors are clear and safe. 

- Integration: REST->gRPC gateway for included services; absence returns 404/501 with descriptive messages.
- CLI E2E: Login discovery, caching, command blocking, error messaging.
- Traefik: Route correctness per scenario; behavior for missing endpoints.
- Data model: Conditional field requirements validated per scenario.
- Profiles: Deploy each scenario via mage deploy:kindPreset; verify openapi and endpoints.
- Regression: Ensure full EMF scenario parity with current API suite.

### Open Issues
- Strategy for OpenAPI proto-level splitting of services into seperate files - modified directory structure and buf-gen make target implementation.
- Long-term plan post-Traefik gateway removal and impacts.
- Handling version compatibility between CLI and the proposed capabilities service (what happens when the service does not exist and CLI expects it to exist?).
- Detailed scenario definitions on the Inventory level - NB APIs should be alligned with the Inventory resource availability in each scenario.
- Managing cross-domain scenarios when EIM-only vs multi-domain APIs are required.
- Managing apiv2 image version used by infra-core argo application - deployment level.

### Uncertainties

- How does potential removal of the API gateway affect the exposure of APIs to the client? (In relation to ADR: https://jira.devtools.intel.com/browse/ITEP-79422)
- How will the decomposition and availability of APIs within the API service be mapped back to the Inventory and the set of South Bound (SB) APIs?
- Which approach to exposing the set of operational EMF services/features is accepted (In relation to ADR: https://github.com/open-edge-platform/edge-manageability-framework/pull/1106)







