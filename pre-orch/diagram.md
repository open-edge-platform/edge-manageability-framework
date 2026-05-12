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

---

## Edge Node Onboarding Flow

### How it works (step-by-step)

When an edge node is onboarded to this orchestrator, the following flow occurs:

```
1. Admin creates host in UI
2. Edge node boots via PXE or downloads provisioning image
3. Edge node registers with orchestrator
4. Orchestrator provisions and manages the edge node
```

### Detailed Flow

```mermaid
sequenceDiagram
    participant ADMIN as Admin User
    participant UI as orch-ui-admin<br/>:443 via traefik
    participant API as nexus-api-gw
    participant HOST as host-manager
    participant ONB as onboarding-manager
    participant DKAM as dkam
    participant VAULT as vault-0
    participant PG as postgresql-cluster
    participant KC as keycloak-0
    participant CERT as certificate-file-server
    participant HAPROXY as haproxy-ingress<br/>192.168.99.40
    participant EDGE as Edge Node

    %% Step 1: Admin registers host
    Note over ADMIN,UI: Step 1 — Admin registers edge node
    ADMIN->>+UI: Browse to https://192.168.99.30/ui (port 443)
    UI->>API: POST /v1/hosts (create host record)
    API->>KC: Validate auth token
    KC-->>API: Token valid
    API->>HOST: Create host entry
    HOST->>PG: Store host record (state: REGISTERED)
    HOST-->>API: Host ID + onboarding token
    API-->>UI: Display provisioning details
    UI-->>ADMIN: Show host ID & download link

    %% Step 2: Edge node gets provisioning material
    Note over ADMIN,EDGE: Step 2 — Edge node gets provisioning image/token
    ADMIN->>EDGE: Provide provisioning USB/ISO or PXE config
    EDGE->>HAPROXY: PXE boot request (port 80/8080)
    HAPROXY->>ONB: Forward PXE/boot request
    ONB->>PG: Lookup host by MAC/serial
    ONB-->>EDGE: Serve boot image + onboarding config

    %% Step 3: Edge node downloads certs & registers
    Note over EDGE,VAULT: Step 3 — Edge node obtains certificates
    EDGE->>CERT: GET /cert (download CA certificate via traefik :443)
    CERT->>VAULT: Retrieve signing cert
    VAULT-->>CERT: Return cert
    CERT-->>EDGE: CA certificate bundle

    %% Step 4: Edge node authenticates & completes onboarding
    Note over EDGE,ONB: Step 4 — Edge node registers with orchestrator
    EDGE->>HAPROXY: POST /onboard (port 443 with onboarding token)
    HAPROXY->>ONB: Forward onboarding request
    ONB->>KC: Validate onboarding token
    KC-->>ONB: Token valid
    ONB->>DKAM: Generate device key & attestation
    DKAM->>VAULT: Store device keys
    VAULT-->>DKAM: Keys stored
    DKAM-->>ONB: Device identity ready
    ONB->>HOST: Update host state
    HOST->>PG: Update host (state: ONBOARDED)
    ONB-->>EDGE: Return cluster config + credentials

    %% Step 5: Edge node joins fleet
    Note over EDGE,HOST: Step 5 — Edge node becomes managed
    EDGE->>HAPROXY: Connect to cluster-connect-gateway (port 443)
    HAPROXY->>HOST: Edge node heartbeat
    HOST->>PG: Update host (state: RUNNING)
    HOST-->>EDGE: Management commands
```

### Pod Roles in Onboarding

| Pod | Role in Onboarding |
|---|---|
| **orch-ui-admin** | Admin UI to create/manage hosts |
| **nexus-api-gw** | API gateway — routes REST calls to backend services |
| **keycloak-0** | Authenticates admin users and validates onboarding tokens |
| **host-manager** | Manages host lifecycle (REGISTERED → ONBOARDED → RUNNING) |
| **onboarding-manager** | Handles PXE boot, serves provisioning images, processes onboarding requests |
| **dkam** | Device Key & Attestation Manager — generates device identity and keys |
| **vault-0** | Stores device keys and signing certificates |
| **certificate-file-server** | Serves CA certificates to edge nodes |
| **postgresql-cluster** | Persists host records, state, and metadata |
| **haproxy-ingress** | Entry point for edge node traffic (PXE boot, onboarding, cluster connect) |
| **traefik** | Entry point for admin/UI HTTPS traffic |
| **cluster-connect-gateway** | Maintains persistent connection with onboarded edge nodes |

### Network Ports Used

| Entry Point | IP | Ports | Purpose |
|---|---|---|---|
| **traefik** (LoadBalancer) | 192.168.99.30 | 443 (HTTPS), 80 (HTTP redirect) | Admin UI, API, cert downloads |
| **haproxy-ingress** (LoadBalancer) | 192.168.99.40 | 80, 443, 8080 | PXE boot, edge onboarding, cluster connect |

### Host State Machine

```
REGISTERED → ONBOARDED → RUNNING
     ↑            ↓          ↓
     └── FAILED ←─┴── ERROR ←┘
```
