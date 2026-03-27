# KVM Angular Integration Guide
## Browser → Go Server → MPS → AMT: Deep Dive

This guide explains exactly how every layer of the KVM stack communicates —
from the user filling in the browser form, through the HTTP handlers and Go server,
all the way to the AMT device sending back pixel data.

---

## Architecture Overview

```
+------------------------------------------------------------------+
|  BROWSER  (Angular -- port 4200)                                 |
|                                                                  |
|  app.ts                                                          |
|    Consent flow  ----------->  POST /api/consent/{guid}          |
|    Connect KVM   ----------->  POST /api/connect                 |
|                                        |                         |
|  kvm.service.ts                        |                         |
|    WebSocket /ws  <--------------------+                         |
|         |    (binary frames, RFB protocol)                       |
|         v                                                        |
|  kvm-viewer.component.ts                                         |
|    - Draws pixel tiles onto <canvas>                             |
|    - Sends PointerEvent / KeyEvent back via WebSocket            |
+------------------------------------------------------------------+
                          |
          proxy.conf.json | /api --> :8080   /ws --> :8080
                          |
+------------------------------------------------------------------+
|  GO SERVER  kvm_server.go  (port 8080)                           |
|                                                                  |
|  HTTP Handlers                                                   |
|    POST /api/connect        -->  handleConnect()                 |
|    POST /api/disconnect     -->  handleDisconnect()              |
|    GET  /api/status         -->  handleStatus()                  |
|    GET  /api/consent/{guid} -->  handleConsentGet()              |
|    POST /api/consent/{guid} -->  handleConsentPost()             |
|    GET  /ws                 -->  handleKVMWebSocket()            |
|                                                                  |
|  KVMSession (internal)                                           |
|    sendToMPS()      -- UTF-8 encodes binary, sends text frame    |
|    readFromMPS()    -- UTF-8 decodes text frame -> raw binary    |
|    sendToBrowser()  -- relays raw binary to browser WebSocket    |
+------------------------------------------------------------------+
                          |
                          |  wss://<mpsHost>/relay/webrelay.ashx
                          |  WebSocket TEXT frames (UTF-8 binary)
                          |  Sec-WebSocket-Protocol: <redirect-token>
                          |
+------------------------------------------------------------------+
|  MPS                                                             |
|                                                                  |
|  REST  GET  /api/v1/authorize/redirection/{guid}                 |
|        GET  /api/v1/amt/kvm/{guid}   (consent request)           |
|        POST /api/v1/amt/kvm/{guid}   (consent submit)            |
|                                                                  |
|  WS    wss://<mpsHost>/relay/webrelay.ashx?p=2&host=...          |
+------------------------------------------------------------------+
                          |
                          |  AMT Redirect Protocol
                          |  RedirectStart -> DigestAuth -> ChannelOpen
                          |  then: raw RFB bytes (transparent relay)
                          |
+------------------------------------------------------------------+
|  INTEL AMT DEVICE  (port 16994)                                  |
|                                                                  |
|  KVM / RFB 3.8 session                                           |
|    Screen --> FramebufferUpdate (320 x 64x64 tiles, RGB565)      |
|    Input  <-- PointerEvent / KeyEvent                            |
+------------------------------------------------------------------+
```

### Component roles

| Layer | Component | Role |
|---|---|---|
| Browser | `app.ts` | Form inputs, consent flow, connect/disconnect buttons |
| Browser | `kvm.service.ts` | REST calls to Go server + owns the WebSocket connection |
| Browser | `kvm-viewer.component.ts` | RFB state machine, canvas renderer, mouse/keyboard encoder |
| Proxy | `proxy.conf.json` | Forwards `/api` and `/ws` from port 4200 to port 8080 |
| Go Server | `handleConnect()` | Exchanges JWT for redirect token, opens MPS WebSocket, runs AMT handshake |
| Go Server | `handleKVMWebSocket()` | Upgrades browser HTTP to WebSocket, flushes queued frames, starts relay |
| Go Server | `KVMSession` | Owns both WebSocket connections; encodes/decodes UTF-8 binary transport |
| MPS | REST API | Issues redirect tokens, proxies consent requests to AMT device |
| MPS | WebSocket relay | Tunnels AMT Redirect protocol + RFB between Go server and AMT device |
| AMT | KVM engine | Sends screen tiles (RFB Raw encoding), receives pointer/key events |

---

## Layer 1 — Browser (Angular)

### 1.1 Connection form (`app.ts`)

The root component holds three user-supplied values:

```typescript
mpsHost    = ''   // MPS server hostname
deviceGuid = ''   // UUID of the target AMT device
jwtToken   = ''   // Bearer token for MPS REST API
```

None of these values are hard-coded in the committed code.
They are sent to the Go server via REST — the browser never connects to MPS directly.

### 1.2 Consent flow (`app.ts`)

AMT requires the physical device user to approve a KVM session.
The browser drives this with two REST calls:

**Request consent code:**
```
GET /api/consent/{deviceGuid}
Authorization: Bearer <jwtToken>
```
→ MPS tells the AMT device to display a 6-digit code on screen.
→ Browser shows an input box for the operator to type the code.

**Submit consent code:**
```
POST /api/consent/{deviceGuid}
Authorization: Bearer <jwtToken>
Content-Type: application/json
{ "consentCode": "123456" }
```
→ MPS validates. On 200 OK the **Connect KVM** button becomes active.

### 1.3 Connect (`kvm.service.ts`)

`connect()` runs a two-phase sequence:

**Phase 1 — Control plane (REST)**

```typescript
const resp = await fetch('/api/connect', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    mpsHost:    this.mpsHost,
    deviceGuid: this.deviceGuid,
    port:       16994,
    mode:       'kvm',
    jwtToken:   this.jwtToken,
  }),
});
// 200 OK → Go server has opened the MPS WebSocket and completed AMT handshake
```

**Phase 2 — Data plane (WebSocket)**

```typescript
this.socket = new WebSocket('ws://localhost:4200/ws');
this.socket.binaryType = 'arraybuffer';   // ← must be binary, not text

this.socket.onmessage = (event: MessageEvent) => {
  // Raw RFB bytes from AMT device — forwarded to KvmViewerComponent
  this.kvmData$.next(event.data as ArrayBuffer);
};
```

Mouse and keyboard events flow back:

```typescript
sendData(data: ArrayBuffer): void {
  if (this.socket?.readyState === WebSocket.OPEN) {
    this.socket.send(data);   // RFB PointerEvent or KeyEvent, encoded as binary
  }
}
```

### 1.4 KVM viewer component (`kvm-viewer.component.ts`)

The component implements the **RFB 3.8 client state machine** entirely in the browser:

| State | What happens |
|---|---|
| `ProtocolVersion` | Receives `"RFB 003.008\n"`, replies with same string |
| `Security` | Reads security type list, selects None (type 1), replies `0x01` |
| `SecurityResult` | Reads 4-byte result (must be 0), sends `ClientInit` (shared=1) |
| `ServerInit` | Reads canvas dimensions + pixel format (RGB565 16bpp), sends `SetEncodings` + first `FramebufferUpdateRequest` |
| `Normal` | Receives `FramebufferUpdate` messages, decodes Raw tiles, paints canvas |

**SetEncodings sent by browser:**
```
Raw (0), KvmDataChannel (1092), DesktopSize (-223)
```
ZRLE is intentionally omitted — AMT falls back to Raw, which the browser can decode directly.

**RGB565 fast-path decoder:**
```typescript
const v = dataView.getUint16(i * 2, true);  // little-endian
out[i*4 + 0] = (v >> 8) & 0xF8;   // Red   (bits 15-11)
out[i*4 + 1] = (v >> 3) & 0xFC;   // Green (bits 10-5)
out[i*4 + 2] = (v << 3) & 0xF8;   // Blue  (bits 4-0)
out[i*4 + 3] = 255;                // Alpha
```

**Mouse event encoding (RFB PointerEvent):**
```typescript
const msg = new Uint8Array(6);
msg[0] = 5;                            // message type = PointerEvent
msg[1] = buttonMask;                   // button bits: left=1, middle=2, right=4
new DataView(msg.buffer).setUint16(2, x, false);   // x coordinate
new DataView(msg.buffer).setUint16(4, y, false);   // y coordinate
this.sendToServer(msg.buffer);
```

**Keyboard event encoding (RFB KeyEvent):**
```typescript
const msg = new Uint8Array(8);
msg[0] = 4;                            // message type = KeyEvent
msg[1] = down ? 1 : 0;                 // 1 = down, 0 = up
new DataView(msg.buffer).setUint32(4, keysym, false);  // X11 keysym
this.sendToServer(msg.buffer);
```

---

## Layer 2 — Angular Proxy

The Angular server (`npm start`) uses `proxy.conf.json` to forward requests
to the Go server — no CORS issues during development:

```json
{
  "/api": {
    "target": "http://localhost:8080",
    "secure": false,
    "changeOrigin": true
  },
  "/ws": {
    "target": "ws://localhost:8080",
    "secure": false,
    "ws": true
  }
}
```

| Browser request | Proxied to Go server |
|---|---|
| `GET /api/status` | `http://localhost:8080/api/status` |
| `POST /api/connect` | `http://localhost:8080/api/connect` |
| `GET /api/consent/{guid}` | `http://localhost:8080/api/consent/{guid}` |
| `POST /api/consent/{guid}` | `http://localhost:8080/api/consent/{guid}` |
| `WS /ws` | `ws://localhost:8080/ws` |

---

## Layer 3 — Go Server (`kvm_server.go`)

### 3.1 Central structs

```go
// ConnectRequest — JSON body from browser POST /api/connect
type ConnectRequest struct {
    MPSHost    string `json:"mpsHost"`
    DeviceGUID string `json:"deviceGuid"`
    Port       int    `json:"port"`
    Mode       string `json:"mode"`
    JWTToken   string `json:"jwtToken"`
}

// KVMServer — singleton; owns the session and HTTP mux
type KVMServer struct {
    session  *KVMSession   // nil when disconnected
    mu       sync.RWMutex
    upgrader websocket.Upgrader
}

// KVMSession — one AMT KVM session
type KVMSession struct {
    mpsConn     *websocket.Conn  // connection to MPS
    browserConn *websocket.Conn  // connection to Angular browser
    state       string           // "start" → "auth" → "channel" → "active"
    done        chan struct{}     // closed on disconnect
}
```

### 3.2 HTTP handlers — control plane

**`POST /api/connect` → `handleConnect()`**

1. Decode `ConnectRequest` from JSON body.
2. Call MPS REST API to get an AMT redirect token:
   ```
   GET https://<mpsHost>/api/v1/amt/redirectionservice/{guid}
   Authorization: Bearer <jwtToken>
   → { "token": "<redirect-token>" }
   ```
3. Build the MPS WebSocket URL:
   ```
   wss://<mpsHost>/relay/webrelay.ashx?tls=0&p=2&host=<guid>&port=16994
   &mode=kvm&token=<redirect-token>
   ```
4. Open the MPS WebSocket (TLS, skip verify for internal MPS).
5. Start the AMT Redirect handshake in a goroutine (`handleAMTProtocol()`).
6. Return `200 OK` immediately — the browser can now open the `/ws` WebSocket.

**`GET /api/consent/{guid}` → `handleConsentGet()`**

Proxies to MPS REST:
```
GET https://<mpsHost>/api/v1/amt/kvm/{guid}
Authorization: Bearer <jwtToken>
```
Returns the MPS response to the browser.

**`POST /api/consent/{guid}` → `handleConsentPost()`**

Proxies to MPS REST:
```
POST https://<mpsHost>/api/v1/amt/kvm/{guid}
Authorization: Bearer <jwtToken>
{ "consentCode": "123456" }
```

**`GET /api/status` → `handleStatus()`**

```go
json.NewEncoder(w).Encode(StatusResponse{
    State:   session.getState(),   // "active", "connecting", etc.
    Logs:    srv.recentLogs(),
    Device:  session.deviceGUID,
    MPSHost: session.mpsHost,
})
```

**`GET /ws` → `handleKVMWebSocket()`**

This is the most important handler — it bridges the browser canvas to the live AMT session.
Here is what happens step by step when the browser calls `new WebSocket('ws://localhost:4200/ws')`:

**Step 1 — HTTP → WebSocket upgrade**
```go
conn, err := srv.upgrader.Upgrade(w, r, nil)
```
The HTTP GET on `/ws` is upgraded to a full-duplex WebSocket connection.
The `upgrader.CheckOrigin` always returns `true` so the Angular dev server
origin (`localhost:4200`) is never rejected.

**Step 2 — Guard: reject if no active session**
```go
if session == nil || session.getState() == "disconnected" || session.getState() == "error" {
    conn.Close()
    return
}
```
If the browser opens the WebSocket before `POST /api/connect` has been called
(or if the MPS connection failed), the connection is immediately closed.
This prevents the viewer from hanging in a connected-but-dead state.

**Step 3 — Attach browser connection to session**
```go
session.browserMu.Lock()
oldConn := session.browserConn
session.browserConn = conn   // ← THIS is when the session knows the browser is ready
session.browserMu.Unlock()
if oldConn != nil {
    oldConn.Close()  // replace stale connection from a previous viewer tab
}
```
`session.browserConn` is `nil` until this line runs. Any RFB data that arrived
from AMT before this point was queued in `pendingBrowserFrames` rather than dropped.

**Step 4 — Flush queued frames**
```go
session.flushPendingBrowserFrames()
```
This is explained in full detail below.

**Step 5 — Browser keepalive ping loop**
```go
go func() {
    ticker := time.NewTicker(30 * time.Second)
    for {
        conn.WriteControl(websocket.PingMessage, []byte("ping"), deadline)
    }
}()
```
A goroutine pings the browser WebSocket every 30 seconds. Without this, a static
screen (no AMT updates) would cause the browser or a proxy to drop the idle connection.
The browser's WebSocket API responds with a Pong automatically; each Pong resets
the 120‑second read deadline.

**Step 6 — Start relay loop**
```go
session.readFromBrowser()
```
This call **blocks** for the lifetime of the connection. It reads every binary
frame the browser sends (RFB PointerEvent / KeyEvent), UTF-8 encodes them, and
writes them to the MPS WebSocket. When the browser disconnects, `readFromBrowser()`
returns and the handler exits.

---

### Frame queuing

There is a **race window** between `/api/connect` returning 200 and the browser
opening the `/ws` WebSocket. In that window (typically 50–200 ms), AMT may have
already sent the first fragment of the RFB handshake (`"RFB 003.008\n"`) or even
the first `FramebufferUpdate`. If the Go server discarded those bytes, the RFB
state machine in the browser would never receive the protocol version string and
the handshake would stall forever.

The queue is a plain slice on the session:

```go
type KVMSession struct {
    browserConn          *websocket.Conn
    browserMu            sync.Mutex
    pendingBrowserFrames [][]byte   // ← the queue
    ...
}
```

**Enqueue path** — `sendToBrowser()` is called by `readFromMPS()` every time a
decoded frame arrives from AMT:

```go
func (s *KVMSession) sendToBrowser(data []byte) {
    s.browserMu.Lock()
    defer s.browserMu.Unlock()

    if s.browserConn == nil {
        // Browser not connected yet — save the frame
        copied := make([]byte, len(data))
        copy(copied, data)                              // deep copy — MPS buffer may be reused
        s.pendingBrowserFrames = append(s.pendingBrowserFrames, copied)
        s.log("[RFB] Queued %d bytes for browser", len(data))
        return
    }
    // Browser is connected — send directly
    s.browserConn.WriteMessage(websocket.BinaryMessage, data)
}
```

The `copy()` is critical — the underlying MPS read buffer is reused on the next
`ReadMessage()` call, so the slice must be deep-copied before being stored.

**Dequeue path** — `flushPendingBrowserFrames()` is called exactly once, in
`handleKVMWebSocket()`, immediately after `session.browserConn` is set:

```go
func (s *KVMSession) flushPendingBrowserFrames() {
    s.browserMu.Lock()
    defer s.browserMu.Unlock()

    if s.browserConn == nil || len(s.pendingBrowserFrames) == 0 {
        return
    }

    s.log("[RFB] Flushing %d queued browser frame(s)", len(s.pendingBrowserFrames))
    for _, frame := range s.pendingBrowserFrames {
        if err := s.browserConn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
            s.log("[WARN] Failed to flush queued frame: %v", err)
            break  // if send fails here, session is broken anyway
        }
    }
    s.pendingBrowserFrames = nil  // release memory
}
```

Because both `sendToBrowser()` and `flushPendingBrowserFrames()` hold `browserMu`,
there is no race: the flush drains the queue and from that point on `browserConn`
is non-nil, so new frames go directly to the browser via `sendToBrowser()`.

### 3.3 AMT Redirect handshake (`handleAMTProtocol()`)

The Go server speaks the AMT Redirect protocol over the MPS WebSocket before any
RFB data flows. The sequence is:

**Step 1 — RedirectStart**
```
Go → MPS: [0x10, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00]
           (8-byte StartRedirection command, type=0x10, mode=KVM)
MPS → Go: [0x11, 0x00, status, 0x00, ...]
           (StartRedirectionReply, status=0 means OK to proceed)
```

**Step 2 — Digest Authentication**
```
Go → MPS: [0x13, 0x00, 0x00, 0x00, ...]  (AuthenticateSessionRequest, type=query)
MPS → Go: [0x14, 0x00, challenge-data]   (AuthReply with Digest realm/nonce)

Go computes:
  HA1 = MD5(username + ":" + realm + ":" + password)
  HA2 = MD5("POST:" + "/RedirectionService")
  response = MD5(HA1 + ":" + nonce + ":" + HA2)

Go → MPS: [0x13, 0x00, 0x00, 0x00, ...digest-response...]
MPS → Go: [0x14, 0x00, 0x00, ...]  (AuthReply, status=0 = authenticated)
```

**Step 3 — ChannelOpen (KVM)**
```
Go → MPS: [0x40, 0x00, 0x00, 0x00, 0x27, 0x10, 0x00, 0x00]
           (OpenDataChannel, type=KVM)
```

After this, the MPS WebSocket carries raw RFB protocol bytes — the session is
`"active"` and becomes a transparent relay.

### 3.4 UTF-8 binary encoding — the hidden transport detail

MPS sends WebSocket **text frames** where bytes ≥ 0x80 are encoded as 2-byte
UTF-8 sequences. For example, `0xFF` arrives as `0xC3 0xBF`.

The Go server handles this transparently:

**MPS → Browser** (`readFromMPS()`):
```go
msgType, data, err := session.mpsConn.ReadMessage()
if msgType == websocket.TextMessage {
    data = decodeUTF8Binary(data)  // expand 2-byte sequences → raw bytes
}
session.sendToBrowser(data)        // send raw binary to browser
```

**Browser → MPS** (`readFromBrowser()`):
```go
_, data, err := session.browserConn.ReadMessage()
session.sendToMPS(data)  // encodes raw bytes → UTF-8 text frame for MPS
```

```go
func encodeUTF8Binary(src []byte) []byte {
    for _, b := range src {
        if b < 0x80 {
            dst = append(dst, b)
        } else {
            dst = append(dst, 0xC0|(b>>6), 0x80|(b&0x3F))
        }
    }
}
```

The browser never sees UTF-8 encoding — it always receives and sends raw binary.

### 3.5 Relay goroutines

Once `"active"`, two goroutines run concurrently:

```
goroutine 1: readFromMPS()
  loop:
    raw = mpsConn.ReadMessage()
    decoded = decodeUTF8Binary(raw)   // if text frame
    browserConn.WriteMessage(Binary, decoded)

goroutine 2: readFromBrowser()
  loop:
    raw = browserConn.ReadMessage()
    encoded = encodeUTF8Binary(raw)
    mpsConn.WriteMessage(Text, encoded)
```

A `done` channel is closed when either side disconnects, causing both goroutines
to exit cleanly.

### 3.6 Frame queuing (browser arrives late)

`/api/connect` returns 200 immediately after the AMT handshake completes.
The MPS session can start sending RFB data before the browser has opened `/ws`.
The Go server queues these frames:

```go
func (s *KVMSession) sendToBrowser(data []byte) {
    s.browserMu.Lock()
    defer s.browserMu.Unlock()
    if s.browserConn == nil {
        s.pendingBrowserFrames = append(s.pendingBrowserFrames, data)
        return
    }
    s.browserConn.WriteMessage(websocket.BinaryMessage, data)
}
```

When the browser WebSocket connects, `flushPendingBrowserFrames()` drains the
queue before the normal relay starts.

---

## JWT Token Flow and Redirect Token Exchange

Two separate tokens are in play. Understanding which token does what is essential
for debugging authentication failures.

### Token 1 — JWT (Keycloak / OIDC bearer token)

The JWT is issued by the orchestration stack's identity provider (Keycloak).
It authorises calls to the **MPS REST API**. The browser supplies it; the Go server
passes it on. It is never generated inside the KVM stack.

**Who holds it:** Browser → Go server (passed in every REST call body and proxied header).

**How it is used:**

| Call | How JWT is sent |
|---|---|
| `GET /api/v1/authorize/redirection/{guid}` | Cookie: `jwt=<token>` + header `ActiveProjectID: ` |
| `GET /api/v1/amt/kvm/{guid}` (consent request) | Header: `Authorization: Bearer <token>` |
| `POST /api/v1/amt/kvm/{guid}` (consent submit) | Header: `Authorization: Bearer <token>` |
| MPS WebSocket upgrade | Header: `Authorization: Bearer <token>` + Cookie: `jwt=<token>` |

```go
// getMPSRedirectToken — JWT sent as Cookie (MPS REST requirement)
req.AddCookie(&http.Cookie{Name: "jwt", Value: keycloakToken})
req.Header.Set("ActiveProjectID", "")

// MPS WebSocket upgrade — JWT sent both ways (belt-and-suspenders)
headers.Set("Cookie", fmt.Sprintf("jwt=%s", req.JWTToken))
headers.Set("Authorization", fmt.Sprintf("Bearer %s", req.JWTToken))
```

---

### Token 2 — Redirect Token (short-lived AMT session token)

The redirect token authoriises a **single WebSocket relay session** to a specific
AMT device. It is scoped to one device GUID and expires quickly.

**Who issues it:** MPS REST API.  
**Who requests it:** Go server (never the browser).  
**How it is used:** Sent as the `Sec-WebSocket-Protocol` header during the MPS WebSocket upgrade.

#### Exchange sequence

```
Go server
  │
  ├─ 1. Build URL:
  │      https://<mpsHost>/api/v1/authorize/redirection/<deviceGuid>
  │
  ├─ 2. GET request with JWT:
  │      Cookie: jwt=<keycloakToken>
  │      ActiveProjectID: (empty)
  │
  ├─ 3. MPS REST returns:
  │      { "token": "eyJ..." }   ← this is the redirect token
  │
  └─ 4. Use redirect token to open WebSocket to MPS:
         wss://<mpsHost>/relay/webrelay.ashx
           ?p=2
           &host=<deviceGuid>
           &port=16994
           &tls=0
           &tls1only=0
           &mode=kvm

         Headers sent on WebSocket upgrade:
           Sec-WebSocket-Protocol: <redirectToken>   ← redirect token here
           Cookie:                 jwt=<keycloakToken>
           Authorization:          Bearer <keycloakToken>
```

Go code that does this:

```go
func getMPSRedirectToken(mpsHost, deviceGUID, keycloakToken string) (string, error) {
    url := fmt.Sprintf("https://%s/api/v1/authorize/redirection/%s", mpsHost, deviceGUID)

    req, _ := http.NewRequest("GET", url, nil)
    req.AddCookie(&http.Cookie{Name: "jwt", Value: keycloakToken})
    req.Header.Set("ActiveProjectID", "")

    resp, _ := client.Do(req)
    // resp body: { "token": "<redirect-token>" }
    json.NewDecoder(resp.Body).Decode(&tokenResp)
    return tokenResp.Token, nil
}

// Then open MPS WebSocket with redirect token as Sec-WebSocket-Protocol
wsURL := fmt.Sprintf(
    "wss://%s/relay/webrelay.ashx?p=2&host=%s&port=%d&tls=0&tls1only=0&mode=%s",
    mpsHost, deviceGUID, port, "kvm")

headers := http.Header{}
headers.Set("Sec-WebSocket-Protocol", redirectToken)    // ← redirect token
headers.Set("Cookie",                 "jwt="+jwtToken)  // ← JWT (secondary)
headers.Set("Authorization",          "Bearer "+jwtToken)

conn, _, _ := dialer.Dial(wsURL, headers)
```

---

### Full token flow diagram

```
User / Orchestration Stack
         │
         │  issues JWT (Keycloak OIDC)
         ▼
    Browser (Angular)
         │
         │  POST /api/connect  { jwtToken: "eyJ..." }
         ▼
    Go Server
         │
         │  Step A — exchange JWT for redirect token
         │  GET https://<mpsHost>/api/v1/authorize/redirection/<guid>
         │  Cookie: jwt=<jwtToken>
         │
         ▼
    MPS REST API
         │  returns { "token": "<redirectToken>" }
         ▼
    Go Server
         │
         │  Step B — open relay WebSocket with redirect token
         │  wss://<mpsHost>/relay/webrelay.ashx?p=2&host=<guid>&port=16994&mode=kvm
         │  Sec-WebSocket-Protocol: <redirectToken>
         │  Authorization: Bearer <jwtToken>
         │
         ▼
    MPS WebSocket Relay
         │  (validates redirect token, opens pipe to AMT device)
         ▼
    AMT Device (port 16994)
         │  AMT Redirect Protocol handshake (Go server drives)
         │  RedirectStart → DigestAuth → ChannelOpen
         ▼
    RFB session active (pure relay)
         Browser ◄──── Go Server ◄──── MPS ◄──── AMT
```

---

### About Token

| | JWT | Redirect Token |
|---|---|---|
| **Issued by** | Keycloak (OIDC) | MPS REST API |
| **Lifetime** | Minutes–hours (configurable) | Seconds–minutes (single use) |
| **Scope** | All MPS REST API calls | One WebSocket relay to one device |
| **Sent by** | Browser → Go server (in POST body) | Go server only (never browser) |
| **Transport** | HTTP Authorization header / Cookie | `Sec-WebSocket-Protocol` header |
| **If expired** | All API calls fail 401 | WebSocket upgrade fails 401, need new redirect token |

---

## Layer 4 — MPS

MPS sits between the Go server and the AMT device. It provides:

| Interface | Used for |
|---|---|
| `GET /api/v1/amt/redirectionservice/{guid}` | Exchange JWT for AMT redirect token |
| `GET /api/v1/amt/kvm/{guid}` | Request user consent code |
| `POST /api/v1/amt/kvm/{guid}` | Submit and validate consent code |
| `wss://<host>/relay/webrelay.ashx?...` | WebSocket relay carrying AMT Redirect protocol + RFB |

The Go server is the only component that speaks to MPS directly.
The browser never contacts MPS.

---

## Layer 5 — AMT Device (RFB over AMT Redirect)

After the ChannelOpen message, AMT speaks standard **RFB 3.8** (VNC protocol)
over the relay channel. The full RFB handshake runs between the browser and AMT
device — the Go server does not parse or modify any RFB bytes.

```
AMT sends:  "RFB 003.008\n"       → Go relays → Browser receives
Browser:    "RFB 003.008\n"       → Go relays → AMT receives
AMT sends:  Security types [1]    → relayed →   Browser selects type 1 (None)
Browser:    ClientInit (shared=1) → relayed →   AMT receives
AMT sends:  ServerInit (1280x1024, RGB565) → relayed → Browser resizes canvas
Browser:    SetEncodings (Raw, KvmDataChannel, DesktopSize)  → AMT
Browser:    FramebufferUpdateRequest (full screen, incremental=0) → AMT
AMT sends:  FramebufferUpdate (320 × 64x64 Raw tiles) → Browser paints canvas
Browser:    FramebufferUpdateRequest (incremental=1)   → AMT (loop)
```

---

## Complete Request Trace

Here is the exact sequence of network calls when a user connects and sees the
screen:

```
1. User clicks "Request Consent Code"
   Browser  ──GET /api/consent/{guid}──►  Go server
   Go server ──GET /api/v1/amt/kvm/{guid}──►  MPS
   MPS ──► AMT device shows 6-digit code on screen
   Go server ◄── 200 OK ── MPS
   Browser ◄── 200 OK ── Go server (shows code input field)

2. User types code, clicks "Submit Code"
   Browser  ──POST /api/consent/{guid}──►  Go server
   Go server ──POST /api/v1/amt/kvm/{guid}──►  MPS
   Go server ◄── 200 OK ── MPS (consent validated)
   Browser ◄── 200 OK ── Go server → "Connect KVM" button unlocks

3. User clicks "Connect KVM"
   Browser  ──POST /api/connect──►  Go server
   Go server ──GET /redirectionservice/{guid}──►  MPS  (get redirect token)
   Go server ◄── { token: "..." } ── MPS
   Go server ──WebSocket UPGRADE──►  wss://mps/relay/webrelay.ashx?token=...
   Go server ◄──► MPS  (AMT Redirect handshake: RedirectStart → Auth → ChannelOpen)
   Browser ◄── 200 OK ── Go server

4. Browser opens WebSocket
   Browser  ──WebSocket UPGRADE /ws──►  Go server
   Go server flushes queued frames (if any) → Browser
   [relay active]

5. RFB handshake (transparent relay, Go does not parse)
   AMT ──"RFB 003.008\n"──► Go ──► Browser
   Browser ──"RFB 003.008\n"──► Go ──► AMT
   AMT ──security types──► Go ──► Browser
   Browser ──select None──► Go ──► AMT
   ... (SecurityResult, ClientInit, ServerInit)
   Browser ──SetEncodings [Raw, KvmDataChannel, DesktopSize]──► Go ──► AMT
   Browser ──FramebufferUpdateRequest (full)──► Go ──► AMT

6. Live screen (data plane loop)
   AMT ──FramebufferUpdate (320 tiles, RGB565)──► Go ──► Browser ──► canvas
   Browser ──FramebufferUpdateRequest (incremental)──► Go ──► AMT
   [repeats for every screen change]

7. Keyboard / mouse input
   User moves mouse over canvas → browser encodes RFB PointerEvent (6 bytes)
   Browser ──binary WS frame──► Go ──encodeUTF8Binary──► MPS ──► AMT
   AMT receives pointer event, moves remote cursor
```

---

## Running the Stack

**Terminal 1 — Go relay server**
```bash
cd kvm-poc/server
go build -o kvm_server kvm_server.go
./kvm_server
# Listening on :8080
```

**Terminal 2 — Angular viewer**
```bash
cd kvm-poc/kvm-angular-app
npm install
npm start
# http://localhost:4200
```

Open **http://localhost:4200** in Chrome or Edge.

---

## Troubleshooting

| Symptom | Where to look | Cause |
|---|---|---|
| Connect KVM button stays grey | Browser console — check consent POST response | Consent code rejected or not submitted |
| `POST /api/connect` returns 401 | Go server logs | JWT token expired — re-issue from orchestration stack |
| `POST /api/connect` returns 500 | Go server logs | MPS unreachable or redirect token request failed |
| WebSocket `/ws` closes with code 1006 | Browser Network → WS → Frames | MPS WebSocket disconnected — check Go server logs |
| Canvas blank, no tiles | Browser console — `[RFB]` log lines | RFB handshake stalled — check SetEncodings was sent |
| Keyboard not working | Browser console | Canvas lost focus — click the canvas once |
| Screen updates stop | Go server logs | `[WARN] MPS websocket closed` — session timed out, auto-reconnect will retry |
