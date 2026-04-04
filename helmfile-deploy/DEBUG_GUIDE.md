# EMF Helmfile-Deploy Debug Guide (onprem-eim)

## Quick Reference

```bash
# Deploy command
EMF_HELMFILE_ENV=onprem-eim ./helmfile-deploy.sh install

# Sync a single release
EMF_HELMFILE_ENV=onprem-eim helmfile -e onprem-eim -l name=<release> sync

# Check all pod status
kubectl get pods -A | grep -v Running | grep -v Completed
```

---

## Issue 1: Traefik Not Routing / No LoadBalancer IP

**Symptom:** Services like `web-ui.cluster.onprem`, `api.cluster.onprem` unreachable.

**Root Cause:** Traefik chart v39 has a breaking change — middlewares must be nested under `ports.websecure.http.middlewares` (not top-level). Also `service.type: LoadBalancer` was missing.

**Fix:** In `values/traefik.yaml.gotmpl`:
```yaml
service:
  type: LoadBalancer

ports:
  websecure:
    http:
      middlewares:
        - orch-gateway-cors-header@kubernetescrd
```

**Verify:**
```bash
kubectl get svc -n orch-gateway traefik -o wide
# Should show EXTERNAL-IP from MetalLB (e.g., 192.168.99.30)
```

---

## Issue 2: api.cluster.onprem Returns 404

**Symptom:** `curl https://api.cluster.onprem/v1/orgs` returns 404.

**Root Cause:** `nexus-api-gw` IngressRoute missing `host.grpc.name` and `tlsOption`.

**Fix:** In `values/nexus-api-gw.yaml.gotmpl`:
```yaml
host:
  grpc:
    name: "api.{{ .Values.clusterDomain }}"
tlsOption: "gateway-tls"
```

**Verify:**
```bash
kubectl get ingressroute -n orch-gateway nexus-api-gw-http -o yaml | grep -A2 match
```

---

## Issue 3: Full_server.crt 404 (Tinkerbell/DKAM)

**Symptom:** `wget https://tinkerbell-haproxy.cluster.onprem/tink-stack/keys/Full_server.crt` fails with 404.

**Root Cause:** DKAM's `build_sign_ipxe.sh` compiles iPXE from source and only copies the cert to the shared PVC after build completes. The default CPU limit (100m) throttles the build so it never finishes in time.

**Fix:** In `values/infra-onboarding.yaml.gotmpl`:
```yaml
dkam:
  resources: null
```

Or use `values/resource-overrides.yaml` with generous limits for all charts.

**Verify:**
```bash
# Check DKAM pod status and logs
kubectl get pods -n orch-infra | grep dkam
kubectl logs -n orch-infra -l app=dkam --tail=50

# Test cert download
curl -sk https://tinkerbell-haproxy.cluster.onprem/tink-stack/keys/Full_server.crt | head -1
# Should show: -----BEGIN CERTIFICATE-----
```

---

## Issue 4: Web-UI Returns Wrong Content (MIME Type Error)

**Symptom:** Browser console shows `text/html` MIME type error for JavaScript files. Page loads but shows blank or broken UI.

**Root Cause:** Both `web-ui-root` and `web-ui-infra` had IngressRoutes at `PathPrefix(/)` with priority 10. Traefik randomly picks one, serving web-ui-infra's HTML fallback instead of root's nginx proxy.

**Fix:** In `values/web-ui-infra.yaml.gotmpl`:
```yaml
service:
  traefik:
    enabled: false
```

The root chart handles all routing and proxies `/mfe/infrastructure/*` to web-ui-infra via cluster DNS.

**Verify:**
```bash
# Only web-ui-root should have PathPrefix(/)
kubectl get ingressroute -A -o custom-columns='NAME:.metadata.name,MATCH:.spec.routes[*].match' | grep PathPrefix
```

---

## Issue 5: API Returns 403 Forbidden (OPA RBAC)

**Symptom:** `GET /v1/projects/<project>/compute/hosts/summary` returns 403 for all users.

**Root Cause (multi-part):**

### 5a. Missing OPA Roles

The OPA rego policy (`apiv2/rego/authz.rego`) expects these specific short role names:
- `{projectUUID}_im-r` (read)
- `{projectUUID}_im-rw` (read-write)
- `{projectUUID}_en-agent-rw` (edge node agent)
- `{projectUUID}_en-ob` (edge onboarding)

The keycloak-tenant-controller creates these roles but does NOT assign them to groups. Users only get long-named roles like `{projectUUID}_infra-manager-core-read-role` which OPA doesn't check.

**Fix:** Manually assign via Keycloak Admin API:
```bash
# Port-forward to Keycloak
kubectl port-forward -n orch-platform svc/platform-keycloak 18080:8080 &

# Get admin token
KC_PASS=$(kubectl get secret platform-keycloak -n orch-platform -o jsonpath='{.data.admin-password}' | base64 -d)
TOKEN=$(curl -s -X POST 'http://localhost:18080/realms/master/protocol/openid-connect/token' \
  -d 'client_id=admin-cli' -d 'grant_type=password' -d 'username=admin' \
  --data-urlencode "password=$KC_PASS" | jq -r .access_token)

# Get project UUID
PROJECT_UUID="<your-project-uuid>"

# Get role IDs
IMR=$(curl -s "http://localhost:18080/admin/realms/master/roles/${PROJECT_UUID}_im-r" -H "Authorization: Bearer $TOKEN" | jq -r '.id')
IMRW=$(curl -s "http://localhost:18080/admin/realms/master/roles/${PROJECT_UUID}_im-rw" -H "Authorization: Bearer $TOKEN" | jq -r '.id')
ENRW=$(curl -s "http://localhost:18080/admin/realms/master/roles/${PROJECT_UUID}_en-agent-rw" -H "Authorization: Bearer $TOKEN" | jq -r '.id')
ENOB=$(curl -s "http://localhost:18080/admin/realms/master/roles/${PROJECT_UUID}_en-ob" -H "Authorization: Bearer $TOKEN" | jq -r '.id')

# Assign to user
USER_ID="<user-uuid>"
curl -s -X POST "http://localhost:18080/admin/realms/master/users/$USER_ID/role-mappings/realm" \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d "[{\"id\":\"$IMR\",\"name\":\"${PROJECT_UUID}_im-r\"},{\"id\":\"$IMRW\",\"name\":\"${PROJECT_UUID}_im-rw\"},{\"id\":\"$ENRW\",\"name\":\"${PROJECT_UUID}_en-agent-rw\"},{\"id\":\"$ENOB\",\"name\":\"${PROJECT_UUID}_en-ob\"}]"

# Also assign to Edge-Manager-Group for all members
GROUP_ID=$(curl -s "http://localhost:18080/admin/realms/master/groups" -H "Authorization: Bearer $TOKEN" | jq -r ".[] | select(.name == \"${PROJECT_UUID}_Edge-Manager-Group\") | .id")
curl -s -X POST "http://localhost:18080/admin/realms/master/groups/$GROUP_ID/role-mappings/realm" \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d "[{\"id\":\"$IMR\",\"name\":\"${PROJECT_UUID}_im-r\"},{\"id\":\"$IMRW\",\"name\":\"${PROJECT_UUID}_im-rw\"},{\"id\":\"$ENRW\",\"name\":\"${PROJECT_UUID}_en-agent-rw\"},{\"id\":\"$ENOB\",\"name\":\"${PROJECT_UUID}_en-ob\"}]"
```

### 5b. Missing `groups` Scope on webui-client

Keycloak's `webui-client` had `groups` as an optional scope, so JWT tokens never included group membership.

**Fix:**
```bash
# Get scope IDs
GROUPS_SCOPE=$(curl -s "http://localhost:18080/admin/realms/master/client-scopes" -H "Authorization: Bearer $TOKEN" | jq -r '.[] | select(.name=="groups") | .id')
WEBUI_CLIENT=$(curl -s "http://localhost:18080/admin/realms/master/clients?clientId=webui-client" -H "Authorization: Bearer $TOKEN" | jq -r '.[0].id')

# Remove from optional, add to default
curl -s -X DELETE "http://localhost:18080/admin/realms/master/clients/$WEBUI_CLIENT/optional-client-scopes/$GROUPS_SCOPE" -H "Authorization: Bearer $TOKEN"
curl -s -X PUT "http://localhost:18080/admin/realms/master/clients/$WEBUI_CLIENT/default-client-scopes/$GROUPS_SCOPE" -H "Authorization: Bearer $TOKEN"
```

**Verify:**
```bash
# Check apiv2 logs after a request
kubectl logs -n orch-infra deploy/apiv2 -c apiv2-proxy --tail=5
# Should NOT show "GET is blocked by OPA"
```

---

## Issue 6: Template Fails Without Proxy Vars

**Symptom:** `helmfile sync` fails with `map has no entry for key "proxy"`.

**Root Cause:** Templates reference `.Values.proxy.*` but `EMF_HTTP_PROXY` is not set in on-prem.

**Fix:** Created `values/proxy-defaults.yaml` with empty defaults, added to environment values before the env file.

---

## Issue 7: eimScenario: noobb

**Symptom:** May affect OPA policy behavior.

**Root Cause:** `eimScenario: noobb` was set in helmfile-deploy but NOT in the working helm-deploy or chart defaults. Chart default is `fulleim`.

**Fix:** Removed `eimScenario: noobb` from `values/infra-core.yaml.gotmpl`.

---

## Debugging Tips

### Inspect JWT Token Claims
```bash
# Keycloak 26: admin-cli uses lightweight tokens (empty claims)!
# Use webui-client instead for testing:
TOKEN=$(curl -s -X POST 'https://keycloak.cluster.onprem/realms/master/protocol/openid-connect/token' \
  -d 'client_id=webui-client' -d 'grant_type=password' \
  -d 'username=<user>' --data-urlencode 'password=<pass>' | jq -r .access_token)

# Decode JWT payload
echo "$TOKEN" | cut -d'.' -f2 | (cat; echo '==') | base64 -d 2>/dev/null | jq .
```

### Check IngressRoute Priorities
```bash
kubectl get ingressroute -A -o custom-columns='NS:.metadata.namespace,NAME:.metadata.name,PRIORITY:.spec.routes[*].priority,MATCH:.spec.routes[*].match'
```

### Debug apiv2 OPA Decisions
```bash
# Temporarily set debug log level
kubectl patch deploy apiv2 -n orch-infra --type=json \
  -p='[{"op":"replace","path":"/spec/template/spec/containers/0/args/6","value":"-globalLogLevel=debug"}]'

# Watch logs
kubectl logs -n orch-infra deploy/apiv2 -c apiv2-proxy -f | grep -E "debug|error"

# Reset via helmfile sync after debugging
EMF_HELMFILE_ENV=onprem-eim helmfile -e onprem-eim -l name=infra-core sync
```

### Check Keycloak Client Scopes
```bash
# Verify webui-client has groups as default scope
curl -s "http://localhost:18080/admin/realms/master/clients/<client-uuid>/default-client-scopes" \
  -H "Authorization: Bearer $TOKEN" | jq -r '.[].name'
# Expected: basic, email, groups, openid, profile, roles
```

### Verify OPA Expected Roles Exist
```bash
# The OPA rego expects these role patterns:
# {projectUUID}_im-r, {projectUUID}_im-rw, {projectUUID}_en-agent-rw, {projectUUID}_en-ob
curl -s "http://localhost:18080/admin/realms/master/roles/${PROJECT_UUID}_im-r" \
  -H "Authorization: Bearer $TOKEN" | jq .name
```
