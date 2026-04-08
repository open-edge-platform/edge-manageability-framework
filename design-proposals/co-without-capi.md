# Design Proposal: Cluster Management Without CAPI

**Status:** Designing

**Date:** TBD

**Authors:**

- TBD

## Context

The current Cluster Management (formerly called Cluster Orchestration / CO) architecture
relies on Cluster API (CAPI) as a core
dependency for cluster lifecycle management. While CAPI provides a standardized framework,
it also introduces additional control-plane components, reconciliation layers, and
operational complexity and maintenance burden that are not always required for EMF target
use cases.

For many environments, cluster creation could be simplified to use K3s deployed via a static
mechanism, such as cloud-init.

## Problem Statement

Remove the dependency on CAPI and create a replacement cluster orchestration mechanism that
uses K3s to directly bring up clusters on edge nodes. Provide a mechanism so that kubeconfig
files may be retrieved from the platform, making those clusters usable.

## Goals

- Remove CAPI
- Add direct K3s cluster provisioning as an optional feature
- Provide a mechanism for listing known clusters in the CLI
- Provide a mechanism for kubeconfig retrieval in the CLI

## Stretch Goals

- Optionally extend kubeconfig retrieval to the GUI
- Optionally enable application management on the cluster. This may involve our existing
  Application Orchestration "AO" layer, or it may involve finding a third-party application
  management solution, such as **Portainer**.

## Non-Goals

- Lifecycle management (Create, Delete, Upgrade) of clusters at runtime. Node should either
  be onboarded with a cluster or without one, and it remains that way for the life of the node.
- GUI support. Any operations described in this guide must be achievable through the CLI.

## Proposal

**Note: Investigate the OXM profile and understand how it provisions K3s. Use that understanding
to inform the ideas below**

### Rough Idea #1: Add a New Cluster Agent

Implement a replacement cluster agent that will be installed by the `node_agent`.
The agent will be responsible for:

- Installing K3s if it is not already installed
- Reporting cluster status and kubeconfig back to a central cluster orchestration database

Implement a simple centralized database based on PostgreSQL that stores information about
clusters (`cluster_hostname`, `cluster_status`, `cluster_kubeconfig`).

### Rough Idea #2: Extend the Existing Node Agent

Same approach as Rough Idea #1, but without implementing a new database or agent.

Extend the existing `node_agent` and the device manager database so that cluster values may be stored
directly in one of the existing EIM databases. This involves only a handful of fields, and there
is a 1:1 mapping between clusters and edge nodes.

### Rough Idea #3: Use Cloud-Init Directly

Rather than using the `node_agent` to drive K3s installation, use `cloud-init` to handle it directly.

Do not implement a mechanism for kubeconfig retrieval. Users would need to SSH
into the node and retrieve the kubeconfig manually.

### Selected Approach: Hybrid Implementation

A hybrid approach will be implemented that combines **Rough Idea #2** and **Rough Idea #3**.

K3s installation will be handled via custom `cloud-init` scripts during node provisioning, eliminating
the need for runtime cluster management. However, kubeconfig retrieval will be supported through
the existing infrastructure by extending the `node-agent` and leveraging existing EIM databases.

The kubeconfig will be stored as a blob in the host metadata field within the inventory system,
providing a centralized location for kubeconfig access without requiring additional database
schemas or components.

## Implementation Plan

### Phase 1: Node-Agent Extension

Extend the `node-agent` with K3s detection capability:

- Add detection logic to identify when K3s is installed and running on the node
- When K3s is detected as available, extract the kubeconfig from the standard K3s
location (`/etc/rancher/k3s/k3s.yaml`)
- Package cluster information into a `ClusterInfo` object as part of the system information

### Phase 2: Infrastructure Manager Integration

Modify the infrastructure manager service to handle cluster information:

- When the `node-agent` makes its regular gRPC call to the infrastructure manager service with
system information, include the `ClusterInfo` object containing the kubeconfig as a blob.

```
message SystemInfo {
  HWInfo hw_info = 1;

  OsInfo os_info = 2;

  BmInfo bm_ctl_info = 3;

  BiosInfo bios_info = 4;

  ClusterInfo kc_info = 5;  <-- populated with the kubeconfig blob
}
```

- The infrastructure manager will detect the presence of cluster information and attach the
kubeconfig blob to the host metadata field
- This metadata will be inserted into the inventory database through the existing host
information update mechanism
- These steps will be accomplished by leveraging the `HostResource` structure that contains a
metadata field

```
// A Host resource.
message HostResource {
  // OTHER METHODS

  // The metadata associated with the host, represented by a list of key:value pairs.
  repeated resources.common.v1.MetadataItem metadata = 5003 [
    (google.api.field_behavior) = OPTIONAL,
    (buf.validate.field).repeated = {
        min_items: 0,
        max_items: 100,
    }
  ];

}
```


### Phase 3: CLI Integration

Extend the `orch-cli` to support kubeconfig retrieval:

- Add functionality to query and display host metadata from the inventory
- Implement a command to extract and save kubeconfig data from the metadata field
- Users can then use the retrieved kubeconfig to interact with their edge clusters

### Phase 4: Validation and Testing

- Validate the end-to-end flow from K3s detection to kubeconfig retrieval
- Test with various edge node configurations
- Ensure proper error handling when K3s is not available or kubeconfig extraction fails
