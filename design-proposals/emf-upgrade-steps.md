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
Edge node operators must make sure to manually back up any required application manifests  
and data on the nodes, and EMF deployed applications before commencing  
the EMF 3.0 to 3.1 upgrade.
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

Run the cloud installer script (refer [here][Cloud installer script]) in upgrade mode.  
Detailed steps will be captured in user guide.

[Cloud installer script]: https://docs.openedgeplatform.intel.com/edge-manage-docs/dev/deployment_guide/cloud_deployment/cloud_get_started/cloud_start_installer.html

##### Upgrade On-prem Installation

TODO: Document high level steps here, such as running the on-prem 3.1 upgrade script with required config etc

#### Step 3: Re-onboard edge nodes and deploy apps

- Follow the [onboarding guide][Onboarding guide] to re-onboard and re-provision the edge nodes
- Follow the doc [here][Cluster creation documentation] to create edge clusters
- Apply existing deployment packages to the new clusters
- Manually restore any required apps data from the back up

[Onboarding guide]: https://docs.openedgeplatform.intel.com/edge-manage-docs/dev/user_guide/set_up_edge_infra/edge_node_onboard.html
[Cluster creation documentation]: https://docs.openedgeplatform.intel.com/edge-manage-docs/dev/user_guide/set_up_edge_infra/create_clusters.html
