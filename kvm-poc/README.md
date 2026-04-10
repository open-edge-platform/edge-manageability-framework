# KVM PoC — Intel AMT Remote Desktop Viewer

A proof-of-concept browser-based KVM (Keyboard, Video, Mouse) viewer for Intel AMT
devices, using a Go WebSocket relay and an Angular frontend.

| Directory | Language | Role |
|---|---|---|
| `server/` | Go | Control-plane REST + data-plane WebSocket relay |
| `kvm-angular-app/` | Angular 17 | Browser UI — consent flow, connection form, KVM canvas |

---

## Prerequisites

| Tool | Version |
|---|---|
| Go | 1.21+ |
| Node.js | 18+ |
| npm | 9+ |

Runtime access needed:
- **MPS host** — Management Presence Server hostname (e.g. `mps.example.com`)
- **Device GUID** — UUID of the target AMT device
- **JWT token** — bearer token issued by the orchestration stack for MPS API access

---

## Quick Start

**Terminal 1 — Go relay server**
```bash
cd server
go build -o kvm_server kvm_server.go
./kvm_server
# Listens on http://localhost:8080
```

**Terminal 2 — Angular viewer**
```bash
cd kvm-angular-app
npm install
npm start
# Served on http://localhost:4200
# /api and /ws are proxied to localhost:8080 via proxy.conf.json
```

Open **http://localhost:4200** in Chrome or Edge.

---

## UI Flow and Connection Steps

### Step 1 — Enter connection details

Open **http://localhost:4200** and fill in the three required fields:

| Field | Description |
|---|---|
| **MPS Host** | Hostname of the Management Presence Server |
| **Device GUID** | UUID of the target AMT device |
| **JWT Token** | Bearer token for MPS REST API authorization |

---

### Step 2 — Request user consent (Control Plane)

AMT requires explicit consent from the device user before a KVM session can open.

1. Click **Request Consent Code**.
2. The browser sends `GET /api/consent/{deviceGuid}` with the JWT token to the Go server.
3. The Go server forwards the request to the MPS REST API: `GET /api/v1/amt/kvm/{guid}`.
4. MPS instructs the AMT device to display a **6-digit code** on its physical screen.
5. The Go server returns 200 and the browser shows an input field for the code.

---

### Step 3 — Submit the consent code (Control Plane)

1. The device user reads the 6-digit code from the AMT device screen and tells the operator.
2. Enter the 6-digit code in the browser and click **Submit Code**.
3. The browser sends `POST /api/consent/{deviceGuid}` with `{ consentCode: "NNNNNN" }` and the JWT token.
4. The Go server forwards to MPS REST API: `POST /api/v1/amt/kvm/{guid}`.
5. MPS validates the code and returns 200 — consent is approved.
6. The **Connect KVM** button becomes active.

---

### Step 4 — Connect KVM session (Control Plane)

1. Click **Connect KVM**.
2. The browser sends `POST /api/connect` to the Go server with the payload:
   ```json
   {
     "mpsHost":    "<mps-hostname>",
     "deviceGuid": "<device-uuid>",
     "port":       16994,
     "mode":       "kvm",
     "jwtToken":   "<jwt>"
   }
   ```
3. The Go server uses the JWT to fetch an AMT redirect token from MPS REST.
4. The Go server opens a WebSocket to MPS: `wss://<mpsHost>/relay/webrelay.ashx?...`
5. The Go server completes the AMT Redirect protocol handshake over that WebSocket:
   - Sends `RedirectStart`
   - Performs Digest authentication
   - Sends `ChannelOpen` to activate the KVM channel on the AMT device
6. The Go server returns `200 OK` to the browser — the KVM channel is now open.

---

### Step 5 — Live screen and input (Data Plane)

1. On receiving 200, the browser opens a WebSocket: `ws://localhost:8080/ws`.
2. The Go server connects both ends together — browser WebSocket ↔ MPS WebSocket — and relays bytes in both directions.

**Screen data (AMT → Browser):**
- AMT sends RFB `FramebufferUpdate` messages containing 64×64 pixel tiles (Raw encoding, RGB565).
- Go server relays the frames → browser receives binary WebSocket messages.
- The Angular viewer decodes the tiles and draws them on an HTML5 canvas.

**Input events (Browser → AMT):**
- Mouse move/click on the canvas → Angular sends RFB `PointerEvent` as binary WebSocket frame.
- Keypress on the canvas → Angular sends RFB `KeyEvent` as binary WebSocket frame.
- Go server relays both to AMT device via MPS, which controls the remote desktop.

---

## Project Structure

```
kvm-poc/
├── README.md
├── server/
│   ├── kvm_server.go     ← Go: REST /api/connect + /api/consent proxy + WebSocket relay
│   └── go.mod
└── kvm-angular-app/
    ├── proxy.conf.json   ← Dev proxy: /api/* and /ws → localhost:8080
    └── src/app/
        ├── app.ts        ← Form inputs, consent flow, connect/disconnect
        ├── app.html      ← UI template
        ├── services/
        │   └── kvm.service.ts          ← POST /api/connect, opens WebSocket /ws
        └── components/
            └── kvm-viewer.component.ts ← Renders RFB frames on canvas, sends input events
```

---

## Troubleshooting

| Symptom | Cause | Action |
|---|---|---|
| **Connect KVM** button disabled | Consent not yet approved | Complete the 6-digit consent code step first |
| Consent request fails 401 | JWT token invalid or expired | Re-issue JWT token from the orchestration stack |
| Canvas blank after connect | Go server not reachable | Confirm `./kvm_server` is running on port 8080 |
| WebSocket closes immediately | MPS host unreachable | Verify MPS host and network/firewall access |
| Keyboard input not working | Canvas lost focus | Click the canvas once; it auto-focuses on mouse enter |

---


