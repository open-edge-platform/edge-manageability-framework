# Keycloak OIDC Discovery Setup Guide

## Problem
The `secrets-config` pod is stuck in "Progressing" status in CI/on-prem deployments because OIDC discovery validation fails. The pod is configured to reach Keycloak at `https://keycloak.kind.internal`, but DNS resolution for this domain is not configured.

## Root Cause
- `secrets-config` pod needs to reach `https://keycloak.kind.internal/realms/master` for OIDC discovery
- CoreDNS must rewrite `keycloak.kind.internal` → `traefik.orch-gateway.svc.cluster.local` to enable internal pod access
- Traefik then routes HTTPS traffic to the Keycloak service on port 8080
- Without the CoreDNS rewrite rule, DNS resolution fails and OIDC discovery times out

## Solution
The installer now automatically configures CoreDNS during deployment:

```bash
# In installer/Makefile:
${M}/coredns-keycloak: | ${M}/argocd-ready
	@echo "Configuring CoreDNS for Keycloak OIDC discovery..."
	@CLUSTER_DOMAIN=$$(yq -r '.argo.clusterDomain' ${EDGE_MANAGEABILITY_FRAMEWORK_DIR}/orch-configs/clusters/${TARGET_ENV}.yaml); \
	if [ -n "$$CLUSTER_DOMAIN" ] && [ "$$CLUSTER_DOMAIN" != "null" ]; then \
		${EDGE_MANAGEABILITY_FRAMEWORK_DIR}/installer/configure-coredns-keycloak.sh "$$CLUSTER_DOMAIN"; \
	else \
		echo "Warning: clusterDomain not found in cluster config, skipping CoreDNS configuration"; \
	fi
	touch $@
```

## Configuration Steps

### 1. Automatic (via Makefile)
The installer Makefile now automatically runs the CoreDNS configuration as part of the deployment pipeline:

```bash
make install TARGET_ENV=<cluster-name>
```

This will:
1. Create namespaces
2. Set up secrets
3. Deploy ArgoCD
4. **Configure CoreDNS** ← NEW STEP
5. Deploy orchestrator applications

### 2. Manual Configuration (if needed)
If you need to manually configure CoreDNS:

```bash
./installer/configure-coredns-keycloak.sh "orch-10-139-218-125.pid.infra-host.com"
```

This script creates two DNS rewrite rules:
- `keycloak.orch-10-139-218-125.pid.infra-host.com` → `traefik.orch-gateway.svc.cluster.local`
- `keycloak.kind.internal` → `traefik.orch-gateway.svc.cluster.local`

## CI/Mage Deployment Note

When using the Mage build system (`./.github/actions/deploy_kind`), CoreDNS configuration is automatically handled:

```go
// In mage/deploy.go - CoreDNS configuration function
// This function is called by the mage deploy command
// It now ALWAYS configures CoreDNS, even for local-only domains like kind.internal
// Previously it had a skip condition for local domains, which caused CI failures

// CoreDNS rewrite is REQUIRED for all deployments because secrets-config pod uses
// keycloak.kind.internal to reach Keycloak for OIDC discovery validation.
// Without the DNS rewrite rules, the pod cannot reach OIDC endpoint and will timeout.
```

**Important**: The Mage build system was previously skipping CoreDNS configuration for local-only domains. This has been fixed - CoreDNS is now always configured regardless of domain type.

## Verification

After deployment, verify the setup with:

```bash
# Check CoreDNS configuration
kubectl get configmap coredns -n kube-system -o yaml | grep "rewrite name"

# Verify DNS resolution from a test pod
kubectl run -it --rm test --image=curlimages/curl --restart=Never -- \
  nslookup keycloak.kind.internal

# Test OIDC discovery endpoint
kubectl run -it --rm test --image=curlimages/curl --restart=Never -- \
  curl -k -s https://keycloak.kind.internal/.well-known/openid-configuration | head -20
```

## Troubleshooting

### secrets-config pod is stuck "Waiting for OIDC IdP to become ready..."

1. Check if CoreDNS has the rewrite rules:
```bash
kubectl get configmap coredns -n kube-system -o yaml | grep -A 2 "rewrite name"
```

Expected output:
```
        rewrite name keycloak.kind.internal traefik.orch-gateway.svc.cluster.local
        rewrite name keycloak.orch-X-X.pid.infra-host.com traefik.orch-gateway.svc.cluster.local
```

2. Test DNS resolution:
```bash
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  nslookup keycloak.kind.internal
```

3. If DNS doesn't resolve, restart CoreDNS:
```bash
kubectl rollout restart deployment/coredns -n kube-system
```

4. Delete the stuck secrets-config pod to force recreation:
```bash
kubectl delete job -n orch-platform -l app=secrets-config
```

### ArgoCD shows "secrets-config Synced Progressing"

This is **expected** because:
- `secrets-config` is a Kubernetes Job that runs indefinitely as a service
- Jobs are typically expected to complete, but this one runs continuously
- The pod itself is healthy (2/2 Running) even though ArgoCD shows "Progressing"
- This is normal and not a failure state

## Related Files

- `/installer/Makefile` - Deployment orchestration script
- `/installer/configure-coredns-keycloak.sh` - CoreDNS configuration script
- `/argocd/applications/templates/secrets-config.yaml` - secrets-config ArgoCD application
- `/argocd/applications/custom/secrets-config.tpl` - Keycloak OIDC configuration

## Network Architecture

```
┌─────────────────────────────────────────────────────────┐
│ secrets-config pod (orch-platform namespace)             │
│  - Configured to reach: https://keycloak.kind.internal  │
│  - Makes OIDC discovery request                          │
└──────────────────────────┬──────────────────────────────┘
                           │ DNS query: keycloak.kind.internal
                           ▼
┌─────────────────────────────────────────────────────────┐
│ CoreDNS (kube-system namespace)                         │
│  - Rewrite rule: keycloak.kind.internal →              │
│    traefik.orch-gateway.svc.cluster.local              │
└──────────────────────────┬──────────────────────────────┘
                           │ Routes to Traefik service IP
                           ▼
┌─────────────────────────────────────────────────────────┐
│ Traefik (orch-gateway namespace)                        │
│  - IngressRoute: Host(`keycloak.kind.internal`)        │
│  - Terminates HTTPS (port 443)                         │
│  - Forwards to Keycloak service:8080                   │
└──────────────────────────┬──────────────────────────────┘
                           │ HTTP backend
                           ▼
┌─────────────────────────────────────────────────────────┐
│ Keycloak pod (keycloak-system namespace)               │
│  - Listens on :8080 (HTTP)                            │
│  - Serves OIDC discovery at /.well-known/...         │
└─────────────────────────────────────────────────────────┘
```

## Key Configuration Values

| Component | Value |
|-----------|-------|
| Keycloak Hostname | `https://keycloak.{{ .Values.argo.clusterDomain }}` |
| OIDC IdP Address | `https://keycloak.kind.internal` |
| OIDC Discovery URL | `https://keycloak.kind.internal/realms/master` |
| Traefik Route | `Host(\`keycloak.kind.internal\`)` |
| CoreDNS Target | `traefik.orch-gateway.svc.cluster.local` |
| Keycloak Port | 8080 (internal) |
| Traefik Port | 443 (HTTPS) |

