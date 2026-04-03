# Single-IP K3s Orchestrator POC

## Summary

This document records the work done on 2026-03-31 to deploy Edge Orchestrator on a
single Coder VM using K3s with a single IP address for all MetalLB LoadBalancer services.

In a standard on-prem deployment, three separate IPs are required for ArgoCD, Traefik,
and HAProxy. This is impractical for Coder VMs and other environments with limited IP
addresses. Single-IP mode puts all three services on one IP using different ports:

| Service  | Port |
|----------|------|
| Traefik  | 443  |
| ArgoCD   | 8443 |
| HAProxy  | 9443 |

MetalLB is configured with a single shared `orch-pool` instead of three separate pools.
Services coexist on the same IP using `metallb.universe.tf/allow-shared-ip` annotations.

## Deployment Instructions

### Prerequisites

- A Linux VM with at least 30GB disk, 8+ CPU cores, 32GB+ RAM
- A single IP address for the orchestrator
- Docker Hub credentials (for pulling rate-limited images)
- Network connectivity to `registry-rs.edgeorchestration.intel.com`

### Coder Notes

- Ensure you have sufficient space in the root and user directories. 64 GB recommended.

- Ensure the ORCH_DOMAIN variable is set. Reboot your coder instance if it is not.

### Step 1: Clone the repository

```bash
git clone https://github.com/open-edge-platform/edge-manageability-framework.git
cd edge-manageability-framework/on-prem-installers/onprem
```

Check out the branch containing the single-IP fixes (until merged to main):

```bash
git checkout single-ip
```

### Step 2: Configure onprem.env

Edit `onprem.env` and set the following values:

```bash
# Set to the branch where this PR is checked in
export DEPLOY_REPO_BRANCH="single-ip"

# Docker Hub credentials (required to avoid pull rate limits)
export DOCKER_USERNAME=<your-dockerhub-username>
export DOCKER_PASSWORD=<your-dockerhub-password>

# Your cluster domain (must resolve to ORCH_IP from browsers/clients)
# For coder environments, set CLUSTER_DOMAIN to the value of your coder ORCH_DOMAIN.
export CLUSTER_DOMAIN=<your-domain>

# Single-IP mode: set ORCH_IP to your VM's IP address
# Leave ARGO_IP, TRAEFIK_IP, HAPROXY_IP empty
export ORCH_IP='<your-ip>'
```

**If behind a corporate proxy**, also set:

```bash
export ORCH_HTTP_PROXY="http://<proxy-host>:<port>"
export ORCH_HTTPS_PROXY="http://<proxy-host>:<port>"

# CRITICAL: ORCH_NO_PROXY must include Kubernetes-internal DNS suffixes.
# Go's HTTP client matches no_proxy by hostname, not resolved IP.
# Without these entries, in-cluster traffic goes through the proxy and fails
# with x509 certificate errors.
export ORCH_NO_PROXY="localhost,svc,cluster.local,default,internal,\
127.0.0.0/8,10.0.0.0/8,192.168.0.0/16,172.16.0.0/12,169.254.169.254,\
orch-platform,orch-app,orch-cluster,orch-infra,orch-database,cattle-system,\
orch-secret,argocd-repo-server,\
<your-site-specific-no-proxy-entries>"
```

The required `no_proxy` entries are:

| Entry | Purpose |
|-------|---------|
| `svc` | Matches all `*.svc` short service names |
| `cluster.local` | Matches all `*.svc.cluster.local` FQDNs |
| `default` | Matches `kubernetes.default` (K8s API) |
| `internal` | Matches `*.internal` |
| `orch-platform`, `orch-app`, etc. | Matches namespace-scoped service names |
| `argocd-repo-server` | ArgoCD internal communication |

Also adjust the EN_*_PROXY as appropriate.

### Step 3: Install K3s

```bash
bash pre-orch-install.sh k3s install
```

This installs K3s with Traefik and local-storage disabled (replaced by MetalLB and
OpenEBS respectively), and max-pods set to 500.

### Step 4: Deploy Edge Orchestrator

```bash
bash post-orch-install.sh -y
```

The `-y` flag runs non-interactively. This installs ArgoCD, Gitea, generates the
cluster YAML, and deploys the root-app which triggers all other applications.

Monitor deployment progress:

```bash
kubectl get app -A
```

Installation is complete when all applications show `Healthy` and `Synced`.
This typically takes 10-15 minutes.

### Step 5: Trust the self-signed CA certificate

The deployment uses a self-signed CA issued by cert-manager. Clients (orch-cli,
browsers, curl) will reject the TLS certificate unless the CA is added to the
host's trust store:

**Linux (Ubuntu/Debian):**

```bash
kubectl get secret tls-orch -n orch-gateway -o jsonpath='{.data.ca\.crt}' \
  | base64 -d | sudo tee /usr/local/share/ca-certificates/orch-ca.crt > /dev/null
sudo update-ca-certificates
```

**Windows (for browser access):**

First, extract the CA cert on the Linux host:

```bash
kubectl get secret tls-orch -n orch-gateway -o jsonpath='{.data.ca\.crt}' \
  | base64 -d > orch-ca.crt
```

Copy `orch-ca.crt` to the Windows machine (e.g., via `scp`), then import it:

1. Double-click `orch-ca.crt` and click **Install Certificate...**
2. Select **Local Machine**, click Next
3. Select **Place all certificates in the following store**, click Browse
4. Choose **Trusted Root Certification Authorities**, click OK, then Next, then Finish

Alternatively, from an elevated PowerShell:

```powershell
Import-Certificate -FilePath .\orch-ca.crt -CertStoreLocation Cert:\LocalMachine\Root
```

Restart your browser after importing the certificate.

### Step 6: Configure orch-cli and setup multitenancy

Install the `orch-cli` binary if not already available, then configure the endpoint
and log in.

**Note:** The following example uses ORCH_DOMAIN which is usually set inside Coder
environments.

```bash
export ADMIN_USERNAME=admin
export ADMIN_PASSWORD=`kubectl -n orch-platform get secret platform-keycloak -o jsonpath='{.data.admin-password}' | base64 -d && echo`
orch-cli config set api-endpoint https://api.$ORCH_DOMAIN
orch-cli login $ADMIN_USERNAME $ADMIN_PASSWORD
```

```bash
export PROJECT_USERNAME=sample-project-edge-mgr
export PROJECT_PASSWORD=<fill-me-in>
orch-cli set user admin --add-group org-admin-group
orch-cli create organization sample-org
ORG_ID=$(orch-cli get organization sample-org | awk '/^UID:/ {print $2}')
ORCH_PASSWORD=$PROJECT_PASSWORD orch-cli create user $PROJECT_USERNAME --password
orch-cli set user $PROJECT_USERNAME --add-group ${ORG_ID}_Project-Manager-Group
orch-cli login $PROJECT_USERNAME $PROJECT_PASSWORD
orch-cli create project sample-project
PROJECT_ID=$(orch-cli get project sample-project | awk '/^UID:/ {print $2}')
sleep 30s  # wait for project groups to be created
orch-cli login $ADMIN_USERNAME $ADMIN_PASSWORD
orch-cli set user $PROJECT_USERNAME --add-group ${PROJECT_ID}_Edge-Manager-Group
orch-cli set user $PROJECT_USERNAME --add-group ${PROJECT_ID}_Edge-Onboarding-Group
orch-cli set user $PROJECT_USERNAME --add-group ${PROJECT_ID}_Edge-Operator-Group
orch-cli set user $PROJECT_USERNAME --add-group ${PROJECT_ID}_Host-Manager-Group
```

### Step 7: Configure DNS

**Note:** This step is not needed on our Coder development environments.
Coder automatically creates a wildcard DNS record that resolves all subdomains to
the VM's IP address.

For non-Coder environments, add the orchestrator hostnames to `/etc/hosts` on any
machine that needs to access it. Note that using `/etc/hosts` will not suffice for
provisioning edge nodes using UEFI-HTTP. You will have to come up with a real DNS
solution to resolve that.

The following script generates the entries:

```bash
#!/bin/bash
# Usage: generate-hosts.sh <ip> <domain>
# Outputs /etc/hosts entries for all orchestrator subdomains.

if [ $# -ne 2 ]; then
  echo "Usage: $0 <ip> <domain>" >&2
  exit 1
fi

IP=$1
DOMAIN=$2

SUBDOMAINS=(
  web-ui api keycloak vault registry-oci
  observability-ui observability-admin alerting-monitor
  gitea app-orch app-service-proxy ws-app-service-proxy vnc
  docs-ui cluster-management connect-gateway fleet metadata
  infra-node update-node telemetry-node attest-node
  cluster-orch-node onboarding-node onboarding-stream
  device-manager-node logs-node metrics-node release
  tinkerbell-server tinkerbell-haproxy
  mps mps-wss rps rps-wss
)

echo "$IP $DOMAIN"
for sub in "${SUBDOMAINS[@]}"; do
  echo "$IP ${sub}.${DOMAIN}"
done
```

Run it and append to `/etc/hosts`

Alternatively, configure a wildcard DNS record (`*.<domain>` -> `<your-ip>`) to avoid
maintaining individual entries.

### Step 8: Access the orchestrator

- **Web UI:** `https://web-ui.<domain>`
- **ArgoCD:** `https://<domain>:8443`
- **Keycloak:** `https://keycloak.<domain>`
- **Grafana:** `https://observability-ui.<domain>`

### Destroying the K3s Cluster

To completely destroy the deployment and remove K3s from the host:

```bash
cd edge-manageability-framework/on-prem-installers/onprem
bash pre-orch-install.sh k3s uninstall
```

This runs the K3s uninstall script, which stops all K3s services, removes all
containers and pods, deletes the K3s data directory, and removes the K3s binary.
After uninstalling, the following artifacts may remain and can be cleaned up manually:

```bash
# Remove the kubeconfig
rm -f ~/.kube/config

# Remove the self-signed CA from the trust store (if installed)
sudo rm -f /usr/local/share/ca-certificates/orch-ca.crt
sudo update-ca-certificates

# Remove /etc/hosts entries (if added)
# Edit /etc/hosts and remove the orchestrator lines
```

After cleanup, the VM is ready for a fresh deployment starting from Step 3.

## Files Modified

| File | Changes |
|------|---------|
| `on-prem-installers/onprem/pre-orch-install.sh` | max-pods 200->500, node registration wait loop |
| `on-prem-installers/onprem/post-orch-install.sh` | ORCH_IP non-interactive check, ArgoCD shared-IP annotations |
| `on-prem-installers/onprem/onprem.env` | ORCH_IP variable, proxy documentation |
| `on-prem-installers/onprem/cluster_onprem.tpl` | Added `singleIpMode` and `haproxyPort` values |
| `installer/generate_cluster_yaml.sh` | Corrected Helm value paths for ArgoCD and HAProxy ports, export `HAPROXY_PORT` |
| `argocd/applications/custom/infra-onboarding.tpl` | Conditional `:haproxyPort` on `provisioningSvc` and `nginxDnsname` |
| `argocd/applications/templates/metallb-config.yaml` | Reverted to upstream orch-utils chart (v26.1.0) with single-IP support |

`orch-utils` contains a fix to MetalLB to allow creation of a shared IP pool.
That change has been merged.
