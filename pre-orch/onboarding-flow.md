# Edge Node Onboarding Flow

## How it works (step-by-step)

```
1. Admin creates host in UI
2. Edge node boots via PXE (Tinkerbell) or downloads provisioning image
3. Tinkerbell workflows provision the OS on the edge node
4. Edge node registers with orchestrator
5. Orchestrator provisions and manages the edge node
```

---

## Onboarding Flow (Numbered)

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
    ADMIN->>+UI: 1.1 Browse to https://192.168.99.30/ui (port 443)
    UI->>API: 1.2 POST /v1/hosts (create host record)
    API->>KC: 1.3 Validate auth token
    KC-->>API: 1.4 Token valid
    API->>HOST: 1.5 Create host entry
    HOST->>PG: 1.6 Store host record (state: REGISTERED)
    HOST-->>API: 1.7 Host ID + onboarding token
    API-->>UI: 1.8 Display provisioning details
    UI-->>ADMIN: 1.9 Show host ID & download link

    %% Step 2: Edge node gets provisioning material
    Note over ADMIN,EDGE: Step 2 — Edge node gets provisioning image/token
    ADMIN->>EDGE: 2.1 Provide provisioning USB/ISO or PXE config
    EDGE->>HAPROXY: 2.2 PXE boot request (port 80/8080)
    HAPROXY->>ONB: 2.3 Forward PXE/boot request
    ONB->>PG: 2.4 Lookup host by MAC/serial
    ONB-->>EDGE: 2.5 Serve boot image + onboarding config

    %% Step 3: Edge node downloads certs & registers
    Note over EDGE,VAULT: Step 3 — Edge node obtains certificates
    EDGE->>CERT: 3.1 GET /cert (download CA certificate via traefik :443)
    CERT->>VAULT: 3.2 Retrieve signing cert
    VAULT-->>CERT: 3.3 Return cert
    CERT-->>EDGE: 3.4 CA certificate bundle

    %% Step 4: Edge node authenticates & completes onboarding
    Note over EDGE,ONB: Step 4 — Edge node registers with orchestrator
    EDGE->>HAPROXY: 4.1 POST /onboard (port 443 with onboarding token)
    HAPROXY->>ONB: 4.2 Forward onboarding request
    ONB->>KC: 4.3 Validate onboarding token
    KC-->>ONB: 4.4 Token valid
    ONB->>DKAM: 4.5 Generate device key & attestation
    DKAM->>VAULT: 4.6 Store device keys
    VAULT-->>DKAM: 4.7 Keys stored
    DKAM-->>ONB: 4.8 Device identity ready
    ONB->>HOST: 4.9 Update host state
    HOST->>PG: 4.10 Update host (state: ONBOARDED)
    ONB-->>EDGE: 4.11 Return cluster config + credentials

    %% Step 5: Edge node joins fleet
    Note over EDGE,HOST: Step 5 — Edge node becomes managed
    EDGE->>HAPROXY: 5.1 Connect to cluster-connect-gateway (port 443)
    HAPROXY->>HOST: 5.2 Edge node heartbeat
    HOST->>PG: 5.3 Update host (state: RUNNING)
    HOST-->>EDGE: 5.4 Management commands
```

---

## Tinkerbell Provisioning Flow (Numbered)

Tinkerbell handles the bare-metal OS provisioning via PXE before the edge node can onboard.

```mermaid
sequenceDiagram
    participant EDGE as Edge Node<br/>(bare-metal)
    participant DHCP as DHCP Server<br/>(network)
    participant HAPROXY as haproxy-ingress<br/>192.168.99.40:80/8080
    participant BOOTS as tink-boots<br/>(iPXE server)
    participant TINK_SRV as tink-server<br/>(workflow engine)
    participant HEGEL as hegel<br/>(metadata service)
    participant PG as postgresql-cluster
    participant ONB as onboarding-manager
    participant SMEE as smee<br/>(DHCP proxy)

    Note over EDGE,SMEE: Step 1 — Network Boot Discovery
    EDGE->>DHCP: 1.1 DHCP DISCOVER (PXE client)
    DHCP-->>EDGE: 1.2 DHCP OFFER (IP + next-server)
    SMEE->>EDGE: 1.3 ProxyDHCP — point to iPXE URL (haproxy :8080)

    Note over EDGE,BOOTS: Step 2 — iPXE Boot
    EDGE->>HAPROXY: 2.1 HTTP GET iPXE script (port 8080)
    HAPROXY->>BOOTS: 2.2 Forward to tink-boots
    BOOTS->>TINK_SRV: 2.3 Lookup hardware by MAC
    TINK_SRV->>PG: 2.4 Query hardware record
    PG-->>TINK_SRV: 2.5 Return hardware + workflow ID
    TINK_SRV-->>BOOTS: 2.6 Return iPXE config
    BOOTS-->>EDGE: 2.7 iPXE script (kernel + initrd URLs)

    Note over EDGE,TINK_SRV: Step 3 — Load OS Installer
    EDGE->>HAPROXY: 3.1 Download kernel + initrd (port 8080)
    HAPROXY->>BOOTS: 3.2 Serve kernel/initrd
    BOOTS-->>EDGE: 3.3 Boot into HookOS (in-memory Linux)

    Note over EDGE,HEGEL: Step 4 — Fetch Metadata
    EDGE->>HAPROXY: 4.1 GET /metadata (port 80)
    HAPROXY->>HEGEL: 4.2 Forward metadata request
    HEGEL->>TINK_SRV: 4.3 Lookup instance metadata
    TINK_SRV->>PG: 4.4 Query metadata
    PG-->>TINK_SRV: 4.5 Return metadata
    TINK_SRV-->>HEGEL: 4.6 Instance metadata
    HEGEL-->>EDGE: 4.7 Return metadata JSON

    Note over EDGE,PG: Step 5 — Execute Workflow Actions
    EDGE->>HAPROXY: 5.1 tink-worker connects to tink-server (port 443)
    HAPROXY->>TINK_SRV: 5.2 gRPC: GetWorkflowActions
    TINK_SRV->>PG: 5.3 Fetch workflow actions
    PG-->>TINK_SRV: 5.4 Return action list
    TINK_SRV-->>EDGE: 5.5 Action list (partition, format, install OS, write config)
    EDGE->>EDGE: 5.6 Execute actions (disk partition, OS write, bootloader install)
    EDGE->>HAPROXY: 5.7 Report action status
    HAPROXY->>TINK_SRV: 5.8 Update workflow state
    TINK_SRV->>PG: 5.9 Store state: SUCCESS

    Note over EDGE,ONB: Step 6 — Reboot into Provisioned OS
    EDGE->>EDGE: 6.1 Reboot from disk
    EDGE->>HAPROXY: 6.2 POST /onboard (onboarding-manager via port 443)
    HAPROXY->>ONB: 6.3 Begin orchestrator onboarding (see Onboarding Flow Step 4)
```

---

## Combined Flow Overview (Numbered)

```mermaid
graph LR
    A["1. Admin creates host<br/>via UI :443"] --> B["2. Edge node PXE boots<br/>via haproxy :8080"]
    B --> C["3. Tinkerbell provisions OS<br/>(boots → tink-server → workflow)"]
    C --> D["4. Edge node reboots<br/>into provisioned OS"]
    D --> E["5. Edge node gets certs<br/>via traefik :443"]
    E --> F["6. Edge node onboards<br/>via haproxy :443"]
    F --> G["7. DKAM generates keys<br/>stored in vault"]
    G --> H["8. Edge node joins fleet<br/>via cluster-connect-gateway"]
```

---

## Pod Roles in Onboarding

| Pod | Role |
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

### Tinkerbell-Specific Pods

| Pod | Role |
|---|---|
| **smee** | DHCP proxy — directs PXE clients to iPXE boot URL |
| **tink-boots** | iPXE server — serves boot scripts, kernel, and initrd |
| **tink-server** | Workflow engine — manages hardware records and workflow execution |
| **hegel** | Metadata service — provides instance metadata to booting nodes |

---

## Network Ports Used

| Entry Point | IP | Ports | Purpose |
|---|---|---|---|
| **traefik** (LoadBalancer) | 192.168.99.30 | 443 (HTTPS), 80 (HTTP redirect) | Admin UI, API, cert downloads |
| **haproxy-ingress** (LoadBalancer) | 192.168.99.40 | 80 (metadata/TFTP), 443 (onboard/gRPC), 8080 (iPXE/PXE boot) | PXE boot, Tinkerbell, edge onboarding, cluster connect |

---

## Host State Machine

```
REGISTERED → PROVISIONING → ONBOARDED → RUNNING
     ↑            ↓              ↓          ↓
     └── FAILED ←─┴──── ERROR ←──┴── ERROR ←┘
```
