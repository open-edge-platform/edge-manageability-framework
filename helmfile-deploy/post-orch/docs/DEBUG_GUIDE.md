# EMF On-Prem Debug Guide

**Applies to:** `onprem-eim` | `onprem-vpro` | `onprem-eim-co`

## Quick Reference

```bash
# Check all pod status
kubectl get pods -A | grep -v Running | grep -v Completed

# Sync a single release (replace <profile> with onprem-eim, onprem-vpro, or onprem-eim-co)
EMF_HELMFILE_ENV=<profile> helmfile -e <profile> -l name=<release> sync
```

---

# Common Issues (All Profiles)

These issues apply to **onprem-eim**, **onprem-vpro**, and **onprem-eim-co**.

## 1. Traefik Not Routing / No LoadBalancer IP

**Symptom:** Services like `web-ui.cluster.onprem`, `api.cluster.onprem` are unreachable.

**Verify:**
```bash
kubectl get svc -n orch-gateway traefik -o wide
# Should show EXTERNAL-IP from MetalLB
```

If no external IP is assigned, ensure `service.type: LoadBalancer` is set in the Traefik values and MetalLB is configured correctly.

---

## 2. API Returns 404

**Symptom:** `curl https://api.cluster.onprem/v1/orgs` returns 404.

**Verify:**
```bash
kubectl get ingressroute -n orch-gateway nexus-api-gw-http -o yaml | grep -A2 match
```

Ensure `nexus-api-gw` IngressRoute has the correct `host.grpc.name` set to `api.<clusterDomain>`.

---

## 3. Web-UI Shows Blank or Broken Page

**Symptom:** Browser console shows `text/html` MIME type error for JavaScript files.

**Cause:** Conflicting IngressRoutes — both `web-ui-root` and `web-ui-infra` have `PathPrefix(/)` at the same priority.

**Verify:**
```bash
# Only web-ui-root should have PathPrefix(/)
kubectl get ingressroute -A -o custom-columns='NAME:.metadata.name,MATCH:.spec.routes[*].match' | grep PathPrefix
```

Disable the standalone Traefik IngressRoute in `web-ui-infra` — the root chart handles all routing.

---

## 4. API Returns 403 Forbidden (OPA RBAC)

**Symptom:** API calls like `GET /v1/projects/<project>/compute/hosts/summary` return 403.

**Cause:** Users missing the short OPA role names (`{projectUUID}_im-r`, `_im-rw`, `_en-agent-rw`, `_en-ob`) or `groups` scope not set as default on `webui-client`.

> **Note:** The helmfile values for `keycloak-tenant-controller` now handle OPA role assignment and client scope configuration automatically. If you still see 403 after a fresh deploy, re-sync the release:
> ```bash
> EMF_HELMFILE_ENV=<profile> helmfile -e <profile> -l name=keycloak-tenant-controller sync
> ```

**Verify:**
```bash
kubectl logs -n orch-infra deploy/apiv2 -c apiv2-proxy --tail=5
# Should NOT show "GET is blocked by OPA"
```

---

# onprem-eim / onprem-eim-co Issues

## 5. Tinkerbell Certificate 404 (DKAM)

**Profiles:** `onprem-eim`, `onprem-eim-co`

**Symptom:** `wget https://tinkerbell-haproxy.cluster.onprem/tink-stack/keys/Full_server.crt` fails with 404.

**Cause:** DKAM's iPXE build requires significant CPU. If resource limits are too low, the build never completes and the certificate is not generated.

**Verify:**
```bash
kubectl get pods -n orch-infra | grep dkam
kubectl logs -n orch-infra -l app=dkam --tail=50

# Test cert availability
curl -sk https://tinkerbell-haproxy.cluster.onprem/tink-stack/keys/Full_server.crt | head -1
# Expected: -----BEGIN CERTIFICATE-----
```

If the DKAM pod is stuck or OOMKilled, increase or remove resource limits for `dkam`.

---

# onprem-vpro Issues

These issues are specific to the **onprem-vpro** profile (Intel AMT / vPro managed edge nodes).

## 6. Platform-Manageability-Agent Unhealthy (404 Errors)

**Symptom:** Agent reports 404 errors when posting AMT status to orchestrator.

**Cause:** IngressRoutes have empty `Host('')` or missing `_m` role in Keycloak M2M group.

> **Note:** The helmfile values for `keycloak-tenant-controller` and `infra-external` now handle the M2M role assignment and IngressRoute hostnames automatically. If you still see 404 after a fresh deploy, re-sync:
> ```bash
> EMF_HELMFILE_ENV=onprem-vpro helmfile -e onprem-vpro -l name=keycloak-tenant-controller sync
> EMF_HELMFILE_ENV=onprem-vpro helmfile -e onprem-vpro -l name=infra-external sync
> ```

**Verify:**
```bash
kubectl get ingressroute -A -o custom-columns='NAME:.metadata.name,MATCH:.spec.routes[*].match' | grep -E "device-manager|mps|rps"
```

Expected hostnames: `device-manager-node.<domain>`, `mps.<domain>`, `rps.<domain>`.

---

## 7. WebSocket Bad Handshake During AMT Activation

**Symptom:** `rpc activate` fails with `websocket: bad handshake` (401).

**Cause:** RPS websocket and REST API share the same hostname. The JWT validation middleware on the REST route also intercepts websocket upgrade requests.

**Fix:** Ensure RPS and MPS have separate hostnames for different endpoints:

| Service | Endpoint | Hostname |
|---|---|---|
| RPS | WebSocket (port 8080) | `rps.<domain>` (no JWT middleware) |
| RPS | REST API (port 8081) | `rps-wss.<domain>` (with JWT middleware) |
| MPS | CIRA (port 4433) | `mps.<domain>` |
| MPS | REST API (port 3000) | `mps-wss.<domain>` |

---

## 8. RAS MPS Hostname Empty After AMT Activation

**Symptom:** After edge node agent reinstall, `rpc amtinfo` shows:
```
RAS MPS Hostname        :          (empty)
RAS Remote Status       : not connected
```

**Cause:** A stuck `rpc` process may hold `/dev/mei0`, preventing LMS from connecting to HECI.

> **Note:** The updated `uninstall_new.sh` already preserves `lms.service` and runs `ensure_lms_healthy()` as a safety net. The steps below are only needed if the issue persists after using the updated uninstall script.

### Recovery Steps

**Step 1: Kill any stuck rpc process**
```bash
sudo fuser -v /dev/mei0
# If rpc is listed:
sudo kill -9 <rpc_pid>
```

**Step 2: Verify LMS is listening**
```bash
ss -tlnp | grep 16992
# Expected: LISTEN 0 5 0.0.0.0:16992
```
If not listening, check `journalctl -u lms.service` for HECI errors and repeat Step 1.

**Step 3: Deactivate AMT**
```bash
sudo /usr/bin/rpc deactivate -local
```

**Step 4: Restart dm-manager (on orchestrator)**
```bash
kubectl -n orch-infra rollout restart deploy/dm-manager
kubectl -n orch-infra rollout status deploy/dm-manager --timeout=60s
```

**Step 5: Restart pm-agent**
```bash
sudo systemctl restart platform-manageability-agent
```

**Step 6: Verify (~2 minutes)**
```bash
sudo /usr/bin/rpc amtinfo | grep -i "ras\|control"
```
Expected:
```
Control Mode            : activated in client control mode
RAS Remote Status       : connected
RAS Trigger             : periodic
RAS MPS Hostname        : mps.<domain>
```

---

# Debugging Tips (All Profiles)

### Inspect JWT Token Claims
```bash
TOKEN=$(curl -s -X POST 'https://keycloak.cluster.onprem/realms/master/protocol/openid-connect/token' \
  -d 'client_id=webui-client' -d 'grant_type=password' \
  -d 'username=<user>' --data-urlencode 'password=<pass>' | jq -r .access_token)

# Decode JWT payload
echo "$TOKEN" | cut -d'.' -f2 | (cat; echo '==') | base64 -d 2>/dev/null | jq .
```

> **Note:** Keycloak 26 `admin-cli` uses lightweight tokens with empty claims. Use `webui-client` for testing.

### Check IngressRoute Priorities
```bash
kubectl get ingressroute -A -o custom-columns='NS:.metadata.namespace,NAME:.metadata.name,PRIORITY:.spec.routes[*].priority,MATCH:.spec.routes[*].match'
```

### Check apiv2 OPA Decisions
```bash
kubectl logs -n orch-infra deploy/apiv2 -c apiv2-proxy --tail=20 | grep -E "blocked|error"
```
