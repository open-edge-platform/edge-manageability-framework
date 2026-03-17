# Design Proposal: Onboarding Deployment Packages from Github Repositories

Author(s): Scott Baker

Last updated: 2026-03-16

## Abstract

This proposal describes a set of steps that may be taken to reduce the platform dependencies of the vpro profile.

## Problem Statement

This ADR explores what a “minimal” vPro Profile installation might look like, with as many
platform dependencies removed as possible. The current dependencies in the vPro profile look like
this (zoom in to read):

```mermaid
flowchart LR

A[client] -->|over internet| loadbalancer(loadbalancer)
loadbalancer -->|SNI, jwt validation, security| traefik{traefik}
traefik -->|jwt creation| keycloak[keycloak]
traefik -->|MPS RPS AMT VPRO| infra-external[infra-external]
traefik -->|TODO| infra-internal[infra-internal]
traefik -->|Gui| web-ui[web-ui]
traefik -->|certs| traefik-extra-objects[traefik-extra-objects]
traefik -->|certs-management| cert-manager[cert-manager]
cert-manager --> |copy ca secret| copy-ca-cert-gateway-to-infra[copy-ca-cert-gateway-to-infra]
keycloak -->|project tenant required for jwt| nexus[nexus]
keycloak -->|storage| postgres[postgres]
nexus -->|links project to roles in keycloak| ktc[keycloak tenant controller]
ktc -->|single tenant| tenancy-init[tenancy-init]
infra-external -->|storage| postgres[postgres]
infra-external -->|secret management| vault[vault]
infra-external -->|refresh vault token| reloader
reloader --> vault[vault]
vault -->|storage| postgres[postgres]
vault -->|vault accounts, enable k8 auth| secrets-config[secrets-config]
postgres -->|postgres secrets, database details copied into app containers| postgres-secrets[postgres-secrets]
postgres -->|deploy postgress| postgres-operator[postgres-operator]
```

Platform components include:

- Ingress pipeline
  - MetalLB, allows the application to be exposed using a load-balancer
  - Traefik, used as an API gateway and an API remapper (for multitenant-compliant API).
    Also serves as an early check for JWT validity.
  - Traefik-extra-objects, holds certificates for configuring Traefik.
  - Cert-Manager, used to manage certificates for Traefik

- Multitenancy
  - Keycloak, used to generate JWT tokens that may be validated by Traefik and by the
    backend components. Used to manage user accounts.
  - Nexus, used as a data model for multitenancy.
  - Tenancy-init, used to initialize a single tenant environment

- Database
  - Postgres, as a database
  - Postgres-operator, used to lifecycle manage the postgres deployment
  - Postgres-secrets, used to initialize database details and populate secrets

- Secret Storage
  - Vault, used to store sensitive secrets
  - Reloader, used to periodically restart DMT when vault tokens change
  - Secrets-config, used to setup vault accounts

- Web-Ui, allows device management using a GUI interface

## Proposal

This proposal is divided into several sub-proposals, which may be executed independently.

### Remove MetalLB

Effort: Low/Medium

Most modern helm charts allow the type of service to be specified – LoadBalancer, NodePort,
ClusterIP, etc. We should expose the same capability, and allow the customer to bring their own
LoadBalancer, which may or may not be MetalLB. Similarly, if the customer wants to use a
NodePort, that should not be prohibited.

### Remove Traefik

Effort: High

Recent changes made it possible to bypass the remapping that was done by tenancy-api-mapper. The
EIM components now directly expose an API that is identical to the external API that was exposed
by the mapper. This may be used to simplify the choice of ingress / API gateway.

However, Traefik is not simply serving as an API mapper. It also aggregates multiple backend
components’ APIs together. If we want to expose only a single service, then Traefik could be
eliminated, but EIM is a combination of multiple services, and this complicates removal of
Traefik.

Removal of Traefik does not necessarily allow us to remove the requirement to have an Ingress,
because of the need to aggregate multiple APIs under a single endpoint. We could leave the
Ingress requirement in place in our charts, and leave it as a customer responsibility to provide
the Ingress itself. The customer could choose Traefik or a different tool. This would also lead
to the customer being responsible for the certificate management that was needed as part of the
Traefik service.

### Remove Reloader Job

Effort: Medium

This task requires research to see if there is an alternate way to refresh Vault tokens in DMT
components. May require DMT changes. Requires investigation and possible consultation with DMT
team.

### Remove Keycloak, Nexus, and Keycloak-Tenant-Controller

Effort: Very High

This requires more understanding regarding recent multitenancy refactoring in EIM, and to
determine whether there is sufficient support for a default-tenant in the API that does not
require a lookup of Nexus

- What JWT would be used w/o Keycloak? How would it be validated.

- Are there tenant controllers in infra-internal that need to be run to setup objects? How would
  they run with no tenancy-datamodel to trigger them?

### Remove Postgres-Operator and Postgres-Secrets

We do not need an operator to manage postgres. We could instantiate the Postgres service directly.
Secrets (username, password, etc) could be configured by simple text file, as is the case with
DMT.

### Simplify Configuration (Vault and Postgres)

Both Vault and Postgres are using Kubernetes jobs for configuration. Looking at another
implementation, DMT, configuration is specified in a simple text file that is ingested during
deployment:

```text
# POSTGRES
POSTGRES_USER=postgresadmin
POSTGRES_PASSWORD=

# VAULT
SECRETS_PATH=secret/data/
VAULT_ADDRESS=http://vault:8200
VAULT_TOKEN=
```

We should be able to do the same.

## Recommendations

Remove MetalLB -> **proceed**

Remove Traefik -> perform **PoC** on removing Traefik as a hard requirement, allowing user to
choose ingress technology themselves.

Remove Reloader -> **proceed**

Remove Keycloak -> **defer**

Remove Postgres Operator and Postgres-Secret -> **proceed**

Simplify Configuration -> **proceed**

## Implementation Plan

TBD

## Decision

TBD

## Open issues (if applicable)

TBD
