# Design Proposal: EMF upgrade steps

Author(s): Diwan Chandrabose, Sunil Parida

Last updated: 2025-07-07

## Abstract

This proposal captures EMF upgrade paths and the steps involved in the upgrades.
This document has to be reviewed as part of every planned release
to make sure upgrade paths to the new release will be designed and supported.

## Upgrades

### Upgrade from EMF 3.0 to 3.1

#### Disruptive upgrade

In EMF 3.1, the node architecture is being changed to install and run a k3s cluster in edges instead of rke2.
Any edge node onboarded on EMF 3.1 (or later versions in the future) and EMF 3.0 will  
run k3s cluster and rke2 cluster respectively.
When an EMF 3.0 instance has to be upgraded to 3.1,
the existing 3.0 edge nodes will need to be re-onboarded post EMF upgrade.
Edge node operators must make sure to manually back up any application manifests,
EMF deployment packages and data on the nodes before commencing the EMF 3.0 to 3.1 upgrade.
And manually restore the apps and data after EMF upgrade and re-onboarding the nodes.
Future upgrades from 3.1 to 3.2 or later will be non-disruptive
as we dont plan to include breaking changes in the edge node architecture in 3.2.

Any effort to provide an automated edge node rke2 to k3s cluster replacement
in an attempt to build a complete non-disruptive upgrade,
would jeopardize 3.1 release.
More importantly the effort would also be useful only one time
to upgrade any existing 3.0 instance.
3.1 to 3.2 and later upgrades would not require this approach.

#### Upgrade steps

#### Step 1: Delete existing nodes in 3.0

Do the following for each host pertaining to each edge node onboarded in EMF.
De-authorizing and deleting a host does not wipe out any deployment packages that are
deployed in the edge cluster.
The deployment packages can be applied to the host once they are re-onboarded and re-provisioned.

- Manually back up any desired apps data in the edge node/cluster
- De-authorize the host by following the steps given in the doc [here][De-auth Host Documentation]
- Delete the host by following the steps given in the doc [here][Delete Host Documentation]

[De-auth Host Documentation]: https://docs.openedgeplatform.intel.com/edge-manage-docs/dev/user_guide/set_up_edge_infra/deauthorize_host.html
[Delete Host Documentation]: https://docs.openedgeplatform.intel.com/edge-manage-docs/dev/user_guide/set_up_edge_infra/delete_host.html

#### Step 2: Upgrade EMF from 3.0 to 3.1

Once the previous step is executed for each node onboarded in the EMF 3.0 instance,
the instance is ready to be upgraded to 3.1.

##### Upgrade Cloud Installation

TODO: Document high level steps here, such as running the provision scipt in upgrade mode etc

##### Upgrade On-prem Installation

### Overview

The [`onprem_upgrade.sh`](https://github.com/open-edge-platform/edge-manageability-framework/blob/add_upgrade_script/on-prem-installers/onprem/onprem_upgrade.sh) script upgrades your OnPrem Edge Orchestrator installation from v3.0 to v3.1.

### Prerequisites

1. **Current Installation**: OnPrem Edge Orchestrator v3.0 must be installed
2. **PostgreSQL**: Service must be running
3. **Edge Nodes**: Remove all Edge Nodes before upgrade (v3.0 uses RKE2, v3.1 uses K3s)

### Usage

```bash
# orch Main terminal
source .env
unset PROCEED
# Set version
export DEPLOY_VERSION="v3.1.0"

# Run upgrade with backup (recommended)
./onprem_upgrade.sh
```


### Upgrade Process

The script performs the following:

1. **Validates** current installation and PostgreSQL status
2. **Downloads** deb packages and repository artifacts
3. **Prompts** for configuration and manual config file review
4. **Upgrades** components in sequence:
   - OS Configuration
   - Gitea
   - ArgoCD
   - Edge Orchestrator
5. **Restores** PostgreSQL databases and syncs ArgoCD applications

#### Step 3: Re-onboard edge nodes and deploy apps

- Follow the [onboarding guide][Onboarding guide] to re-onboard and re-provision the edge nodes
- Follow the doc [here][Cluster creation documentation] to create edge clusters
- Apply existing deployment packages to the new clusters
- Manually restore any required apps data from the back up

[Onboarding guide]: https://docs.openedgeplatform.intel.com/edge-manage-docs/dev/user_guide/set_up_edge_infra/edge_node_onboard.html
[Cluster creation documentation]: https://docs.openedgeplatform.intel.com/edge-manage-docs/dev/user_guide/set_up_edge_infra/create_clusters.html
