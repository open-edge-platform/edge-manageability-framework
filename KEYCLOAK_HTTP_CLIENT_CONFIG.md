# Keycloak HTTP Client Configuration: Understanding RemoveProxy()

## Summary

The `KeycloakLogin()` function in `mage/tenant_utils.go` properly configures the HTTP client to work correctly with Keycloak by:

1. Creating a Resty client via GoCloak
2. Setting a 60-second timeout for token operations
3. **Calling `RemoveProxy()` to disable proxy routing**
4. Using this client for all Keycloak authentication

This configuration ensures reliable connectivity regardless of network environment.

## Code Location

**File**: `/mage/tenant_utils.go` lines 479-503

```go
func KeycloakLogin(ctx context.Context) (*gocloak.GoCloak, *gocloak.JWT, error) {
	keycloakURL := getKeycloakBaseURL()
	fmt.Printf("[KEYCLOAK] Logging in to: %s\n", keycloakURL)

	adminPass, err := GetKeycloakSecret()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get keycloak admin password: %w", err)
	}

	client := gocloak.NewClient(keycloakURL)

	// Configure HTTP client: extended timeout and disable proxy for reliability
	restyClient := client.RestyClient()
	restyClient.SetTimeout(60 * time.Second)
	restyClient.RemoveProxy() // Don't route through corporate proxy
	client.SetRestyClient(restyClient)

	jwtToken, err := client.LoginAdmin(ctx, adminUser, adminPass, KeycloakRealm)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to login to keycloak %s: %w", keycloakURL, err)
	}
	fmt.Printf("[KEYCLOAK] Login successful\n")

	return client, jwtToken, nil
}
```

## Why RemoveProxy() is Necessary

### The Problem

When environment proxy settings are configured:
```bash
http_proxy=http://proxy-dmz.intel.com:912
https_proxy=http://proxy-dmz.intel.com:912
no_proxy=10.*,10.0.0.0/8,pid.infra-host.com,...
```

The Resty HTTP client automatically uses these settings. However:

1. **For localhost:8080 (port-forward)**: 
   - ✅ Localhost bypasses proxy naturally (localhost is in implicit no_proxy)
   - But explicit proxy settings might interfere
   
2. **For internal cluster DNS**:
   - ❌ `platform-keycloak.keycloak-system.svc.cluster.local` doesn't bypass proxy
   - ❌ Proxy can't resolve cluster internal DNS names
   - ❌ Causes connection failures

3. **For external domain**:
   - Complex interaction between no_proxy rules and actual network
   - Unreliable from local machines

### The Solution: RemoveProxy()

By calling `restyClient.RemoveProxy()`, we tell the HTTP client to:
- Ignore all proxy environment variables
- Make direct connections to the target
- Avoid proxy policy filtering

This works because:

1. **localhost:8080** ✅
   - Already local, no network path needed
   - Direct connection is fastest

2. **Internal cluster DNS** ⚠️ (now fails at HTTP level, not proxy level)
   - Resty tries direct connection
   - Still won't resolve from host machine
   - But failure is clear (DNS resolution failure, not proxy error)

3. **External domain** ❌ (still won't work from local)
   - Direct connection to AWS cloud IP
   - But cloud IP is network-unreachable from local machine
   - Clear timeout instead of proxy policy block

### Network Path Comparison

```
WITHOUT RemoveProxy():
┌─────────────────────────────────────────────────────────────┐
│ Go HTTP Client (with proxy settings enabled)                │
├─────────────────────────────────────────────────────────────┤
│ 1. Check if target matches no_proxy patterns                │
│ 2. If not matched: Route through corporate proxy            │
│ 3. If matched: Try direct connection                        │
│ 4. Proxy may still intercept based on policy                │
└─────────────────────────────────────────────────────────────┘
     ↓
   Result: Unpredictable, depends on proxy policies

WITH RemoveProxy():
┌─────────────────────────────────────────────────────────────┐
│ Go HTTP Client (proxy disabled)                             │
├─────────────────────────────────────────────────────────────┤
│ 1. Ignore all proxy environment variables                   │
│ 2. Make direct connection to target                         │
│ 3. No proxy policy filtering                                │
│ 4. Fallback chain determines which URL works                │
└─────────────────────────────────────────────────────────────┘
     ↓
   Result: Predictable - either works or fails clearly
```

## Integration with Fallback Detection

The `RemoveProxy()` call works in conjunction with `getKeycloakBaseURL()`:

### Attempt 1: External Domain
```
URL: https://keycloak.orch-10-139-218-125.pid.infra-host.com:443
RemoveProxy: Enabled
Result: Tries direct connection to AWS IP
        ❌ Network unreachable from local (timeout)
        ✅ Works inside AWS network
Fallback: Try next URL
```

### Attempt 2: Internal Cluster DNS
```
URL: http://platform-keycloak.keycloak-system.svc.cluster.local:8080
RemoveProxy: Enabled
Result: Tries DNS resolution from local
        ❌ DNS not resolvable (cluster-internal domain)
        ✅ Works from inside cluster pod
Fallback: Try next URL
```

### Attempt 3: Localhost with Port-Forward ✅
```
URL: http://localhost:8080
RemoveProxy: Enabled
Result: Direct connection to localhost
        ✅ Works reliably (port-forward tunnels to cluster)
        ✅ No network path issues
        ✅ No proxy interference
Final: Use this URL
```

## Timeout Configuration

```go
restyClient.SetTimeout(60 * time.Second)
```

**Why 60 seconds?**

1. **Keycloak operations can take time**:
   - User creation
   - Group assignments
   - Role mappings
   - Multiple sequential operations

2. **Network latency factors**:
   - Port-forward tunnel setup
   - Kubernetes API communication
   - Internal service discovery (if using cluster DNS)

3. **Prevents premature failures**:
   - Short timeouts cause false negatives in fallback detection
   - Long timeout ensures operations complete
   - 60 seconds is reasonable balance

**Example from detection logic**:
```go
func canReachKeycloak(url string) bool {
	client := &http.Client{Timeout: 2 * time.Second}  // Fast detection
	...
}

func KeycloakLogin() {
	...
	restyClient.SetTimeout(60 * time.Second)  // Generous for actual operations
	...
}
```

Note: Detection uses 2-second timeout for quick fallback, but actual operations use 60 seconds.

## When This Configuration is Used

The configured Resty client is used in:

1. **`KeycloakLogin(ctx context.Context)`**
   - Line 496: `client.LoginAdmin(ctx, adminUser, adminPass, KeycloakRealm)`
   - All subsequent operations via this client

2. **Propagated to all tenant setup flows**:
   - `CreateDefaultMtSetup()` → calls `KeycloakLogin()`
   - `CreateProjectAdminInOrg()` → calls `KeycloakLogin()`
   - `CreateEdgeInfraUsers()` → calls `KeycloakLogin()`
   - `CreateClusterOrchUsers()` → calls `KeycloakLogin()`
   - User creation, group assignments, role mappings all use this client

## Debugging: How to Verify Configuration

### Check if proxy is interfering
```bash
# See current proxy settings
echo "http_proxy=$http_proxy"
echo "https_proxy=$https_proxy"
echo "no_proxy=$no_proxy"

# Verify RemoveProxy behavior
# (Can't directly test Resty RemoveProxy, but observe behavior)
```

### Verify Keycloak connectivity with current client config
```bash
# With port-forward running
kubectl port-forward -n keycloak-system svc/platform-keycloak 8080:8080 &

# Run tenant setup
timeout 15 mage tenantUtils:createDefaultMtSetup 2>&1 | head -50

# Expected output shows RemoveProxy worked:
# [Keycloak] Using: http://localhost:8080
# [KEYCLOAK] Login successful ✅
```

### Monitor client configuration changes
```bash
# If adding more client configuration, ensure RemoveProxy() is still called
# Check tenant_utils.go KeycloakLogin() to verify RemoveProxy() is present
grep -n "RemoveProxy" mage/tenant_utils.go
# Output: 491: restyClient.RemoveProxy() // Don't route through corporate proxy
```

## Related Code References

**Constants** (dev_utils.go):
- Line 480: `adminUser = "admin"` - Keycloak admin username
- Line 482: `defaultServicePort = 443` - HTTPS port
- Line 476: `serviceDomain = "orch-10-139-218-125.pid.infra-host.com"`

**Keycloak Base URL Detection** (dev_utils.go):
- Lines 487-510: `getKeycloakBaseURL()` - Smart URL detection with fallbacks
- Lines 512-531: `canReachKeycloak()` - Connectivity testing

**Keycloak Authentication** (tenant_utils.go):
- Lines 479-503: `KeycloakLogin()` - Client configuration & admin login
- Lines 457-475: `GetKeycloakSecret()` - Retrieve Keycloak password

## Best Practices

When modifying Keycloak client configuration in the future:

1. **Always preserve RemoveProxy()**: Don't remove this call
   ```go
   ✅ GOOD:
   restyClient := client.RestyClient()
   restyClient.SetTimeout(60 * time.Second)
   restyClient.RemoveProxy()  // Keep this!
   client.SetRestyClient(restyClient)
   
   ❌ BAD:
   restyClient := client.RestyClient()
   restyClient.SetTimeout(60 * time.Second)
   // REMOVED RemoveProxy() - will break with corporate proxies!
   client.SetRestyClient(restyClient)
   ```

2. **If adding client certificates**:
   ```go
   restyClient := client.RestyClient()
   restyClient.SetTimeout(60 * time.Second)
   restyClient.RemoveProxy()  // Still needed
   restyClient.SetCertificate(cert)  // Add new config AFTER RemoveProxy
   client.SetRestyClient(restyClient)
   ```

3. **For custom headers/authentication**:
   ```go
   restyClient := client.RestyClient()
   restyClient.SetTimeout(60 * time.Second)
   restyClient.RemoveProxy()  // Still needed
   restyClient.SetHeader("X-Custom", "value")  // Add new config AFTER
   client.SetRestyClient(restyClient)
   ```

4. **Testing**: Always verify with `kubectl port-forward` running:
   ```bash
   # Test locally
   kubectl port-forward -n keycloak-system svc/platform-keycloak 8080:8080 &
   timeout 15 mage tenantUtils:createDefaultMtSetup
   
   # Verify success
   echo $?  # Should be 0
   ```

## Summary

The `RemoveProxy()` configuration is **critical for reliability**:

| Scenario | With RemoveProxy | Without RemoveProxy |
|----------|-----------------|-------------------|
| localhost:8080 | ✅ Works fast | ⚠️ May be intercepted |
| Internal DNS | ✅ Clear failure | ❌ Proxy policy error |
| External domain | ⚠️ Clear timeout | ❌ Proxy policy block |
| **Overall** | **✅ Predictable** | **❌ Unreliable** |

**The current implementation is correct and should not be changed.**
