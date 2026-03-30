# MetalLB Deployment

Standalone helmfile for deploying MetalLB load balancer and IP address pool configuration for EMF on-prem.

Deploys MetalLB v0.15.2 with two IPAddressPools:
- `TRAEFIK_IP` — assigned to Traefik in `orch-gateway`
- `HAPROXY_IP` — assigned to HAProxy in `orch-boots`

## Usage

```bash
cd helmfile-deploy/metallb

# Install
TRAEFIK_IP=192.168.99.30 HAPROXY_IP=192.168.99.40 helmfile apply

# Uninstall
helmfile destroy
```
