<!-- SPDX-FileCopyrightText: 2026 Intel Corporation -->
<!--                                                          -->
<!-- SPDX-License-Identifier: Apache-2.0                      -->

# OpenEBS LocalPV Deployment

Standalone helmfile for deploying OpenEBS LocalPV dynamic provisioner for EOM on-prem.

Deploys OpenEBS LocalPV v4.3.0 with:

- `openebs-hostpath` StorageClass (set as default)

## Usage

```bash
cd pre-orch/openebs-localpv

# Install (default version 4.3.0)
helmfile apply

# Install with custom version
LOCALPV_VERSION=4.4.0 helmfile apply

# Uninstall
helmfile destroy
```
