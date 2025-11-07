# Design Proposal: Exposing only the required North Bound APIs and CLI commands for the workflow as part of EIM decomposition

Author(s) EIM-Core team

Last updated: 7/11/25

## Abstract

In context of EIM decomposition the North Bound API service should be treated as an independent interchangeable module.
The [EIM proposal for modular decomposition](https://github.com/open-edge-platform/edge-manageability-framework/blob/main/design-proposals/eim-modular-decomposition.md) calls out a need for exposing both a full set of EMF APIs, and a need for exposing only a subset of APIs as required by individual workflows taking advantage of a modular architecture. This proposal will explore, how the APIs can be decomposed and how the decomposed output can be used as version of API service module.

## Background and Context

In EMF 2025.2 the API service is deployed via a helm chart deployed by Argo CD. The API service is run and deployed in a container kick-started from the API service container image. The API is build using the OpenAPI spec. There are multiple levels of APIs currently available with individual specs available for each domain in [orch-utils](https://github.com/open-edge-platform/orch-utils/tree/main/tenancy-api-mapping/openapispecs/generated)

The list of domain APIs include:

- Catalog and Catalog utilities APIs
- App deployment manager and app resource manager APIs
- Cluster APIs
- EIM APIs
- Alert Monitoring APIs
- MPS and RPS APIs
- Metadata broker and Tenancy APIs

There are two levels to the API decomposition

- Decomposition of above domain levels
- Decomposition within domain (ie. separation at EIM domain level, where overall set of APIS includes onboarding/provisioning/day2 APIs but another workflow may support only onboarding/provisioning without day2 support )

The following questions must be answered and investigated:

- How the API service is build currently
- How the API service container image is build currently
- How the API service helm charts are build currently
- What level of decomposition is needed from the required workflows
- How to decomposition API at domain level
- How to decomposition API within domain level
- How to build various API service version as per desired workflows using the modular APIs
- How to deliver the various API service versions as per desired workflows
- How to expose the list of available APIs for client consumption (orch-cli)

Uncertainties:

- How does potential removal of the API gateway affect the exposing of the APIs to the client
- How will the decomposition and availability of APIs within the API service be mapped back to the Inventory and the set of SB APIs.

### Decomposing the release of API service as a module

Once the investigation is completed on how the API service is create today decisions must be done on a the service will be build and released as a module.

- The build of the API service itself will depend on the results of top2bottom and bottom2top decomposition investigations.
- The individual versions of API service can be packaged as versioned container images:
  - apiv2-emf:x.x.x
  - apiv2-workflow1:x.x.x
  - apiv2-workflow2:x.x.x
- Alternatively if the decomposition does not result in multiple version of the API service the service could be released as same docker image but managed by flags provided to container that alter the behavior of the API service in runtime.
- The API service itself should still be packaged for deployment as a helmchart regardless of deployment via ArgoCD or other medium/technique. Decision should be made if common helmchart is used with override values for container image and other related values (preferred) or individual helmcharts need to be released.

### Decomposing the API service

An investigation needs to be conducted into how the API service can be decomposed to be rebuilt as various flavours of same API service providing different set of APIs.

- Preferably the total set of APIs serves as the main source of the API service, and other flavours/subsets are automatically derived from this based on the required functionality. Making the maintenance of the API simple and in one place.
- The APIs service should be decomposed at the domain level meaning that all domains or subset of domains should be available as part of the API service flavour. This should allows us to provide as an example EIM related APIs only as needed by workflow. We know that currently the domains have separate generated OpenAPI specs available as consumed by orch-cli.
- The APIs service should be decomposed within the domain level meaning that only subset of the available APIs may need to be released and/or exposed at API service level. As an example within the EIM domain we may not want to expose the Day 2 functionality for some workflows which currently part of the EIM OpenAPI spec.

The following are the usual options to decomposing or exposing subsets of APIs.

- ~~API Gateway that would only expose certain endpoints to user~~ - this is a no go for us as we plan to remove the existing API Gateway and it does not actually solve the problem of releasing only specific flavours of EMF.
- Maintain multiple OpenAPI specification - while possible to create multiple OpenAPI specs, the maintenance of same APIs across specs will be a large burden - still let's keep this option in consideration in terms of auto generating multiple specs from top spec.
- ~~Authentication & Authorization Based Filtering~~ - this is a no go for us as we do not control the end users of the EMF, and we want to provide tailored modular product for each workflow.
- ~~API Versioning strategy~~ - Creating different API versions for each use-case - too much overhead without benefits similar to maintaining multiple OpenAPI specs.
- ~~Proxy/Middleware Layer~~ - Similar to API Gateway - does not fit our use cases
- OpenAPI Spec Manipulation - This approach uses OpenAPI's extension mechanism (properties starting with x-) to add metadata that describes which audiences, use cases, or clients should have access to specific endpoints, operations, or schemas. This approach is worth investigating to see if it can give use the automated approach for creating individual OpenAPI specs for workflows based on labels.
- Other approach to manipulate how a flavour of OpenAPIs spec can be generated from main spec, or how the API service can be build conditionally using same spec.

### Consuming the APIs from the CLI

The best approach would be for the EMF to provide a service/endpoint that will communicate which endpoints/APIs are currently supported by the deployed API service. The CLI would then request that information on login, save the configuration and prevent from using non-supported APIs/commands.

# Appendix: OpenAPI Spec Manipulation with Extensions

This approach uses OpenAPI's extension mechanism (properties starting with `x-`) to add metadata that describes which audiences, use cases, or clients should have access to specific endpoints, operations, or schemas.

## How It Works

### 1. Adding Custom Extensions to Your OpenAPI Spec

```yaml
openapi: 3.0.0
info:
  title: My API
  version: 1.0.0

paths:
  /users:
    get:
      summary: Get all users
      x-audience: ["public", "partner"]
      x-use-case: ["user-management", "reporting"]
      x-access-level: "read"
      responses:
        '200':
          description: Success
          
  /users/{id}:
    get:
      summary: Get user by ID
      x-audience: ["public", "partner", "internal"]
      x-use-case: ["user-management"]
      responses:
        '200':
          description: Success
    delete:
      summary: Delete user
      x-audience: ["internal"]
      x-use-case: ["admin"]
      x-access-level: "write"
      responses:
        '204':
          description: Deleted

  /admin/analytics:
    get:
      summary: Get analytics data
      x-audience: ["internal"]
      x-use-case: ["analytics", "reporting"]
      x-sensitive: true
      responses:
        '200':
          description: Analytics data

components:
  schemas:
    User:
      type: object
      x-audience: ["public", "partner", "internal"]
      properties:
        id:
          type: string
        name:
          type: string
        email:
          type: string
          x-audience: ["internal"]  # Email only for internal use
        ssn:
          type: string
          x-audience: ["internal"]
          x-sensitive: true

# Audience-based filtering
x-audience: ["public", "partner", "internal", "admin"]

# Use case categorization
x-use-case: ["user-management", "reporting", "analytics", "billing"]

# Access level requirements
x-access-level: "read" | "write" | "admin"

# Sensitivity marking
x-sensitive: true

# Client-specific
x-client-type: ["mobile", "web", "api"]

# Environment restrictions
x-environment: ["production", "staging", "development"]

# Rate limiting categories
x-rate-limit-tier: "basic" | "premium" | "enterprise"

# Deprecation info
x-deprecated-for: ["internal"]
x-sunset-date: "2024-12-31"