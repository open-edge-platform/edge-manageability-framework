# EMF Orchestration — External Access Flow & Pod Dependencies

```mermaid
graph TD
    classDef external fill:#E3F2FD,stroke:#1565C0,stroke-width:2px,color:#0D47A1
    classDef ingress fill:#FFF3E0,stroke:#E65100,stroke-width:2px,color:#BF360C
    classDef auth fill:#F3E5F5,stroke:#6A1B9A,stroke-width:2px,color:#4A148C
    classDef api fill:#E8F5E9,stroke:#2E7D32,stroke-width:2px,color:#1B5E20
    classDef data fill:#FBE9E7,stroke:#BF360C,stroke-width:2px,color:#B71C1C
    classDef infra fill:#E0F7FA,stroke:#00695C,stroke-width:2px,color:#004D40
    classDef ui fill:#FFF9C4,stroke:#F57F17,stroke-width:2px,color:#E65100
    classDef secret fill:#FCE4EC,stroke:#880E4F,stroke-width:2px,color:#880E4F

    USER["External User"]:::external

    subgraph INGRESS["Ingress Layer"]
        TRAEFIK["traefik<br/>192.168.99.30<br/>:443 / :80"]
        HAPROXY["haproxy-ingress<br/>192.168.99.40<br/>:443 / :80 / :8080"]
    end

    subgraph AUTH_LAYER["Authentication"]
        AUTH["auth-service"]
        KEYCLOAK["keycloak-0"]
    end

    subgraph API_LAYER["API Gateway"]
        NEXUS_GW["nexus-api-gw"]
    end

    subgraph UI_LAYER["Web UI"]
        UI_ROOT["orch-ui-root"]
        META_BROKER["metadata-broker"]
    end

    subgraph INFRA_LAYER["Edge Infrastructure"]
        ONB_MGR["onboarding-manager"]
        DKAM["dkam"]
        HOST_MGR["host-manager"]
        FLEET_MGR["fleet-manager"]
        CLUSTER_GW["cluster-connect-gateway"]
    end

    subgraph DATA_LAYER["Data & Secrets"]
        PG["postgresql-cluster"]
        VAULT["vault-0"]
        CERT_FS["certificate-file-server"]
    end

    %% External → Ingress
    USER -->|":443 HTTPS"| TRAEFIK
    USER -->|":8080 PXE boot"| HAPROXY

    %% Ingress → Services
    TRAEFIK --> AUTH
    TRAEFIK --> UI_ROOT
    TRAEFIK --> NEXUS_GW
    TRAEFIK --> CERT_FS
    HAPROXY --> ONB_MGR
    HAPROXY --> CLUSTER_GW

    %% Auth
    AUTH --> KEYCLOAK
    KEYCLOAK --> PG

    %% API
    NEXUS_GW --> KEYCLOAK

    %% UI
    UI_ROOT --> META_BROKER
    META_BROKER --> NEXUS_GW

    %% Onboarding
    ONB_MGR --> KEYCLOAK
    ONB_MGR --> PG
    ONB_MGR --> DKAM
    DKAM --> VAULT
    ONB_MGR --> HOST_MGR

    %% Infra
    HOST_MGR --> PG
    FLEET_MGR --> PG
    CLUSTER_GW --> HOST_MGR
    CERT_FS --> VAULT

    %% Apply styles
    TRAEFIK:::ingress
    HAPROXY:::ingress
    AUTH:::auth
    KEYCLOAK:::auth
    NEXUS_GW:::api
    UI_ROOT:::ui
    META_BROKER:::ui
    ONB_MGR:::infra
    DKAM:::infra
    HOST_MGR:::infra
    FLEET_MGR:::infra
    CLUSTER_GW:::infra
    PG:::data
    VAULT:::secret
    CERT_FS:::secret
```

