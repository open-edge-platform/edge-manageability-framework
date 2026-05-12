# EMF Orchestration — External Access Flow & Pod Dependencies

```mermaid
graph TD
    %% External User Access
    USER["External User"]

    %% MetalLB Load Balancer
    METALLB["metallb-controller<br/>metallb-speaker"]

    %% Traefik Ingress - Main HTTPS entry
    TRAEFIK["traefik<br/>LB IP: 192.168.99.30<br/>:443 HTTPS / :80 HTTP"]

    %% HAProxy Ingress - PXE boot entry
    HAPROXY["haproxy-ingress<br/>LB IP: 192.168.99.40<br/>:80 :443 :8080"]

    %% Auth & Identity
    AUTH["auth-service"]
    KEYCLOAK["keycloak-0"]
    KC_TENANT["keycloak-tenant-controller"]

    %% Tenancy / IAM
    NEXUS_GW["nexus-api-gw"]
    TEN_API["tenancy-api-mapping"]
    TEN_MGR["tenancy-manager"]

    %% Database
    PG["postgresql-cluster-1<br/>postgresql-cluster-2"]

    %% Secrets & Certs
    VAULT["vault-0"]
    TOKEN_FS["token-fs"]
    CERT_FS["certificate-file-server"]

    %% Infra Services
    HOST_MGR["host-manager"]
    FLEET_MGR["fleet-manager"]
    ONB_MGR["onboarding-manager"]
    DKAM["dkam"]
    MAINT_MGR["maintenance-manager"]
    UPDATE_MGR["update-manager"]
    CLUSTER_GW["cluster-connect-gateway"]

    %% UI
    UI_ROOT["orch-ui-root"]
    UI_ADMIN["orch-ui-admin"]
    UI_INFRA["orch-ui-infra"]
    META_BROKER["metadata-broker"]

    %% Service Mesh
    ISTIOD["istiod"]

    %% ─── External Access Flow ───────────────────────────────────
    USER -->|":443 HTTPS"| TRAEFIK
    USER -->|":80 HTTP"| TRAEFIK
    USER -->|":443/:80/:8080 PXE"| HAPROXY
    METALLB -.->|"assigns IP"| TRAEFIK
    METALLB -.->|"assigns IP"| HAPROXY

    %% ─── Traefik routes to services ────────────────────────────
    TRAEFIK -->|"forward-auth"| AUTH
    TRAEFIK -->|"/ui"| UI_ROOT
    TRAEFIK -->|"/api"| NEXUS_GW
    TRAEFIK -->|"/cert"| CERT_FS

    %% ─── HAProxy routes ────────────────────────────────────────
    HAPROXY -->|"PXE boot"| ONB_MGR
    HAPROXY -->|"edge connect"| CLUSTER_GW

    %% ─── Auth & Identity dependencies ──────────────────────────
    AUTH --> KEYCLOAK
    KC_TENANT --> KEYCLOAK
    KEYCLOAK --> PG

    %% ─── API Gateway flow ──────────────────────────────────────
    NEXUS_GW --> TEN_API
    NEXUS_GW --> TEN_MGR
    TEN_API --> KEYCLOAK
    TEN_MGR --> KEYCLOAK
    TEN_MGR --> PG

    %% ─── Infra pod-to-pod ──────────────────────────────────────
    HOST_MGR --> PG
    FLEET_MGR --> PG
    ONB_MGR --> PG
    ONB_MGR --> VAULT
    DKAM --> VAULT
    MAINT_MGR --> PG
    UPDATE_MGR --> PG
    CLUSTER_GW --> HOST_MGR

    %% ─── UI dependencies ───────────────────────────────────────
    UI_ROOT --> UI_ADMIN
    UI_ROOT --> UI_INFRA
    UI_ADMIN --> META_BROKER
    UI_INFRA --> META_BROKER
    META_BROKER --> NEXUS_GW

    %% ─── Secrets / Token flow ──────────────────────────────────
    TOKEN_FS --> VAULT
    CERT_FS --> VAULT

    %% ─── Service Mesh sidecar injection ────────────────────────
    ISTIOD -.->|"sidecar inject"| AUTH
    ISTIOD -.->|"sidecar inject"| HOST_MGR
    ISTIOD -.->|"sidecar inject"| FLEET_MGR
    ISTIOD -.->|"sidecar inject"| ONB_MGR
```
