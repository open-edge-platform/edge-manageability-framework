# KVM — EMF vPRO Production Integration Guide
## orch-cli (binary) + Angular UI (pod) + Existing EMF Cluster

This guide explains exactly how to integrate the KVM viewer into a live EMF
vPRO-only deployment.  The key constraint is:

```
orch-cli  →  runs as a binary on the HOST machine  (NOT a K8S pod)
Angular   →  deployed as a pod in the orch-ui namespace
Traefik   →  routes browser traffic to both, via separate path rules
```

---

## 1. Architecture Overview

```
HOST VM  (external IP: 10.139.218.43)
         (kind bridge gateway: 172.18.0.1 -- how pods reach this machine)
+------------------------------------------------------------+
|                                                            |
|  Browser  https://kvm.orch-10-139-218-43.pid.infra-host.com|
|      |                                                     |
|      | HTTPS 443  ---> MetalLB 172.18.255.236              |
|      |                      |                              |
|      |              Traefik (orch-gateway)                 |
|      |                      |                              |
|      |    +-----------------+------------------+           |
|      |    |                                    |           |
|      |    | PathPrefix /api  or  Path /ws      |           |
|      |    v                                    |           |
|      |  K8S Endpoints (orch-ui ns)  PathPrefix /           |
|      |  ip: 172.18.0.1  port: 8080  v                      |
|      |  (kind bridge = host VM)  Angular nginx pod          |
|      |    |                  serves dist/ static files      |
|      |    v                                                 |
|  orch-cli kvm-server  (0.0.0.0:8080)                        |
|  VM: 10.139.218.43 / kind bridge: 172.18.0.1               |
|                  |                                          |
|                  | HTTPS mps-wss.<domain>/api/*  (JWT)      |
|                  | HTTPS mps-wss.<domain>/relay/ (token)    |
|                  v                                          |
|            MPS pod  (orch-infra ns)                         |
|                  |                                          |
|                  | AMT Redirect + RFB                        |
|                  v                                          |
|            Intel AMT Device  :16994                         |
+------------------------------------------------------------+
```

### Component roles

| Component | Where it runs | Purpose |
|---|---|---|
| Angular KVM UI | K8S pod, `orch-ui` namespace | Serves static HTML/JS files to browser |
| nginx (in pod) | K8S pod sidecar | Serves Angular `dist/` on port 80 |
| Traefik | `orch-gateway` namespace | Routes browser to Angular pod OR orch-cli |
| orch-cli (`kvm-server`) | Host machine, port 8080 | KVM relay server: AMT handshake + RFB WebSocket bridge |
| K8S Endpoints object | `orch-ui` namespace | Tells K8S that `kvm-orch-cli` service = `172.18.0.1:8080` (kind bridge gateway = host VM `10.139.218.43`) |
| MPS | `orch-infra` namespace | AMT management relay |
| Keycloak | `orch-platform` namespace | Issues JWT used by orch-cli to call MPS REST |
| AMT Device | Physical/virtual machine | Sends KVM screen frames, receives input |

### Network paths

| Traffic | Path |
|---|---|
| Browser → Angular static files | `kvm.<domain>` → Traefik → nginx pod :80 |
| Browser → orch-cli REST API | `kvm.<domain>/api/*` → Traefik → Endpoints `172.18.0.1:8080` (host VM `10.139.218.43` via kind bridge) |
| Browser → orch-cli WebSocket | `kvm.<domain>/ws` → Traefik → Endpoints `172.18.0.1:8080` (host VM `10.139.218.43` via kind bridge) |
| orch-cli → MPS REST | `https://mps-wss.<domain>/api/v1/...` (JWT auth via `validate-jwt` middleware) |
| orch-cli → MPS WebSocket | `wss://mps-wss.<domain>/relay/...` (redirect token, no JWT check — bypass IngressRoute) |

---

## 2. Existing EMF Infrastructure Used

These components are already running in the live cluster and require NO changes:

```
NAMESPACE      COMPONENT                    ENDPOINT                                  PURPOSE
-----------    -------------------------    ----------------------------------------  -------------------------
orch-gateway   traefik                      172.18.255.236:443                        TLS entry point for all traffic
orch-gateway   validate-jwt middleware       reads Cookie:jwt or Bearer token          JWT validation at edge
orch-gateway   amt-api-mps-kvm-bypass IR    mps-wss.<domain>/relay/* no JWT           WebSocket relay bypass
orch-gateway   amt-api-mps IR               mps-wss.<domain>/* + validate-jwt         MPS REST
orch-infra     mps                          ClusterIP :3000                           AMT management proxy
orch-platform  platform-keycloak            :8080 realm=master                        OIDC token issuer
```

Domain confirmed from live cluster: `orch-10-139-218-43.pid.infra-host.com`

MPS host used by orch-cli: `mps-wss.orch-10-139-218-43.pid.infra-host.com`

---

## 3. orch-cli — kvm-server Subcommand

### 3.1 What needs to be added

orch-cli currently handles AMT profiles via RPS.  A new `kvm-server` subcommand
is added that starts an HTTP server — the same logic as `kvm-poc/server/kvm_server.go`
— but wired into orch-cli's existing auth/config system.

**File to create: `orch-cli/internal/cli/kvm_server.go`**

Key difference from standalone kvm_server.go:
- `kvm-server` cobra command reads `--port`, `--allowed-origin` flags
- Auth token is fetched internally via orch-cli's existing `auth.GetAccessToken(ctx)`
- The browser does NOT need to send a JWT in `POST /api/connect` — orch-cli already
  holds a valid refresh token from `orch-cli login`
- `ConnectRequest` body simplifies to `{ deviceGuid, mpsHost, port, mode }`

```go
// orch-cli/internal/cli/kvm_server.go
func getKVMServerCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "kvm-server",
        Short: "Start the KVM relay HTTP server (bridges browser WebSocket to MPS)",
        Long: `Starts an HTTP server on the configured port.
The Angular KVM UI sends REST calls to this server; it
relays KVM sessions to MPS using credentials from 'orch-cli login'.`,
        RunE: runKVMServer,
    }
    cmd.Flags().String("port",           "8080",  "HTTP listen port")
    cmd.Flags().String("allowed-origin", "*",     "CORS allowed origin (set to https://kvm.<domain> in production)")
    return cmd
}

func runKVMServer(cmd *cobra.Command, _ []string) error {
    port, _          := cmd.Flags().GetString("port")
    origin, _        := cmd.Flags().GetString("allowed-origin")
    apiEndpoint      := viper.GetString("api-endpoint")  // e.g. https://api.orch-...

    cfg := KVMServerConfig{
        Port:          port,
        AllowedOrigin: origin,
        APIEndpoint:   apiEndpoint,
    }
    return StartKVMServer(cfg)
}
```

In `root.go`, register the command:
```go
rootCmd.AddCommand(getKVMServerCommand())
```

### 3.2 Token ownership — key architectural difference

In the dev/PoC setup the browser supplies the JWT in the POST body.
In the EMF integration, **orch-cli owns the token**:

```
DEV setup:
  Browser  --POST /api/connect { jwtToken: "eyJ..." }-->  kvm_server
  kvm_server uses the jwtToken from the request body to call MPS

EMF setup:
  Browser  --POST /api/connect { deviceGuid, mpsHost }-->  orch-cli kvm-server
  orch-cli kvm-server calls auth.GetAccessToken(ctx) internally
  --> exchanges stored refresh token with Keycloak --> gets access token
  --> uses it to call MPS REST API (no JWT ever leaves the browser)
```

This is more secure: the JWT never travels over the network in a request body.

### 3.3 Running orch-cli kvm-server

**Step 1 — Login once (stores refresh token locally)**
```bash
orch-cli login admin \
  --api-endpoint https://api.orch-10-139-218-43.pid.infra-host.com
```
This stores the Keycloak refresh token in `~/.orch-cli/orch-cli.yaml`.
It is valid until the Keycloak session max timeout (default: several hours).

**Step 2 — Start the KVM relay server**
```bash
orch-cli kvm-server \
  --port 8080 \
  --allowed-origin "https://kvm.orch-10-139-218-43.pid.infra-host.com"
# Server listening on :8080
```

The server binds to `0.0.0.0:8080`.  It is reachable on all interfaces:

| Who is connecting | Address to use |
|---|---|
| Local browser on the VM | `http://localhost:8080` |
| Pods inside the kind cluster | `http://172.18.0.1:8080` (kind bridge gateway = this VM) |
| External browser (direct test) | `http://10.139.218.43:8080` (VM external IP) |

The `172.18.0.1` address is the reliable one for K8S Endpoints because it is
the docker bridge gateway that every kind pod has a route to.  `10.139.218.43`
is the VM's external IP — it also works but requires the pod's network to have
a route back to the external interface.

---

## 4. Angular UI — Kubernetes Pod

### 4.1 Build Angular for production

```bash
cd kvm-poc/kvm-angular-app
npm run build
# Output: dist/kvm-angular-app/browser/
```

### 4.2 nginx configuration — replaces proxy.conf.json

In production, nginx inside the pod does what `proxy.conf.json` does in dev.
`/api` and `/ws` are proxied to orch-cli; everything else serves the Angular SPA.

```nginx
# nginx-kvm.conf  (baked into container image)
server {
    listen 80;

    root /usr/share/nginx/html;
    index index.html;

    # Angular SPA fallback
    location / {
        try_files $uri $uri/ /index.html;
    }

    # REST API  →  orch-cli on host (via K8S Service, resolved by nginx in-pod)
    # In-pod DNS resolves kvm-orch-cli.orch-ui.svc → ClusterIP → Endpoints 172.18.0.1:8080 (kind bridge = host VM 10.139.218.43)
    location /api/ {
        proxy_pass         http://kvm-orch-cli.orch-ui.svc.cluster.local:8080;
        proxy_set_header   Host $host;
        proxy_read_timeout 60s;
    }

    # WebSocket relay  →  orch-cli on host
    location = /ws {
        proxy_pass         http://kvm-orch-cli.orch-ui.svc.cluster.local:8080;
        proxy_http_version 1.1;
        proxy_set_header   Upgrade    $http_upgrade;
        proxy_set_header   Connection "Upgrade";
        proxy_read_timeout 3600s;
    }
}
```

Note: The proxy target uses the in-cluster DNS name for orch-cli.
This means the nginx pod never needs to know the host IP directly.

### 4.3 Dockerfile

```dockerfile
# kvm-poc/Dockerfile.angular-ui

FROM nginx:1.27-alpine

# Copy Angular build output
COPY kvm-angular-app/dist/kvm-angular-app/browser /usr/share/nginx/html

# Replace default nginx config
COPY nginx-kvm.conf /etc/nginx/conf.d/default.conf
REMOVE /etc/nginx/conf.d/default.conf  # remove upstream default

EXPOSE 80
```

Build and push:
```bash
cd kvm-poc/
docker build -f Dockerfile.angular-ui -t <your-registry>/kvm-angular-ui:latest .
docker push <your-registry>/kvm-angular-ui:latest
```

---

## 5. Kubernetes Manifests

All manifests go in `kvm-poc/k8s/`.  Apply with:
```bash
kubectl apply -f kvm-poc/k8s/
```

### 5.1 Namespace label (use existing orch-ui)

`orch-ui` namespace already exists in the cluster.  No namespace manifest needed.

### 5.2 K8S Endpoints + Service — bridge to orch-cli on host

This is the key component that lets Traefik (and the nginx pod) reach
orch-cli running outside K8S on the host machine.

```yaml
# kvm-poc/k8s/kvm-orch-cli-endpoints.yaml
# SPDX-License-Identifier: Apache-2.0

# Manual Endpoints: tells K8S that this "service" lives at the host machine.
#
# IP address explanation:
#   Host VM external IP : 10.139.218.43
#   Kind bridge gateway : 172.18.0.1  (how pods reach the host VM)
#
# Use 172.18.0.1 here — it is the docker bridge gateway and is always
# reachable from any pod in a kind cluster without extra routing.
# Verify: docker network inspect kind | grep Gateway
apiVersion: v1
kind: Endpoints
metadata:
  name: kvm-orch-cli
  namespace: orch-ui
subsets:
  - addresses:
      - ip: 172.18.0.1   # kind bridge gateway = host VM 10.139.218.43
    ports:
      - port: 8080
        protocol: TCP
---
apiVersion: v1
kind: Service
metadata:
  name: kvm-orch-cli
  namespace: orch-ui
spec:
  ports:
    - port: 8080
      targetPort: 8080
      protocol: TCP
  # No selector — controlled by the manual Endpoints object above
```

### 5.3 Angular UI — Deployment + Service

```yaml
# kvm-poc/k8s/kvm-angular-ui.yaml
# SPDX-License-Identifier: Apache-2.0

apiVersion: apps/v1
kind: Deployment
metadata:
  name: kvm-angular-ui
  namespace: orch-ui
  labels:
    app: kvm-angular-ui
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kvm-angular-ui
  template:
    metadata:
      labels:
        app: kvm-angular-ui
    spec:
      containers:
        - name: nginx
          image: <your-registry>/kvm-angular-ui:latest
          ports:
            - containerPort: 80
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 200m
              memory: 128Mi
---
apiVersion: v1
kind: Service
metadata:
  name: kvm-angular-ui
  namespace: orch-ui
spec:
  selector:
    app: kvm-angular-ui
  ports:
    - port: 80
      targetPort: 80
      protocol: TCP
```

### 5.4 Traefik IngressRoute

```yaml
# kvm-poc/k8s/kvm-ingressroute.yaml
# SPDX-License-Identifier: Apache-2.0

apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: kvm-ui
  namespace: orch-gateway
  annotations:
    description: "KVM Angular UI + orch-cli relay server"
spec:
  entryPoints:
    - websecure
  routes:
    # --- Priority 20: API calls to orch-cli (binary on host) ---
    - kind: Rule
      match: Host(`kvm.orch-10-139-218-43.pid.infra-host.com`) && PathPrefix(`/api`)
      priority: 20
      services:
        - name: kvm-orch-cli
          namespace: orch-ui
          port: 8080
          scheme: http

    # --- Priority 20: WebSocket relay to orch-cli ---
    - kind: Rule
      match: Host(`kvm.orch-10-139-218-43.pid.infra-host.com`) && Path(`/ws`)
      priority: 20
      services:
        - name: kvm-orch-cli
          namespace: orch-ui
          port: 8080
          scheme: http

    # --- Priority 10: Angular static files ---
    - kind: Rule
      match: Host(`kvm.orch-10-139-218-43.pid.infra-host.com`) && PathPrefix(`/`)
      priority: 10
      services:
        - name: kvm-angular-ui
          namespace: orch-ui
          port: 80
          scheme: http

  tls:
    secretName: tls-orch   # existing wildcard TLS cert
```

**Why no `validate-jwt` middleware on the KVM IngressRoute?**

JWT validation is handled inside orch-cli, not at Traefik level.
orch-cli uses its own stored refresh token to call MPS — it never asks the
browser for a JWT.  The browser-to-orch-cli channel is private (same LAN or
same host machine).

**Note on the WebSocket route**: No special `Upgrade` middleware is needed in
Traefik v3 — the IngressRoute handles WebSocket upgrade automatically when the
client sends `Connection: Upgrade`.

---

## 6. Angular App — Environment Configuration

In production, the Angular app must use relative paths (not `localhost:4200`).
The WebSocket URL must use `wss://` and the current page hostname:

```typescript
// kvm-poc/kvm-angular-app/src/environments/environment.prod.ts
export const environment = {
  production: true,
  // Empty string = use relative paths; nginx + Traefik route them correctly
  apiBase: '',
  wsBase: '',   // derived at runtime from window.location
};
```

In `kvm.service.ts`, the WebSocket URL should be built from the current origin:

```typescript
// Use the page's own hostname so it works both locally and in production
const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
const wsUrl = `${wsProtocol}//${window.location.host}/ws`;
this.socket = new WebSocket(wsUrl);
```

This works for both environments:
- Dev: `ws://localhost:4200/ws` → proxy.conf.json forwards to orch-cli :8080
- Prod: `wss://kvm.<domain>/ws` → Traefik → kvm-orch-cli Endpoints → orch-cli :8080

---

## 7. Complete Request Flow — EMF Production

### 7.1 Prerequisites

```
[host machine]
  orch-cli login admin --api-endpoint https://api.orch-...  # done once
  orch-cli kvm-server --port 8080                           # running as daemon

[K8S cluster]
  kubectl apply -f kvm-poc/k8s/                             # all manifests applied
  kvm-angular-ui pod: Running
  kvm-orch-cli Service + Endpoints: applied (172.18.0.1:8080 = host VM 10.139.218.43)
  kvm-ui IngressRoute: applied
```

### 7.2 Step-by-step browser flow

```
Step 1 — User opens browser
  https://kvm.orch-10-139-218-43.pid.infra-host.com
       |
       | DNS resolves to Traefik's MetalLB IP: 172.18.255.236
       | TLS terminates using existing tls-orch wildcard cert
       |
       v Traefik routes: PathPrefix(/) priority 10 → kvm-angular-ui pod :80
       nginx serves index.html → Angular app loads in browser

Step 2 — User fills in Device GUID
  (mpsHost is pre-configured in orch-cli; browser only needs deviceGuid)
  User types deviceGuid: "a1b2c3d4-..."
  User clicks "Request Consent Code"

Step 3 — Consent request
  Browser  GET  /api/consent/a1b2c3d4           (relative path)
       |
       v Traefik routes: PathPrefix(/api) priority 20 → kvm-orch-cli :8080
       v K8S Endpoints ip=172.18.0.1 → orch-cli on host VM 10.139.218.43 :8080
       |
  orch-cli handleConsentGet()
       |  calls auth.GetAccessToken(ctx)
       |    reads refresh token from ~/.orch-cli/orch-cli.yaml
       |    POST Keycloak /realms/master/protocol/openid-connect/token
       |    returns access_token (JWT)
       |
       |  GET https://mps-wss.<domain>/api/v1/amt/kvm/{guid}
       |  Authorization: Bearer <access_token>
       |
       v MPS tells AMT device to display 6-digit consent code
  Browser ← 200 OK → shows code input field

Step 4 — User submits consent code
  Browser  POST /api/consent/a1b2c3d4  { "consentCode": "123456" }
       v Traefik → kvm-orch-cli Endpoints → orch-cli :8080
  orch-cli handleConsentPost()
       |  re-uses cached access token (or re-fetches if expired)
       |  POST https://mps-wss.<domain>/api/v1/amt/kvm/{guid}  { consentCode }
       v 200 OK → browser enables "Connect KVM" button

Step 5 — User clicks "Connect KVM"
  Browser  POST /api/connect  { "deviceGuid": "a1b2c3d4", "mpsHost": "mps-wss.<domain>", "port": 16994 }
       v Traefik → kvm-orch-cli Endpoints → orch-cli :8080
  orch-cli handleConnect()
       |
       | A) Get redirect token from MPS REST
       |    auth.GetAccessToken(ctx) → JWT
       |    GET https://mps-wss.<domain>/api/v1/authorize/redirection/a1b2c3d4
       |    Cookie: jwt=<accessToken>
       |    ← { "token": "eyJredirect..." }
       |
       | B) Open MPS WebSocket with redirect token
       |    wss://mps-wss.<domain>/relay/webrelay.ashx?p=2&host=a1b2c3d4&port=16994&mode=kvm
       |    Sec-WebSocket-Protocol: <redirectToken>
       |    ← Traefik amt-api-mps-kvm-bypass IngressRoute:
       |      /relay/* → MPS :3000  (NO JWT check — redirect token sufficient)
       |
       | C) AMT Redirect handshake over MPS WebSocket
       |    RedirectStart → DigestAuth → ChannelOpen
       |    state = "active"
       |
       v  200 OK returned to browser

Step 6 — Browser opens WebSocket
  Browser  WebSocket wss://kvm.<domain>/ws
       v Traefik routes: Path(/ws) priority 20 → kvm-orch-cli Endpoints → orch-cli :8080
  orch-cli handleKVMWebSocket()
       - upgrades HTTP to WebSocket
       - attaches browser conn to session
       - flushes any RFB frames queued since step 5 completed
       - starts ping loop (30s keepalive)
       - starts readFromBrowser() relay loop

Step 7 — RFB handshake (transparent relay)
  AMT  "RFB 003.008\n"  →  orch-cli  →  browser
  browser  "RFB 003.008\n"  →  orch-cli  →  AMT
  ... security / ClientInit / ServerInit ...
  browser  SetEncodings [Raw, KvmDataChannel, DesktopSize]  →  AMT
  browser  FramebufferUpdateRequest (full)  →  AMT

Step 8 — Live KVM session (data plane loop)
  AMT  FramebufferUpdate (RGB565 tiles)  →  orch-cli  →  browser  →  canvas.putImageData()
  User  mouse/keyboard  →  browser encodes RFB PointerEvent/KeyEvent
                        →  orch-cli encodeUTF8Binary  →  MPS  →  AMT
```

---

## 8. Token Flow Detail

```
+---------------------------+        +------------------+        +----------+
|  orch-cli binary          |        |  MPS             |        | Keycloak |
|  (host machine)           |        |  (orch-infra ns) |        |(orch-plat)|
+---------------------------+        +------------------+        +----------+
          |                                   |                       |
          | 1. user ran "orch-cli login"       |                       |
          |----------------------------------------->>(stored locally) |
          |   refresh_token saved in            |                       |
          |   ~/.orch-cli/orch-cli.yaml         |                       |
          |                                   |                       |
          | 2. handleConnect() called          |                       |
          |   auth.GetAccessToken(ctx)         |                       |
          |   POST /realms/master/protocol/    |                       |
          |        openid-connect/token        |                       |
          |        grant_type=refresh_token    |                       |
          |------------------------------------------------------------>|
          |<- access_token (JWT, ~5min TTL) ----------------------------|
          |                                   |                       |
          | 3. GET /api/v1/authorize/          |                       |
          |    redirection/{guid}              |                       |
          |    Cookie: jwt=<access_token>      |                       |
          |---------------------------------->|                       |
          |<--- { "token": "<redirectToken>" }|                       |
          |                                   |                       |
          | 4. WebSocket UPGRADE              |                       |
          |    wss://.../relay/webrelay.ashx  |                       |
          |    Sec-WebSocket-Protocol:         |                       |
          |       <redirectToken>             |                       |
          |---------------------------------->|                       |
          |<======= relay open (RFB) ========>|                       |
```

**Nothing from Keycloak or MPS ever touches the browser** in this architecture.
The browser only sends a `{ deviceGuid }` to orch-cli.

---

## 9. Networking Summary

| Source | Destination | How |
|---|---|---|
| Browser | `kvm.<domain>:443` | HTTPS → MetalLB `172.18.255.236` → Traefik |
| Traefik | nginx pod :80 | ClusterIP `kvm-angular-ui.orch-ui.svc` |
| Traefik | orch-cli :8080 | Endpoints `172.18.0.1:8080` → host VM `10.139.218.43` via kind bridge |
| nginx pod | orch-cli :8080 | `kvm-orch-cli.orch-ui.svc.cluster.local:8080` → same Endpoints |
| orch-cli | MPS REST | `https://mps-wss.<domain>/api/...` → Traefik → `validate-jwt` → MPS pod |
| orch-cli | MPS WebSocket | `wss://mps-wss.<domain>/relay/...` → `amt-api-mps-kvm-bypass` IR → MPS pod (no JWT check) |

**IP address reference for this deployment:**

| Address | What it is |
|---|---|
| `10.139.218.43` | Host VM external IP — what the DNS name resolves to externally |
| `172.18.0.1` | Kind bridge gateway — how pods inside the cluster reach the host VM |
| `172.18.0.2` | Kind control-plane node IP |
| `172.18.255.236` | Traefik MetalLB external IP |

Verify the bridge gateway if the kind cluster is re-created:
```bash
docker network inspect kind | grep Gateway
```
If it changes, update only the Endpoints manifest (`kvm-orch-cli-endpoints.yaml`).

---

## 10. Running the Full Stack

### 10.1 One-time setup (host machine)

```bash
# Build orch-cli with kvm-server subcommand
cd 27-March/orch-cli
go build -o orch-cli ./cmd/orch-cli

# Login (stores refresh token)
./orch-cli login admin \
  --api-endpoint https://api.orch-10-139-218-43.pid.infra-host.com

# Verify token works
./orch-cli list host --project <your-project>
```

### 10.2 One-time setup (Kubernetes)

```bash
# Build Angular
cd kvm-poc/kvm-angular-app
npm install && npm run build

# Build container image
cd kvm-poc/
docker build -f Dockerfile.angular-ui -t <registry>/kvm-angular-ui:latest .
docker push <registry>/kvm-angular-ui:latest

# Deploy K8S manifests
kubectl apply -f kvm-poc/k8s/

# Verify
kubectl get pod -n orch-ui
kubectl get svc,endpoints -n orch-ui
kubectl get ingressroute kvm-ui -n orch-gateway
```

### 10.3 Start KVM relay server

```bash
cd 27-March/orch-cli
./orch-cli kvm-server --port 8080 \
  --allowed-origin "https://kvm.orch-10-139-218-43.pid.infra-host.com"
# [INFO] KVM relay server listening on :8080
```

### 10.4 Open the UI

Open Chrome or Edge:
```
https://kvm.orch-10-139-218-43.pid.infra-host.com
```

Enter the AMT device GUID, click "Request Consent Code", follow the 3-step
consent → connect flow.

---

## 11. File Structure to Create

```
kvm-poc/
  k8s/
    kvm-orch-cli-endpoints.yaml     # Endpoints + Service pointing to host :8080
    kvm-angular-ui.yaml             # Deployment + Service for nginx pod
    kvm-ingressroute.yaml           # Traefik IngressRoute for kvm.<domain>
  Dockerfile.angular-ui             # Multi-stage: Angular build + nginx runtime
  nginx-kvm.conf                    # nginx config replacing proxy.conf.json

27-March/orch-cli/
  internal/cli/
    kvm_server.go                   # NEW: cobra subcommand + KVMServerConfig wiring
                                    # logic reused from kvm-poc/server/kvm_server.go
```

Changes needed in existing files:
```
orch-cli/internal/cli/root.go  →  add: rootCmd.AddCommand(getKVMServerCommand())
kvm-poc/kvm-angular-app/src/app/services/kvm.service.ts
  →  build WebSocket URL from window.location.host  (not hardcoded localhost:4200)
```

---

## 12. Troubleshooting

| Symptom | Check | Likely cause |
|---|---|---|
| `kvm.<domain>` shows 404 | `kubectl get ingressroute kvm-ui -n orch-gateway` | IngressRoute not applied |
| Angular loads but `/api/connect` returns 502 | `kubectl exec -it <nginx-pod> -n orch-ui -- curl http://172.18.0.1:8080/api/status` | orch-cli not running on host VM `10.139.218.43`, or Endpoints has wrong IP |
| `/api/connect` returns 401 from MPS | orch-cli logs | Refresh token expired — run `orch-cli login admin` again |
| `/api/connect` returns 500 with "redirect token" | orch-cli logs | MPS REST unreachable or JWT has wrong `ActiveProjectID` |
| WebSocket `wss://.../ws` closes immediately | orch-cli logs for "no active session" | Browser opened WS before `POST /api/connect` completed |
| WebSocket upgrade fails at Traefik | Browser Network → WS → 400/426 | `proxy_http_version 1.1` missing in nginx conf, or Traefik route missing Path `/ws` |
| Canvas blank, no tiles | Browser console `[RFB]` messages | RFB handshake stalled; check orch-cli sees `handleAMTProtocol` "active" |
| `172.18.0.1` connection refused from inside pod | `curl http://172.18.0.1:8080/api/status` from pod shell | orch-cli binary not running on host VM `10.139.218.43`, or firewall blocking kind bridge `172.18.0.0/16` |
