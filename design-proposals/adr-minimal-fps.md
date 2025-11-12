# ADR-001: Minimal Foundation Platform Services (FPS) for EMF

**Status:** Proposed

**Date:** 2025-09-30

## Context

The `eim-modular-decomposition.md` design proposal outlines a strategy to decompose the Edge Infrastructure Manager (EIM) into modular, independently consumable components. Currently, EIM services are tightly coupled to a monolithic deployment of shared infrastructure, including PostgreSQL, Keycloak, and other Foundation Platform Services (FPS). This makes it difficult for users to deploy subsets of EIM functionality without inheriting the entire infrastructure footprint.

To enable the modular evolution tracks, particularly "Track #1 (Status Quo + Use Case Enablement)" and "Track #2 (Bring-Your-Own Infrastructure)", we must first define a clear, minimal set of foundational services required for any functional EIM deployment. This ADR specifies that minimal core.

## Decision

We will define the minimal required Foundation Platform Services (FPS) stack for the Edge Management Framework (EMF) to consist of the following four components:

1.  **Identity Service (Keycloak):** Provides centralized authentication and authorization (IAM) for all EIM services, APIs, and users. It is responsible for managing realms, clients, users, and issuing JWT tokens that are fundamental to securing inter-service communication and tenancy.

2.  **Secrets Management Service (Vault):** Provides secure storage, access, and lifecycle management for all secrets, including database credentials, service tokens, TLS certificates, and private keys. This decouples secret management from application configuration and enhances security.

3.  **Ingress Gateway (Traefik):** Acts as the reverse proxy and API gateway for the platform. It manages all external network traffic, provides TLS termination, and routes requests to the appropriate EIM microservices.

4. **Auth-service:** Acts as a service-to-service authentication and authorization gateway that integrates with Keycloak. It is an authentication middleware that validates JWT tokens issued by Keycloak for incoming API requests. It enforces authorization policies for EMF micro services and acts as centralized authentication layer betwee  traefik ingress gateway and back EMF services.

Here is the sqeuence of network traffic flow.
External Request → Traefik (Ingress) → Auth-Service → EIM Microservice
                                ↓
                          Keycloak (validates JWT)
                                ↓
                          Vault (retrieves secrets for Keycloak connection)

Typical Use Cases platform services in EIM

a. API Gateway Authentication: Validates that requests to EIM REST APIs have valid JWT tokens

b. Service Mesh Security: Ensures inter-service communication is authenticated

c. Multi-tenancy Enforcement: Routes requests based on tenant claims in JWT tokens

d. Token Introspection: Validates token expiration, scopes, and permissions before forwarding requests

These four services constitute the baseline infrastructure upon which all EIM modules, whether deployed individually or as a complete stack, will operate by default. The "Bring-Your-Own Infrastructure" track will focus on creating abstraction layers to make these specific components pluggable and replaceable with third-party equivalents.

### Optional Observability Services

While not strictly required for basic EIM functionality, the following services are strongly recommended for production deployments:

1. **Log Aggregation Service (Grafana Loki):** Provides centralized log collection, log aggregation, storage, and querying service that collects from all EIM components. Loki enables operators to troubleshoot issues, monitor system behavior, and maintain audit trails and operational insights across the distributed edge infrastructure. It includes 3 


## Consequences

### Positive

-   **Clear Dependency Contract:** Establishes a well-defined, minimal infrastructure baseline for developers and operators, simplifying the development of new modules.
-   **Enables Modularity:** Provides a stable foundation required to proceed with Track #1 and Track #2 of the modular decomposition. Use-case-specific deployments can rely on this core being present.
-   **Consistent Security Model:** Centralizes identity, secrets, and access control, ensuring a consistent security posture across all EIM components, regardless of the deployment profile.
-   **Simplified Onboarding:** New deployments have a clear, documented list of prerequisite infrastructure services.

### Negative

-   **Initial Overhead:** Even the most minimal EIM module deployment will require this foundational stack, which carries a non-trivial resource footprint.
-   **Technology Lock-in (Default):** While Track #2 aims to make these services pluggable, the default implementation creates a dependency on Keycloak, Vault, PostgreSQL, and Traefik.
-   **Configuration Complexity:** Operators must correctly configure the integration between EIM modules and these four foundational services.