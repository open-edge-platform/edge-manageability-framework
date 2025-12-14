# On-Premise Installers - cmd/

This directory contains the **pure shell script** installation components for Edge Orchestrator on-premise deployment.

## Overview

The installation system now works entirely with **shell scripts** - no Go compiler, pre-built binaries, or `.deb` packages are required. Everything is implemented as standalone bash scripts that can run on any Linux system with standard utilities.

## Directory Structure

```
cmd/
├── onprem-config-installer/    # OS configuration installer
│   ├── install.sh              # Pure shell script - configures OS
│   ├── after-install.sh        # Wrapper that calls install.sh
│   └── main.go                 # [DEPRECATED] Legacy Go code (not used)
│
├── onprem-ke-installer/        # Kubernetes Engine (RKE2) installer  
│   ├── install.sh              # Pure shell script - deploys RKE2
│   ├── after-install.sh        # Wrapper that calls install.sh
│   ├── after-upgrade.sh        # Wrapper that calls install.sh -upgrade
│   └── main.go                 # [DEPRECATED] Legacy Go code (not used)
│
├── onprem-gitea/               # Gitea repository server installer
│   ├── install.sh              # Pure shell script - installs Gitea
│   └── after-install.sh        # Same as install.sh (for compatibility)
│
├── onprem-argo-cd/             # ArgoCD GitOps installer
│   ├── install.sh              # Pure shell script - installs ArgoCD
│   └── after-install.sh        # Same as install.sh (for compatibility)
│
└── onprem-orch-installer/      # Main orchestrator installer
    ├── install.sh              # Pure shell script - pushes repos, deploys
    ├── after-install.sh        # Wrapper that calls install.sh
    └── main.go                 # [DEPRECATED] Legacy Go code (not used)
```

## Installation Flow

The installation is executed through the main scripts in `onprem/` directory:

1. **onprem_pre_install.sh** → Calls:
   - `cmd/onprem-config-installer/after-install.sh` (configures OS)
   - `cmd/onprem-ke-installer/after-install.sh` (installs RKE2)

2. **onprem_orch_install.sh** → Calls:
   - `cmd/onprem-gitea/after-install.sh` (installs Gitea)
   - `cmd/onprem-argo-cd/after-install.sh` (installs ArgoCD)
   - `cmd/onprem-orch-installer/after-install.sh` (deploys orchestrator)

3. **onprem_upgrade.sh** → Calls:
   - Various after-install.sh and after-upgrade.sh scripts for upgrades

## How It Works

### Pure Shell Script Execution

Instead of:
```bash
# OLD: Download .deb package → Install with dpkg → Run binary
apt-get install onprem-config-installer.deb
/usr/bin/onprem-config-installer
```

We now use:
```bash
# NEW: Clone repository → Run shell scripts directly
cd edge-manageability-framework/on-prem-installers
bash onprem/onprem_pre_install.sh
  ↓ calls
bash cmd/onprem-config-installer/after-install.sh
  ↓ calls
bash cmd/onprem-config-installer/install.sh  # Pure shell script
```

### Why after-install.sh?

The `after-install.sh` scripts serve as:
1. **Legacy compatibility** - Maintain the same interface that .deb packages used
2. **Environment setup** - Set PATH and other environment variables
3. **Simplification** - Provide a consistent entry point for each installer

## Requirements

- **Bash**: Shell scripting environment
- **Standard Linux utilities**: curl, tar, openssl, base64, grep, sed, awk, etc.
- **kubectl**: Kubernetes command-line tool (installed by RKE2)
- **helm**: Kubernetes package manager (installed by config-installer)
- **NO Go compiler required** - Everything runs as shell scripts

## Key Components

### 1. onprem-config-installer

**Purpose**: Configure the operating system for Edge Orchestrator

**Script**: `install.sh`

**What it does**:
- Updates sysctl configuration (inotify limits)
- Installs yq (YAML processor) tool
- Installs Helm chart manager
- Creates required hostpath directories (/var/openebs/local)
- Configures kernel modules (dm-snapshot, dm-mirror)

**Entry point**: `after-install.sh` → `install.sh`

### 2. onprem-ke-installer

**Purpose**: Deploy RKE2 Kubernetes cluster

**Script**: `install.sh`

**What it does**:
- Runs RKE2 installer script
- Customizes RKE2 configuration
- Installs OpenEBS LocalPV provisioner
- Creates etcd certificates secret
- Verifies cluster health

**Entry point**: 
- Install: `after-install.sh` → `install.sh`
- Upgrade: `after-upgrade.sh` → `install.sh -upgrade`

### 3. onprem-gitea

**Purpose**: Install Gitea Git repository server

**Script**: `install.sh`

**What it does**:
- Generates TLS certificates for Gitea
- Creates Gitea namespaces and secrets
- Deploys Gitea via Helm
- Creates Gitea user accounts (argocd, apporch, clusterorch)
- Generates access tokens for each user

**Entry point**: `after-install.sh` → `install.sh`

### 4. onprem-argo-cd

**Purpose**: Install ArgoCD GitOps continuous deployment tool

**Script**: `install.sh`

**What it does**:
- Configures proxy settings for ArgoCD
- Mounts TLS certificates from Gitea
- Deploys ArgoCD via Helm with custom values

**Entry point**: `after-install.sh` → `install.sh`

### 5. onprem-orch-installer

**Purpose**: Deploy Edge Orchestrator software via ArgoCD

**Script**: `install.sh`

**What it does**:
- Extracts edge-manageability-framework tarball
- Pushes repository contents to Gitea (via Kubernetes job)
- Creates Gitea credential secrets for ArgoCD
- Installs ArgoCD root-app to trigger deployment

**Entry point**: `after-install.sh` → `install.sh`

## Environment Variables

The installers use these environment variables:

- `ORCH_INSTALLER_PROFILE`: Deployment profile (e.g., "onprem", "onprem-dev")
- `GIT_REPOS`: Path to directory containing repository tarballs
- `IMAGE_REGISTRY`: Docker registry for Gitea images (default: docker.io)
- `DOCKER_USERNAME`, `DOCKER_PASSWORD`: Optional credentials for RKE2 customization
- `KUBECONFIG`: Path to Kubernetes config (auto-set to ~/.kube/config)

## Development Notes

### Main.go Files (Deprecated)

The `main.go` files in each directory are **DEPRECATED** and no longer used during installation. They are kept for:
- Historical reference
- Understanding the original logic
- Potential future reference if needed

**All installation now uses pure shell scripts** - no Go compiler required.

### Adding New Installers

To add a new installer component:

1. Create a new directory under `cmd/`
2. Create `install.sh` with your installation logic (pure shell script)
3. Create `after-install.sh` wrapper script:
   ```bash
   #!/usr/bin/env bash
   set -o errexit
   export PATH=$PATH:/usr/local/bin
   SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
   bash "$SCRIPT_DIR/install.sh"
   ```
4. Make scripts executable: `chmod +x install.sh after-install.sh`
5. Call it from the appropriate main installation script in `onprem/`

### Testing

Test installers individually:
```bash
# Test config installer
sudo bash cmd/onprem-config-installer/after-install.sh

# Test KE installer
sudo bash cmd/onprem-ke-installer/after-install.sh

# Test Gitea installer
sudo IMAGE_REGISTRY=docker.io bash cmd/onprem-gitea/after-install.sh

# Test with install.sh directly
sudo bash cmd/onprem-config-installer/install.sh
```

## Migration from .deb Packages

Previously, these installers were:
1. Built as Go binaries using Mage
2. Packaged as .deb files using FPM
3. Downloaded from release service
4. Installed with `dpkg -i` or `apt-get install`
5. Executed as `/usr/bin/onprem-*` binaries

Now they are:
1. ~~Built~~ No build step needed
2. ~~Packaged~~ No packaging needed  
3. ~~Downloaded~~ No download needed
4. ~~Installed~~ No installation needed
5. Executed directly as shell scripts

This change:
- ✅ Eliminates build infrastructure requirements
- ✅ No Go compiler dependency during installation
- ✅ Simplifies the release process
- ✅ Makes code more transparent and auditable
- ✅ Reduces deployment artifacts (no .deb files, no binaries)
- ✅ Enables easier customization by users
- ✅ Works on any Linux system with standard utilities
- ✅ Instant execution (no compilation overhead)

## Troubleshooting

**Error: "bash: install.sh: No such file or directory"**
- Ensure you're running from the correct directory
- Check that install.sh exists and is executable: `ls -la cmd/*/install.sh`

**Installer fails with permission denied**
- Run with sudo: `sudo bash cmd/*/after-install.sh`
- Some installers need root for system configuration

**kubectl command not found**
- RKE2 installer hasn't run yet
- PATH needs /usr/local/bin (set by after-install.sh)

**helm command not found**
- Config installer hasn't run yet
- Run onprem_pre_install.sh first to install dependencies

## See Also

- Main installation README: `../onprem/README.md`
- RKE2 scripts: `../rke2/`
- Mage build scripts (deprecated): `../mage/`
