---
name: pre-env-setting
description: Manage and configure pre-orch.env settings
---

# Pre-Env Setting

Manage and configure pre-orch.env settings.

## What this does
- Reviews current pre-orch.env configuration
- Updates cluster provider settings
- Configures IP addresses for MetalLB
- Sets component enable/disable flags
- Validates configuration for deployment

## Usage
Use this skill when you want to:
- Review pre-orch configuration
- Update cluster settings
- Configure MetalLB IP ranges
- Set max pods per node
- Enable/disable components

---

Manage pre-orch.env by:
1. **Show current configuration**:
   ```bash
   cat pre-orch/pre-orch.env
   ```
2. **Key settings to review**:
   - **CLUSTER_PROVIDER**: `kind`, `k3s`, or `rke2`
   - **MAX_PODS_PER_NODE**: Default 110, adjust for cluster size
   - **METALLB_IP_RANGE**: Must not conflict with DHCP (e.g., "192.168.1.240-192.168.1.250")
   - **EMF_ORCH_IP**: IP from MetalLB range for orchestrator
   - **ENABLE_OPENEBS**: Usually `true` for persistent storage
   - **ENABLE_METALLB**: `true` for load balancer (false for K3s with built-in LB)
3. **Update settings** as requested:
   - Edit specific lines
   - Validate IP format
   - Check for conflicts
4. **Special configurations**:
   - **Coder/single-IP deployments**: Set all IPs to same value
   - **K3s clusters**: May disable MetalLB if using built-in LB
   - **Development (KIND)**: Use default IP ranges
5. **Validate configuration**:
   - IP addresses in valid format
   - MetalLB range has enough IPs
   - No IP conflicts with network
   - Required variables set
6. **Report configuration summary** and any issues found

**Common settings**:
```bash
# Development (KIND)
CLUSTER_PROVIDER=kind
ENABLE_METALLB=true
METALLB_IP_RANGE="172.18.255.200-172.18.255.250"

# Production K3s (without MetalLB)
CLUSTER_PROVIDER=k3s
ENABLE_METALLB=false
MAX_PODS_PER_NODE=200

# Single-IP Coder deployment
CLUSTER_PROVIDER=k3s
METALLB_IP_RANGE="10.0.0.100-10.0.0.100"
EMF_ORCH_IP=10.0.0.100
```
