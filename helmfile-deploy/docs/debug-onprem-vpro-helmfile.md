# On-Prem vPro Helmfile Deployment - Debug Report

**Date:** April 2026  
**Environment:** On-prem, k3s/rke2, Traefik ingress, domain `cluster.onprem`  
**Edge Node:** 192.168.170.29 (hostname: suniltest1, Ubuntu 24.04.3, kernel 6.17.0-20-generic)  
**AMT:** Version 18.0.10, Build 2351, SKU 16392 (AMT Pro Corporate), CCM mode  

---

## Issue 1: Platform-Manageability-Agent Unhealthy (404 Errors)

**Symptom:** Agent reported 404 errors when posting AMT status to orchestrator.

**Root Cause:**
1. Missing `_m` role in Keycloak M2M group — the TenantInterceptor rejected requests from the agent because it lacked the required role.
2. Empty `Host('')` in Traefik IngressRoutes — routes had no hostname, so requests couldn't reach the backend services.

**Fix (Runtime):**
- Added `<org-id>_<project-id>_m` role to Keycloak `Edge-Node-M2M-Service-Account` group via Keycloak Admin API.
- Patched IngressRoutes with correct hostnames (`device-manager-node.cluster.onprem`, `mps.cluster.onprem`, etc.).

**Fix (Helmfile):**
- **File:** `helmfile-deploy/values/keycloak-tenant-controller.yaml.gotmpl`
- Added `"<org-id>_<project-id>_m"` to the M2M group role mappings.

---

## Issue 2: WebSocket Bad Handshake During AMT Activation

**Symptom:** `rpc activate` failed with `websocket: bad handshake`. The RPS websocket endpoint returned 401 because Traefik's `validate-jwt` middleware intercepted the websocket upgrade request.

**Root Cause:** Both the REST API (port 8081) and websocket (port 8080) on RPS shared the same hostname. The IngressRoute for the REST API had JWT validation middleware applied, and since both routes matched the same host, websocket connections were also hitting the JWT middleware.

**Fix (Helmfile):**
- **File:** `helmfile-deploy/values/infra-external.yaml.gotmpl`
- Separated RPS hostnames:
  - `rps.host.grpc.name=rps.<domain>` → websocket endpoint (port 8080, no JWT middleware)
  - `rps.host.webport.name=rps-wss.<domain>` → REST API endpoint (port 8081, with JWT middleware)
- Similarly for MPS:
  - `mps.host.cira.name=mps.<domain>` → CIRA endpoint (port 4433)
  - `mps.host.webport.name=mps-wss.<domain>` → REST API endpoint (port 3000)
- Added `mps.commonName=mps.<domain>` for TLS certificate SAN.

---

## Issue 3: Helmfile vs ArgoCD Config Misalignment

**Symptom:** Working ArgoCD deployment had different hostname assignments than the helmfile deployment.

**Root Cause:** The helmfile values template had not been updated to match the ArgoCD reference configuration in `argocd/applications/custom/infra-external.tpl`.

**Fix (Helmfile):**
- **File:** `helmfile-deploy/values/infra-external.yaml.gotmpl`
- Aligned all hostname, TLS, and IngressRoute config with ArgoCD:

| Parameter | Before (Helmfile) | After (Aligned with ArgoCD) |
|---|---|---|
| `rps.host.grpc.name` | empty/wrong | `rps.<domain>` |
| `rps.host.webport.name` | empty/wrong | `rps-wss.<domain>` |
| `mps.commonName` | missing | `mps.<domain>` |
| `mps.host.cira.name` | missing | `mps.<domain>` |
| `mps.host.webport.name` | empty/wrong | `mps-wss.<domain>` |
| `dm-manager.host.grpc.name` | empty/wrong | `device-manager-node.<domain>` |

---

## Issue 4: Power Reset Appears to Not Work

**Symptom:** User reported power reset not working on edge node.

**Root Cause:** Power reset was actually working correctly. MPS returned HTTP 200 with `SUCCESS` status, and AMT confirmed the reboot. The confusion was caused by:
1. Device state getting stuck at `RESET_REPEAT` — this is expected behavior while the system waits for the node to come back up.
2. "Dropped events" log messages — these were from the tenant controller client (event bus consumer), not the DeviceReconciler that handles the actual power action.

**Resolution:** No fix needed — power reset was functioning correctly. The state tracking is working as designed.

---

## Issue 5: Uninstall Script Bugs

**Symptom:** Original uninstall script had multiple issues that left the edge node in a broken state.

**Root Cause:** Script bugs including missing `main()` call, incorrect service names, and incomplete cleanup.

**Fix:** Created `uninstall_new.sh` with:
- Proper `set -Eeuo pipefail` error handling
- Correct service list
- Manifest-based cleanup with fallback
- AppArmor profile unloading before file removal
- User/group cleanup
- Sudoers file removal

---

## Issue 6: RAS MPS Hostname Missing After AMT Activation

**Symptom:** After AMT activation, `rpc amtinfo` showed:
```
RAS MPS Hostname :          (empty)
RAS Remote Status: not connected
```
CIRA was not configured, so the edge node couldn't establish a remote management tunnel to MPS.

**Root Cause Chain:**
1. The uninstall script masked `lms.service` (Intel Local Manageability Service).
2. On reinstall, `pm-agent` tried to start LMS via `systemctl` but failed because:
   - LMS was masked (systemctl start returns error on masked units)
   - The AppArmor profile for pm-agent (`/etc/apparmor.d/opt.edge-node.bin.pm-agent`) uses `ix` (inherit execute) for `/usr/bin/systemctl`, meaning systemctl runs under pm-agent's confined profile, which lacks permissions for systemd's private socket operations.
3. Without LMS running, the CIRA configuration step during AMT activation failed:
   ```
   CIRA: Failed to add mps
   CIRA: Failed to put AMT_RemoteAccessPolicyAppliesToMPS
   ```
4. AMT was activated in CCM mode but without CIRA/MPS hostname configured.

**Fix (Manual Recovery):**
```bash
# 1. Unmask and start LMS
sudo systemctl unmask lms.service
sudo systemctl enable lms.service
sudo systemctl start lms.service

# 2. Deactivate AMT to clear stale state
sudo /usr/bin/rpc deactivate -p '<amt-password>'

# 3. Restart dm-manager to clear "activation already in progress" lock
kubectl -n orch-infra rollout restart deploy/dm-manager

# 4. Restart pm-agent for clean re-activation
sudo systemctl restart platform-manageability-agent
```

**Fix (Permanent — uninstall_new.sh):**
- Removed `lms.service` from the `SERVICES` array so it won't be stopped/disabled/masked.
- Removed `lms` from `apt-get remove` and `apt-get purge` commands.

LMS is a platform-level Intel AMT service, not an edge-node agent — it must remain installed and unmasked for CIRA to work.

**Verification:**
```
$ rpc amtinfo
...
RAS MPS Hostname :       mps.cluster.onprem
RAS Remote Status:       connected
RAS Trigger:             periodic
```

---

## Summary of Helmfile Changes

### `helmfile-deploy/values/keycloak-tenant-controller.yaml.gotmpl`
- Added `_m` role to `Edge-Node-M2M-Service-Account` group

### `helmfile-deploy/values/infra-external.yaml.gotmpl`
- `mps.commonName` = `mps.<domain>`
- `mps.host.cira.name` = `mps.<domain>` (port 4433, CIRA)
- `mps.host.webport.name` = `mps-wss.<domain>` (port 3000, REST API)
- `rps.host.grpc.name` = `rps.<domain>` (port 8080, websocket, no JWT)
- `rps.host.webport.name` = `rps-wss.<domain>` (port 8081, REST API, with JWT)
- `dm-manager.host.grpc.name` = `device-manager-node.<domain>`

### Edge Node: `uninstall_new.sh`
- Removed `lms.service` from SERVICES array
- Removed `lms` from apt-get remove/purge commands

---

## Known Remaining Issue (Non-Blocking)

**AppArmor profile blocks pm-agent systemctl operations:**  
The pm-agent AppArmor profile at `/etc/apparmor.d/opt.edge-node.bin.pm-agent` allows `/usr/bin/systemctl ix`, but the `ix` (inherit execute) means systemctl runs under pm-agent's confined profile, which lacks permissions for systemd D-Bus/socket operations. If LMS ever gets stopped or masked again, pm-agent cannot self-recover by restarting it.

**Recommendation:** Either update the AppArmor profile to use `Px` (discrete profile transition) or `Ux` (unconfined execute) for systemctl, or ensure the packaging/install process guarantees LMS is always running before pm-agent starts.
