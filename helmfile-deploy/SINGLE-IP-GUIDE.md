# Single-IP Mode — Technical Guide

## 1. Overview

Standard on-prem EMF deployments require **two separate IP addresses** for MetalLB
LoadBalancer services (Traefik and HAProxy). This is impractical for constrained
environments (e.g., Coder VMs, lab machines) that only have a **single IP**.

Single-IP mode removes this barrier by assigning each service a **unique port** on
the same shared IP address.

| Service  | Multi-IP Mode          | Single-IP Mode              |
|----------|------------------------|-----------------------------|
| Traefik  | Own IP, port 443       | Shared IP, port **443**     |
| HAProxy  | Own IP, port 443       | Shared IP, port **9443**    |

**Key design principles:**
- Full backward compatibility — multi-IP mode remains the default
- Single env var (`EMF_ORCH_IP`) enables single-IP mode
- All provisioning URLs (iPXE boot scripts, config maps) automatically include `:9443`
- A Traefik IngressRoute bridges HAProxy content on port 443 for convenience

---

## 2. Architecture

### 2.1 Multi-IP Mode (Default)

```
                    ┌─────────────────────────────┐
                    │        MetalLB              │
                    │                             │
                    │  Pool: "traefik"            │
                    │    IP: 192.168.99.30/32     │
                    │    NS: orch-gateway         │
                    │                             │
                    │  Pool: "haproxy-controller" │
                    │    IP: 192.168.99.40/32     │
                    │    NS: orch-boots           │
                    └──────┬──────────┬───────────┘
                           │          │
              ┌────────────┘          └────────────┐
              ▼                                    ▼
   ┌──────────────────┐                 ┌──────────────────┐
   │  Traefik (:443)  │                 │  HAProxy (:443)  │
   │  192.168.99.30   │                 │  192.168.99.40   │
   │  orch-gateway    │                 │  orch-boots      │
   └────────┬─────────┘                 └────────┬─────────┘
            │                                    │
    ┌───────┴────────┐                    ┌──────┴───────┐
    │ Web UI, API,   │                    │ Tinkerbell   │
    │ Keycloak,      │                    │ PXE Boot     │
    │ Grafana, etc.  │                    │ Files        │
    └────────────────┘                    └──────────────┘
```

### 2.2 Single-IP Mode

```
                    ┌─────────────────────────────┐
                    │        MetalLB              │
                    │                             │
                    │  Pool: "orch-pool"          │
                    │    IP: 192.168.99.30/32     │
                    │    autoAssign: false         │
                    │    Shared via annotation:    │
                    │    allow-shared-ip:          │
                    │      orch-services           │
                    └──────┬──────────┬───────────┘
                           │          │
                    Same IP│          │Same IP
                           ▼          ▼
              ┌────────────────────────────────────┐
              │         192.168.99.30              │
              │                                    │
              │  ┌──────────────┐ ┌──────────────┐ │
              │  │ Traefik      │ │ HAProxy      │ │
              │  │ Port: 443    │ │ Port: 9443   │ │
              │  │ orch-gateway │ │ orch-boots   │ │
              │  └──────┬───────┘ └──────┬───────┘ │
              └─────────┼────────────────┼─────────┘
                        │                │
               ┌────────┴──────┐  ┌──────┴───────┐
               │ Web UI, API,  │  │ Tinkerbell   │
               │ Keycloak,     │  │ PXE Boot     │
               │ Grafana, etc. │  │ Files        │
               └───────────────┘  └──────────────┘

          ┌──────────────────────────────────────────┐
          │  single-ip-routes (Traefik IngressRoute) │
          │                                          │
          │  tinkerbell-haproxy.<domain>:443          │
          │     ──► Traefik ──► tinkerbell nginx     │
          │     (StripPrefix /tink-stack)             │
          │                                          │
          │  This allows port-443 access to PXE      │
          │  files without specifying :9443           │
          └──────────────────────────────────────────┘
```

### 2.3 Request Flow — Single-IP Mode

```
  Client (browser / wget / iPXE)
    │
    │  https://tinkerbell-haproxy.cluster.onprem/tink-stack/keys/Full_server.crt
    │  DNS resolves to 192.168.99.30
    │
    ├──► Port 443 (Traefik)
    │      │
    │      │  IngressRoute: tinkerbell-haproxy-via-traefik
    │      │  Match: Host(`tinkerbell-haproxy.*`) && PathPrefix(`/tink-stack`)
    │      │  Middleware: StripPrefix(/tink-stack)
    │      │  Backend: tinkerbell.orch-infra:8080
    │      │
    │      └──► tinkerbell nginx ──► /keys/Full_server.crt ──► 200 OK
    │
    └──► Port 9443 (HAProxy) — also works
           │
           │  HAProxy Ingress: tinkerbell-haproxy-ingress
           │  Host: tinkerbell-haproxy.cluster.onprem
           │  path-rewrite: /tink-stack/(.*) → /\1
           │  Backend: tinkerbell.orch-infra:8080
           │
           └──► tinkerbell nginx ──► /keys/Full_server.crt ──► 200 OK
```

---

## 3. Configuration

### 3.1 Enable Single-IP Mode

Set `EMF_ORCH_IP` in both env files:

**pre-orch.env:**
```bash
EMF_ORCH_IP=192.168.99.30
# EMF_TRAEFIK_IP and EMF_HAPROXY_IP are auto-derived from EMF_ORCH_IP
```

**post-orch.env:**
```bash
EMF_ORCH_IP=192.168.99.30
```

When `EMF_ORCH_IP` is set:
- `EMF_TRAEFIK_IP` and `EMF_HAPROXY_IP` are automatically set to the same value
- `singleIpMode` becomes `true` in helmfile environments
- `haproxyPort` becomes `9443`
- The `single-ip-routes` chart is enabled
- MetalLB creates a single shared `orch-pool` instead of separate pools

### 3.2 Multi-IP Mode (Default)

Leave `EMF_ORCH_IP` unset/commented and set individual IPs:

```bash
# EMF_ORCH_IP=
EMF_TRAEFIK_IP=192.168.99.30
EMF_HAPROXY_IP=192.168.99.40
```

---

## 4. Files Modified

### 4.1 Pre-Orchestrator (MetalLB + K3s Setup)

| File | Purpose |
|------|---------|
| `pre-orch/pre-orch.env` | Added `EMF_ORCH_IP` variable |
| `pre-orch/pre-orch.sh` | Validates IPs, derives Traefik/HAProxy IPs from `EMF_ORCH_IP` |
| `pre-orch/helmfile.yaml.gotmpl` | Passes `singleIpMode` and uses `EMF_ORCH_IP` as fallback for IPs |
| `pre-orch/metallb/metallb-config/templates/ipAddressPool.yaml` | Creates shared `orch-pool` in single-IP mode |
| `pre-orch/metallb/metallb-config/values.yaml` | Added `singleIpMode` default |
| `pre-orch/metallb/values-metallb-config.yaml.gotmpl` | Passes `singleIpMode` from helmfile values |

### 4.2 Post-Orchestrator (Traefik + HAProxy + Apps)

| File | Purpose |
|------|---------|
| `post-orch/post-orch.env` | Added `EMF_ORCH_IP` variable |
| `post-orch/environments/onprem-eim-settings.yaml.gotmpl` | Added `singleIpMode`, `haproxyPort`, `single-ip-routes.enabled` |
| `post-orch/environments/defaults-disabled.yaml.gotmpl` | Added `single-ip-routes.enabled: false` default |
| `post-orch/values/traefik.yaml.gotmpl` | Adds `orch-pool` + `allow-shared-ip` annotations |
| `post-orch/values/ingress-haproxy.yaml.gotmpl` | Renamed from `.yaml`; adds annotations + port 9443 |
| `post-orch/values/infra-onboarding.yaml.gotmpl` | Appends `:9443` to `provisioningSvc` and `nginxDnsname` |
| `post-orch/helmfile.yaml.gotmpl` | Updated haproxy values ref; added `single-ip-routes` release |
| `post-orch/charts/single-ip-routes/` | New chart: Traefik IngressRoute for HAProxy paths on :443 |

---

## 5. Component Deep-Dive

### 5.1 MetalLB — IP Pool Configuration

**Multi-IP mode** creates two separate pools with `serviceAllocation` to restrict which
namespace can claim each IP:

```yaml
# Pool: traefik → orch-gateway namespace only
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: traefik
spec:
  addresses: ["192.168.99.30/32"]
  autoAssign: true
  serviceAllocation:
    namespaces: [orch-gateway]

# Pool: haproxy-controller → orch-boots namespace only
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: haproxy-controller
spec:
  addresses: ["192.168.99.40/32"]
  autoAssign: true
  serviceAllocation:
    namespaces: [orch-boots]
```

**Single-IP mode** creates one shared pool. Services claim the same IP via the
`metallb.universe.tf/allow-shared-ip` annotation:

```yaml
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: orch-pool
spec:
  addresses: ["192.168.99.30/32"]
  autoAssign: false     # Services must explicitly request via annotation
```

### 5.2 Service Annotations

Both Traefik and HAProxy services get two annotations in single-IP mode:

```yaml
# Traefik Service (orch-gateway namespace)
metadata:
  annotations:
    metallb.universe.tf/address-pool: orch-pool
    metallb.universe.tf/allow-shared-ip: orch-services

# HAProxy Service (orch-boots namespace)
metadata:
  annotations:
    metallb.universe.tf/address-pool: orch-pool
    metallb.universe.tf/allow-shared-ip: orch-services
```

The `allow-shared-ip` annotation value (`orch-services`) must match across all services
that share the IP. MetalLB uses this as a grouping key.

### 5.3 HAProxy Port Remapping

In single-IP mode, HAProxy's HTTPS port is changed from 443 to 9443 to avoid
conflict with Traefik:

```yaml
# ingress-haproxy.yaml.gotmpl
controller:
  service:
    ports:
      https: 9443    # Only in single-IP mode; default is 443
```

### 5.4 Infra-Onboarding URL Updates

The `provisioningSvc` and `nginxDnsname` values conditionally append `:9443`:

```yaml
# When haproxyPort != 443
provisioningSvc: tinkerbell-haproxy.cluster.onprem:9443
nginxDnsname: tinkerbell-haproxy.cluster.onprem:9443
```

This ensures:
- The `boot.ipxe` script downloads vmlinuz/initramfs from `https://tinkerbell-haproxy.<domain>:9443/tink-stack/`
- Edge node ConfigMaps reference the correct port for provisioning

### 5.5 Single-IP Routes Chart

The `single-ip-routes` chart (only enabled when `EMF_ORCH_IP` is set) creates a
Traefik IngressRoute that mirrors HAProxy's tinkerbell path on port 443:

```yaml
# Middleware: strip /tink-stack prefix
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: tink-stack-stripprefix
spec:
  stripPrefix:
    prefixes: [/tink-stack]

# IngressRoute: route tinkerbell-haproxy host through Traefik
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: tinkerbell-haproxy-via-traefik
spec:
  entryPoints: [websecure]
  routes:
    - kind: Rule
      match: Host(`tinkerbell-haproxy.<domain>`) && PathPrefix(`/tink-stack`)
      middlewares:
        - name: tink-stack-stripprefix
      services:
        - name: tinkerbell
          namespace: orch-infra
          port: 8080
          scheme: http
  tls:
    secretName: tls-orch
```

This allows both access patterns:
- `https://tinkerbell-haproxy.<domain>/tink-stack/...` → Traefik (443) → tinkerbell nginx
- `https://tinkerbell-haproxy.<domain>:9443/tink-stack/...` → HAProxy (9443) → tinkerbell nginx

---

## 6. Data Flow: Edge Node PXE Boot (Single-IP)

```
  Edge Node (iDRAC / iPXE UEFI-HTTP Boot)
    │
    │ 1. DHCP → gets IP, next-server = tinkerbell-haproxy.<domain>
    │
    │ 2. HTTP Boot: https://tinkerbell-haproxy.<domain>:9443/tink-stack/signed_ipxe.efi
    │    ──► HAProxy :9443 ──► tinkerbell nginx ──► signed_ipxe.efi
    │
    │ 3. iPXE runs boot.ipxe script:
    │    download-url = https://tinkerbell-haproxy.<domain>:9443/tink-stack
    │
    │ 4. Downloads vmlinuz + initramfs via HAProxy:
    │    ──► HAProxy :9443 ──► tinkerbell nginx ──► vmlinuz-x86_64
    │    ──► HAProxy :9443 ──► tinkerbell nginx ──► initramfs-x86_64
    │
    │ 5. Boots Micro-OS, connects to tink-server for workflows
    │    ──► Traefik :443 ──► tink-server (gRPC)
    │
    │ 6. Agent downloads config (orchRegistry, orchFileServer, etc.)
    │    All config endpoints use Traefik :443
```

---

## 7. Verification Commands

```bash
# Check MetalLB pool
kubectl get ipaddresspool -n metallb-system

# Check service ports and external IPs
kubectl get svc -n orch-gateway traefik
kubectl get svc -n orch-boots ingress-haproxy-kubernetes-ingress

# Check annotations
kubectl get svc -n orch-gateway traefik -o jsonpath='{.metadata.annotations}'
kubectl get svc -n orch-boots ingress-haproxy-kubernetes-ingress -o jsonpath='{.metadata.annotations}'

# Check IngressRoute (single-ip-routes)
kubectl get ingressroute -n orch-gateway tinkerbell-haproxy-via-traefik
kubectl get middleware -n orch-gateway tink-stack-stripprefix

# Test PXE file access via Traefik (port 443)
wget https://tinkerbell-haproxy.<domain>/tink-stack/keys/Full_server.crt --no-check-certificate

# Test PXE file access via HAProxy (port 9443)
wget https://tinkerbell-haproxy.<domain>:9443/tink-stack/keys/Full_server.crt --no-check-certificate

# Verify boot.ipxe has :9443
kubectl exec -n orch-infra deploy/tinkerbell -- cat /usr/share/nginx/html/boot.ipxe | grep download-url

# Verify provisioningSvc has :9443
helm get values infra-onboarding -n orch-infra | grep provisioningSvc
```

---

## 8. Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| MetalLB `invalid CIDR "/32"` | Env vars not exported before running helmfile | Run `set -a && source pre-orch.env && set +a` first, or use `pre-orch.sh` |
| 404 on `tinkerbell-haproxy.<domain>:443` | Hitting Traefik instead of HAProxy | Use `:9443` or deploy `single-ip-routes` chart |
| 404 on `/tink-stack/Installer` | File doesn't exist | Check actual files: `kubectl exec -n orch-infra deploy/tinkerbell -- ls /usr/share/nginx/html/` |
| `single-ip-routes` not enabled | `EMF_ORCH_IP` not set in `post-orch.env` | Uncomment/set `EMF_ORCH_IP=<ip>` in `post-orch.env` |
| HAProxy service still on 443 | Re-deploy needed | `helmfile -e onprem-eim -l app=ingress-haproxy sync` |

---

## 9. Reference: Port Mapping Summary

| Service | Namespace | Multi-IP Port | Single-IP Port | Protocol |
|---------|-----------|---------------|----------------|----------|
| Traefik (web/API gateway) | orch-gateway | 443 | 443 | HTTPS |
| HAProxy (PXE/tinkerbell) | orch-boots | 443 | 9443 | HTTPS |
| Traefik → tinkerbell bridge | orch-gateway | N/A | 443 (via IngressRoute) | HTTPS |
| Tink Server (gRPC) | orch-infra | 42113 | 42113 | gRPC |
| AMT/MPS passthrough | orch-gateway | 4433 | 4433 | TCP |
