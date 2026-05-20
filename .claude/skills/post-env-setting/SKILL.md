---
name: post-env-setting
description: Manage and configure post-orch.env settings
---

# Post-Env Setting

Manage and configure post-orch.env settings.

## What this does
- Reviews current post-orch.env configuration
- Updates deployment profile
- Configures IP addresses for services
- Sets feature flags
- Validates configuration for deployment

## Usage
Use this skill when you want to:
- Review post-orch configuration
- Change deployment profile
- Configure service IP addresses
- Enable/disable features
- Set component versions

---

Manage post-orch.env by:
1. **Show current configuration**:
   ```bash
   cat post-orch/post-orch.env
   ```
2. **Key settings to review**:
   - **DEPLOYMENT_PROFILE**: `onprem-eim` or `onprem-vpro`
   - **EMF_ORCH_IP**: Orchestrator service IP
   - **EMF_TRAEFIK_IP**: Traefik ingress IP
   - **EMF_HAPROXY_IP**: HAProxy IP (if separate)
   - **VERSION_TAGS**: Component version overrides
   - **ENABLE_***: Feature flags for components
   - **Proxy settings**:
     - **EMF_HTTP_PROXY**: HTTP proxy URL for orchestrator services
     - **EMF_HTTPS_PROXY**: HTTPS proxy URL for orchestrator services
     - **EMF_NO_PROXY**: Comma-separated list of hosts/CIDRs to bypass proxy
   - **Edge Node Proxy settings**:
     - **EMF_EN_HTTP_PROXY**: HTTP proxy for edge nodes
     - **EMF_EN_HTTPS_PROXY**: HTTPS proxy for edge nodes
     - **EMF_EN_FTP_PROXY**: FTP proxy for edge nodes
     - **EMF_EN_SOCKS_PROXY**: SOCKS proxy for edge nodes
     - **EMF_EN_NO_PROXY**: No-proxy list for edge nodes
3. **Update settings** as requested:
   - Edit specific lines
   - Validate values
   - Check consistency with profile
4. **Profile-specific considerations**:
   - **onprem-eim**: Standard deployment with EIM
   - **onprem-vpro**: Includes Intel vPro management
5. **Special configurations**:
   - **Coder/single-IP**: All service IPs same as Coder host
   - **Multi-IP**: Separate IPs for Traefik, HAProxy, Orchestrator
   - **Development**: Use default settings
6. **Validate configuration**:
   - IP addresses valid and from MetalLB range
   - Profile matches environment files
   - Feature flags consistent
   - Required variables set
   - No conflicting settings
7. **Report configuration summary** and any issues found

**Common settings**:
```bash
# Standard on-prem EIM
DEPLOYMENT_PROFILE=onprem-eim
EMF_ORCH_IP=192.168.1.240
EMF_TRAEFIK_IP=192.168.1.241
EMF_HAPROXY_IP=192.168.1.242

# vPro deployment
DEPLOYMENT_PROFILE=onprem-vpro
# ... same IPs plus vPro components enabled

# Single-IP Coder
DEPLOYMENT_PROFILE=onprem-eim
EMF_ORCH_IP=10.0.0.100
EMF_TRAEFIK_IP=10.0.0.100
EMF_HAPROXY_IP=10.0.0.100

# Behind corporate proxy
EMF_HTTP_PROXY="http://proxy.corp.com:8080"
EMF_HTTPS_PROXY="http://proxy.corp.com:8080"
EMF_NO_PROXY="localhost,127.0.0.1,10.0.0.0/8,.corp.com"
EMF_EN_HTTP_PROXY="http://proxy.corp.com:8080"
EMF_EN_HTTPS_PROXY="http://proxy.corp.com:8080"
EMF_EN_NO_PROXY="localhost,127.0.0.1,10.0.0.0/8,.corp.com"
```

**Important environment files affected**:
- `post-orch/environments/onprem-eim-settings.yaml.gotmpl`
- `post-orch/environments/onprem-eim-features.yaml.gotmpl`
- `post-orch/environments/profile-vpro.yaml.gotmpl`
