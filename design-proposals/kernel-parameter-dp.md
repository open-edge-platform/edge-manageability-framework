# Design Proposal: Kernel parameter update on Edge Nodes

## Abstract

This document proposes a mechanism to manage kernel parameters on edge nodes in Edge Manageability Framework (EMF).
Kernel parameters are crucial for tuning the operating system to meet the specific performance, security, and resource
constraints of diverse edge environments. The ability to remotely modify these parameters is an essential feature for
managing and optimizing edge nodes at scale.

## Proposal

The proposed mechanism allows for the update of kernel parameters on edge nodes as a day-2 operation.
The update process is exclusively managed through the Orch-cli.
A user specifies the desired kernel parameters within the `OSUpdatePolicy` object.
Subsequently, the user schedules a maintenance window.
During this window, the Maintenance Manager (MM) and Platform Update Agent (PUA) will apply the specified
kernel parameters to the target EdgeNode.

Kernel Parameter workflow

```mermaid
sequenceDiagram
%%{wrap}%%
autonumber
    participant User
    box Edge Infrastructure Manager
    participant Inventory
    participant MM as Maintenance Manager
    end
    box Edge Node
    participant PUA as Platform Update Agent
    end

    rect rgb(255,255,255)
        note over User,Inventory: User creates an OS Update Policy with desired kernel parameter values and schedule an update
        User->>Inventory: Create new OSUpdatePolicy
        User->>Inventory: Assign OSUpdatePolicy to Instance via the update_policy field
        User->>Inventory: Create a OS Update Schedule for an Instance
    end

    loop Edge Node polling
        PUA->>MM: PlatformUpdateStatusRequest
        note left of PUA: Response contains information from update_policy and the scheduled update
        MM->>PUA: PlatformUpdateStatusResponse
    end
    
    note over MM,PUA: Update Schedule Start
    PUA->>PUA: Start Update
    PUA->>MM: PlatformUpdateStatusRequest with updateStatu "STATUS_TYPE_STARTED"
    MM->>Inventory: Create an OSUpdateRun, with start_time, linking to the OSUpdatePolicy
    
    alt Update successful on the Edge Node
        Edge Node->>MM: PlatformUpdateStatusRequest with updateStatus "STATUS_TYPE_UPDATED"
        MM->>Inventory: Update OSUpdateRun with the IDLE status and end_time
    else Update failed on the Edge Node
        Edge Node->>MM: PlatformUpdateStatusRequest with updateStatus "STATUS_TYPE_FAILED"
        MM->>Inventory: Update OSUpdateRun with the ERROR status, end_time and status_details
    end
```

### Proposed changes

#### EIM and Maintenance Manager

The validation logic that blocks kernel parameter updates for immutable operating systems will be removed.
This change enables kernel parameter modifications on all supported OS types, ensuring consistent manageability.

#### Platform Update Agent (PUA)

The Platform Update Agent (PUA) will be updated to manage kernel parameter modifications on immutable operating systems.
The current implementation, which disallows these changes, will be refactored to provide a unified update mechanism for
both mutable and immutable operating systems. This ensures a consistent application of kernel parameters across all
supported OS types.

#### Orch-CLI

- Update Orch-cli to pass kernel parameter as part of OSProfileUpdate.
- Schedule the Edgenode in maintainance mode so that Kernel Paramter changes can be applied.

### Limitation

- On EMT the Kernel parameter can be applied only in non Secure boot mode.
