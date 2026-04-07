# OpenEBS LocalPV Deployment

Standalone helmfile for deploying OpenEBS LocalPV dynamic provisioner for EMF on-prem.

Deploys OpenEBS LocalPV v4.3.0 with:
- `openebs-hostpath` StorageClass (set as default)

## Usage

```bash
cd helmfile-deploy/pre-orch/openebs-localpv

# Install (default version 4.3.0)
helmfile apply

# Install with custom version
LOCALPV_VERSION=4.4.0 helmfile apply

# Uninstall
helmfile destroy
```
