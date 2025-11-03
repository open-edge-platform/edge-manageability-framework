# Keycloak URL Detection: Network Architecture & Resolution Strategy

## Problem Statement

When developing locally with Keycloak deployed in a remote Kind cluster (running on AWS Coder), developers encounter connectivity issues trying to authenticate with Keycloak. The system must intelligently detect and use the correct URL based on network location.

## Network Architecture

### Environment Setup
- **Keycloak Deployment**: Kubernetes cluster running on AWS Coder (Cloud-hosted)
- **Keycloak Service**: Internal ClusterIP `10.96.170.2:8080` (platform-keycloak in keycloak-system namespace)
- **Keycloak Domain**: `keycloak.orch-10-139-218-125.pid.infra-host.com` (resolves to `10.139.218.125`)
- **Ingress**: Traefik LoadBalancer in orch-gateway namespace
  - ClusterIP: `10.96.126.191`
  - EXTERNAL-IP (for Kind): `172.18.255.237`
- **Developer Machine**: Local Linux machine outside AWS network
- **Corporate Proxy**: `proxy-dmz.intel.com:912` (Fortinet proxy)
- **no_proxy Configuration**: Includes `10.*, 10.0.0.0/8, pid.infra-host.com`

### Network Connectivity Issues

#### 1. External Domain Path (FAILS from local)
```
Local Developer → Direct to 10.139.218.125:443
                  ❌ Bypasses proxy (10.* in no_proxy)
                  ❌ AWS cloud IP unreachable from local network
                  ❌ TLS handshake: "unexpected eof while reading"

Alternative: Forced through proxy
Local Developer → proxy-dmz.intel.com:912 → Keycloak
                  ✅ Tunnel established
                  ❌ Blocked by proxy policy ("internal IP from external network")
                  ❌ Returns Fortinet proxy block page
```

**Root Cause**: `10.139.218.125` is AWS-hosted IP that:
1. Is NOT reachable from local machine's network
2. Matches `10.* ` pattern in no_proxy → bypasses proxy
3. When forced through proxy, Intel's DMZ policy blocks 10.x ranges as "internal"

#### 2. Internal DNS Path (FAILS from local)
```
Local Developer → DNS lookup platform-keycloak.keycloak-system.svc.cluster.local
                  ❌ Not resolvable (cluster internal DNS, doesn't exist on local)
                  ❌ Returns 504 Bad Gateway through k8s proxy

This only works from inside cluster pods.
```

#### 3. Localhost with Port-Forward Path (WORKS) ✅
```
Local Developer → kubectl port-forward → localhost:8080
                  ✅ No network path issues
                  ✅ Routes to internal service
                  ✅ Works from host machine

This is the reliable solution for local development.
```

## URL Detection Strategy

The code implements intelligent 4-tier fallback detection in `mage/dev_utils.go`:

```go
func getKeycloakBaseURL() string {
    // Priority 1: Explicit override via environment variable
    if envURL := os.Getenv("KEYCLOAK_URL"); envURL != "" {
        return envURL
    }

    candidates := []string{
        // Priority 2: External domain via Traefik ingress
        // Works: Inside AWS cluster or from CI/CD with network access
        // Fails: From local developer machine (network unreachable)
        fmt.Sprintf("https://keycloak.%s:443", serviceDomainWithPort),
        
        // Priority 3: Internal Kubernetes cluster DNS
        // Works: From inside cluster pods
        // Fails: From local developer machine (DNS not resolvable)
        "http://platform-keycloak.keycloak-system.svc.cluster.local:8080",
        
        // Priority 4: Localhost with port-forward
        // Works: Local development with running port-forward
        // Requires: kubectl port-forward svc/platform-keycloak 8080:8080 -n keycloak-system
        "http://localhost:8080",
    }

    for _, url := range candidates {
        if canReachKeycloak(url) {
            fmt.Printf("[Keycloak] Using: %s\n", url)
            return url
        }
        fmt.Printf("[Keycloak] %s - Failed\n", url)
    }

    fmt.Printf("[Keycloak] Falling back to: %s (requires port-forward)\n", fallbackURL)
    return fallbackURL
}
```

### Fallback Behavior at Runtime

```
[Keycloak] Attempting to detect Keycloak URL...
  [Keycloak] https://keycloak.orch-10-139-218-125.pid.infra-host.com:443 - Failed: context deadline exceeded
  [Keycloak] http://platform-keycloak.keycloak-system.svc.cluster.local:8080 - HTTP 504 (failed)
[Keycloak] Falling back to: http://localhost:8080 (requires: kubectl port-forward...)
[KEYCLOAK] Logging in to: http://localhost:8080
Handling connection for 8080
[KEYCLOAK] Login successful ✅
```

## Usage Guidelines

### For Local Development ✅

```bash
# Terminal 1: Set up port-forward
kubectl port-forward svc/platform-keycloak 8080:8080 \
    -n keycloak-system

# Terminal 2: Run mage commands
timeout 10 mage tenantUtils:createDefaultMtSetup

# Expected output:
# [Keycloak] Using: http://localhost:8080
# [KEYCLOAK] Login successful
```

**Why this works:**
- No network path issues between local machine and localhost
- Port-forward tunnels through cluster API to internal service
- Avoids all proxy and network reachability problems

### For CI/CD Inside AWS Cluster ✅

```bash
# Inside pod/CI environment:
mage tenantUtils:createDefaultMtSetup

# Expected: Auto-detects based on network
# Priority 2 (external domain) if accessible from CI network
# Priority 3 (internal DNS) if running as pod in cluster
# Priority 4 (localhost) if port-forward is available
```

**Why this works:**
- CI/CD running in AWS network has network access to cloud IPs
- Internal DNS works from inside cluster
- Fallback chain handles all deployment scenarios

### For Specific Network Scenarios

```bash
# Force specific URL (overrides detection):
export KEYCLOAK_URL="https://keycloak.orch-10-139-218-125.pid.infra-host.com:443"
mage tenantUtils:createDefaultMtSetup

# Use internal DNS from CI pod:
export KEYCLOAK_URL="http://platform-keycloak.keycloak-system.svc.cluster.local:8080"
mage tenantUtils:createDefaultMtSetup

# Use local port-forward:
export KEYCLOAK_URL="http://localhost:8080"
mage tenantUtils:createDefaultMtSetup
```

## Technical Details: Why Each Path Fails/Works

### External Domain (10.139.218.125)

**From Local Developer Machine:**

1. DNS resolves: `keycloak.orch-10-139-218-125.pid.infra-host.com` → `10.139.218.125` ✅
2. no_proxy rule triggers: `10.*` pattern matches → proxy bypassed ✅
3. Direct connection attempt: Local → AWS cloud IP ❌
   - Network route doesn't exist (different network, VPN not active, etc.)
   - TLS handshake fails: `unexpected eof while reading`
4. Alternative (force proxy): `--noproxy "" -x proxy` ✅ connects but...
   - Intel DMZ proxy policy blocks 10.x ranges
   - Returns Fortinet proxy block page
   - Message: "internal IP access not permitted from DMZ proxy"

**From Inside Kubernetes Pod:**
- ✅ Works: Pod can reach any internal service
- ✅ IngressRoute routes to platform-keycloak:8080
- ✅ HTTPS termination at Traefik works

**From Inside AWS Network (CI/CD):**
- ✅ Works: AWS network access to cloud IPs
- ✅ IngressRoute active and accessible

### Internal DNS (platform-keycloak.keycloak-system.svc.cluster.local)

**From Local Developer Machine:**
1. Local resolver doesn't know about `svc.cluster.local` names ❌
2. CoreDNS in cluster (10.96.0.10) only accessible from inside cluster ❌
3. HTTP 504 response (likely from k8s API proxy if configured)
4. Not a viable solution for local development

**From Inside Kubernetes Pod:**
- ✅ Works: CoreDNS resolves cluster DNS names
- ✅ Service IP 10.96.170.2 accessible
- ✅ Standard k8s service discovery

### Localhost with Port-Forward ✅

**From Local Developer Machine:**
```
kubectl port-forward svc/platform-keycloak 8080:8080 -n keycloak-system

Creates: localhost:8080 ← (k8s API tunnel) → ClusterIP 10.96.170.2:8080
                            ↓
                    platform-keycloak (Keycloak pod)
```

**Why it works:**
- Tunnels through Kubernetes API connection
- No external network path required
- No proxy policy restrictions
- No DNS resolution needed
- Simple, reliable, standard Kubernetes practice

**Performance:**
- Minimal latency (direct tunnel through kubectl)
- Suitable for development/testing
- Not recommended for production (manual setup required)

## Code Implementation

See `/mage/dev_utils.go` lines 478-531:

```go
const (
    serviceDomain = "orch-10-139-218-125.pid.infra-host.com"
    defaultServicePort = 443
)

var serviceDomainWithPort = fmt.Sprintf("%s:%d", serviceDomain, defaultServicePort)

func getKeycloakBaseURL() string {
    // 4-tier fallback detection logic
    // With detailed logging for each attempt
}

func canReachKeycloak(url string) bool {
    // Test connectivity with 10-second timeout
    // Accept 200 or 401 (Unauthorized acceptable)
    // Log detailed error information
}
```

## Debugging Tips

### Check Keycloak Detection
```bash
timeout 10 mage tenantUtils:createDefaultMtSetup 2>&1 | head -20
# Look for: [Keycloak] ... - Failed: <reason>
# Reasons: "context deadline exceeded" = timeout, "HTTP 504" = not accessible, etc.
```

### Test Connectivity Manually
```bash
# External domain (will timeout from local):
curl -v https://keycloak.orch-10-139-218-125.pid.infra-host.com:443/realms/master

# Internal DNS (will fail from local):
curl -v http://platform-keycloak.keycloak-system.svc.cluster.local:8080/realms/master

# Localhost (requires port-forward running):
kubectl port-forward svc/platform-keycloak 8080:8080 -n keycloak-system &
curl -v http://localhost:8080/realms/master
```

### Check Proxy Configuration
```bash
echo "http_proxy=$http_proxy"
echo "https_proxy=$https_proxy"
echo "no_proxy=$no_proxy"

# Check if address matches no_proxy:
# 10.139.218.125 matches: 10.*, 10.0.0.0/8 → bypasses proxy
```

### Verify Service Configuration
```bash
# Check Keycloak service:
kubectl get svc -n keycloak-system platform-keycloak
kubectl get svc -n keycloak-system platform-keycloak-headless

# Check Traefik ingress:
kubectl get ingressroute -n orch-gateway orch-platform-keycloak -o yaml

# Check Traefik service (EXTERNAL-IP for local access):
kubectl get svc -n orch-gateway traefik
```

## Summary

| Scenario | URL | Status | Requirements |
|----------|-----|--------|--------------|
| Local dev | localhost:8080 | ✅ Works | `kubectl port-forward` running |
| Inside cluster pod | internal DNS | ✅ Works | Running as pod |
| Inside AWS cluster | external domain | ✅ Works | Network access to cloud IPs |
| Local w/o port-forward | external domain | ❌ Timeout | Network unreachable |
| CI/CD in cloud | external domain | ✅ Works | AWS network access |
| Override | KEYCLOAK_URL env | ✅ Works | Set environment variable |

**The current implementation is correct and handles all scenarios properly through intelligent fallback detection.**
