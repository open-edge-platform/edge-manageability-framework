# Keycloak Development Quick Reference

## Quick Start: Local Development

### Prerequisites
```bash
# Ensure cluster is running
kubectl cluster-info

# Verify Keycloak is deployed
kubectl get pods -n keycloak-system | grep keycloak
```

### Run Tenant Setup Locally

```bash
# Terminal 1: Start port-forward
kubectl port-forward -n keycloak-system svc/platform-keycloak 8080:8080

# Terminal 2: Run setup
cd /home/seu/workspace/edge-manageability-framework
timeout 15 mage tenantUtils:createDefaultMtSetup

# Expected output:
# [Keycloak] Attempting to detect Keycloak URL...
# [KEYCLOAK] Logging in to: http://localhost:8080
# [KEYCLOAK] Login successful
# Creating default organization...
```

## Understanding the URL Detection

### When Each URL is Used

| URL | Environment | Status | Requirements |
|-----|-------------|--------|--------------|
| `https://keycloak.orch-10-139-218-125.pid.infra-host.com:443` | CI/CD in AWS network | ✅ Works | Inside AWS network |
| `http://platform-keycloak.keycloak-system.svc.cluster.local:8080` | Pod in cluster | ✅ Works | Running as pod |
| `http://localhost:8080` | Local machine | ✅ Works | Port-forward running |
| `KEYCLOAK_URL` env var | Any | ✅ Works | Set environment variable |

### URL Detection Flow

```
1. Check KEYCLOAK_URL env var
   ↓ (if not set)
2. Try external domain (10.139.218.125)
   ↓ (times out from local)
3. Try internal cluster DNS
   ↓ (DNS fails from local)
4. Use localhost:8080 (requires port-forward)
   ↓ ✅ SUCCESS
```

## Troubleshooting

### Issue: Timeout Connecting to Keycloak

```bash
# Check 1: Port-forward running?
ps aux | grep "port-forward"
# Expected: kubectl port-forward ... 8080:8080

# Check 2: Start port-forward if missing
kubectl port-forward -n keycloak-system svc/platform-keycloak 8080:8080

# Check 3: Test connectivity
curl -v http://localhost:8080/realms/master
# Expected: HTTP 200 or 401
```

### Issue: "Context Deadline Exceeded"

This is expected and means:
- External domain (`10.139.218.125`) is being tested
- Direct connection to AWS cloud IP times out from your local network
- Fallback will try next option (internal DNS)
- Eventually uses localhost:8080 if port-forward is running

**No action needed** - it's working as designed.

### Issue: "HTTP 504"

This is expected and means:
- Internal cluster DNS is being tested
- Local machine can't resolve cluster-internal domain names
- Fallback will try next option (localhost:8080)

**No action needed** - it's working as designed.

## HTTP Client Configuration Details

### The RemoveProxy() Call

**Location**: `mage/tenant_utils.go`, line 491

**Why it's there**: Corporate proxy environment has conflicting rules
- `no_proxy=10.*` → Bypasses proxy
- But proxy policy blocks 10.x as "internal"
- RemoveProxy() avoids this conflict by using direct connections

**Do not remove this line** - it's critical for reliability.

```go
restyClient := client.RestyClient()
restyClient.SetTimeout(60 * time.Second)
restyClient.RemoveProxy()  // ← CRITICAL, don't remove
client.SetRestyClient(restyClient)
```

### Timeout: Why 60 Seconds?

- Keycloak operations (user creation, group assignments) take time
- Multiple sequential operations compound delays
- 60 seconds is generous but not excessive
- URL detection uses separate 2-second timeout for quick fallback

## Common Mage Commands

```bash
# Create default setup (org, project, users)
mage tenantUtils:createDefaultMtSetup

# Create specific org
mage tenantUtils:createOrg <org-name>

# Create specific project in org
mage tenantUtils:createProjectInOrg <org-name> <project-name>

# Create project admin user
mage tenantUtils:createProjectAdminInOrg <org-name> <admin-username>

# Create edge infrastructure users
mage tenantUtils:createEdgeInfraUsers <org-name> <project-name> <user-prefix>

# List org details
mage tenantUtils:getOrg <org-name>

# Print default orchestrator password
mage tenantUtils:orchPassword
```

## Network Configuration

### Current Setup

- **Keycloak**: Running in cluster as Kubernetes Deployment
- **Service**: `platform-keycloak` in `keycloak-system` namespace, port 8080
- **Ingress**: Traefik `IngressRoute` in `orch-gateway` namespace
- **Domain**: `keycloak.orch-10-139-218-125.pid.infra-host.com` (AWS cloud IP)
- **For local access**: Use port-forward to localhost:8080

### Why Port-Forward is Best for Local Dev

✅ **Advantages**:
- No network routing issues
- No proxy policy conflicts  
- No DNS resolution problems
- Simple, standard Kubernetes practice
- Works reliably with corporate proxies

❌ **Why not external domain from local**:
- 10.139.218.125 is AWS cloud IP
- Local machine is outside AWS network
- no_proxy rules cause proxy bypass
- Cloud IP unreachable from local network

❌ **Why not internal DNS from local**:
- `svc.cluster.local` names only resolve inside cluster
- Requires CoreDNS access (cluster-internal)
- Local machine has no cluster DNS server

## Environment Variables

### KEYCLOAK_URL

Override automatic URL detection:

```bash
# Use specific URL
export KEYCLOAK_URL="http://localhost:8080"
mage tenantUtils:createDefaultMtSetup

# Or for CI/CD
export KEYCLOAK_URL="http://platform-keycloak.keycloak-system.svc.cluster.local:8080"
mage tenantUtils:createDefaultMtSetup
```

### ORCH_DEFAULT_PASSWORD

Default password for created users (must meet Keycloak policy):
- Minimum 14 characters
- At least 1 digit
- At least 1 special character (!@#$%^&*()_+-=[]{}|;:,.<>?)
- At least 1 uppercase letter
- At least 1 lowercase letter

```bash
export ORCH_DEFAULT_PASSWORD="MySecurePass123!"
mage tenantUtils:createDefaultMtSetup
```

## Related Documentation

See these files for more details:

1. **KEYCLOAK_NETWORK_ARCHITECTURE.md**
   - Complete network topology explanation
   - Why each URL path works/fails
   - Debugging tips and manual tests
   - Usage guidelines for different scenarios

2. **KEYCLOAK_HTTP_CLIENT_CONFIG.md**
   - HTTP client configuration details
   - Why RemoveProxy() is necessary
   - Best practices for future modifications
   - Integration with fallback detection

3. **KEYCLOAK_INVESTIGATION_SUMMARY.md**
   - Investigation findings
   - Code changes summary
   - Technical deep dive

## Key Code Locations

| File | Function | Purpose |
|------|----------|---------|
| `mage/dev_utils.go:487-510` | `getKeycloakBaseURL()` | Smart URL detection |
| `mage/dev_utils.go:512-531` | `canReachKeycloak()` | Connectivity testing |
| `mage/tenant_utils.go:479-503` | `KeycloakLogin()` | Client config & auth |
| `mage/tenant_utils.go:37-80` | `CreateDefaultMtSetup()` | Main setup flow |

## Quick Diagnostics

```bash
# Check port-forward status
lsof -i :8080

# Test Keycloak from localhost
curl -v http://localhost:8080/realms/master

# Test Keycloak from external domain (will timeout)
timeout 3 curl -v https://keycloak.orch-10-139-218-125.pid.infra-host.com:443/realms/master

# Check proxy settings
echo "Proxy: $http_proxy / $https_proxy"
echo "No-proxy: $no_proxy"

# Get Keycloak admin password
kubectl get secret platform-keycloak -n keycloak-system -o jsonpath='{.data.admin-password}' | base64 -d

# View pod logs
kubectl logs -n keycloak-system -l app.kubernetes.io/name=keycloak -f
```

## Support

If issues occur:

1. **First**: Check port-forward is running
2. **Then**: Review logs in troubleshooting section above
3. **Else**: See KEYCLOAK_NETWORK_ARCHITECTURE.md for detailed debugging
4. **Finally**: Check related documentation files for context

Remember: URL detection logs show which option is being tried - this helps diagnose network issues.
