# Design Proposal: Extensions to Edge Infrastructure Manager Core services and APIs

Author(s): Edge Infrastructure Manager Team

Last updated: 05/13/2025

## Abstract

To properly manage vPRO/Active Management Technology (AMT)/Intel Standard Manageability (ISM) services in the Edge
Orchestrator is required to extend Infrastructure Manager APIs and
[data model](https://github.com/open-edge-platform/infra-core/blob/main/inventory/docs/inventory-er-diagram.svg).

This design proposal elaborate on the extensions to be introduced and the User flows to be supported.

## Proposal

Changes are expected in the data-model and a bit of re-structuring in the existing `HostResource` should be evaluated:

- `BaremetalControllerKind` - identifies the type of Device Management
- `BmcIP` - the IP of the Device Management controller
- `BmcUsername` - the username to access the Device Management controller
- `BmcPassword` - the password to access the Device Management controller

They are currently mis-used by `OnboardingManager`. We should create a proper `migration` and we should include as part
of the migration `PxeMac`. Ideally this will be the steps:

`BaremetalControllerKind` identifies the type of Device Management and becomes a shared enum;

- `BaremetalControllerKind` ->> {UNSPECIFIED, NONE, IPMI, VPRO, PDU}

We introduce a new resource to abstract Baseboard Management Controller `BMCResource` and "connected" to the
[Host](https://github.com/open-edge-platform/infra-core/blob/main/inventory/api/compute/v1/compute.proto#L47) with the
cardinality 0...M.

- `BaremetalControllerKind` - identifies the type of Device Management
- `BmcIP` - the IP of the Device Management controller
- `BmcUsername` - the username to access the Device Management controller
- `BmcPassword` - the password to access the Device Management controller

[Host](https://github.com/open-edge-platform/infra-core/blob/main/inventory/api/compute/v1/compute.proto#L47) will be
extended as follow:

- `BootIP` - IP address learned during the device discovery
- `BootMac` - Mac address learned during the device discovery
- `DesiredPowerState` ->> {UNSPECIFIED, ON, SLEEP, OFF, HIBERNATE, POWER_CYCLE}
- `CurrentPowerState` ->> {UNSPECIFIED, ON, SLEEP, OFF, HIBERNATE, POWER_CYCLE}
- `PowerStatus` ->> is a modern status with some well known messages {Powered On, Light Sleeping, Deep Sleeping,
Powered Off, Hibernated}
- `PowerOffPolicy` ->> {IMMEDIATE_OFF, ORDERED_OFF}
- `PowerOnTime` ->> the time of last startup (from which `Uptime` could be calculated by the UI)
- `AMTSKU` ->> a string reporting AMT/ISM version. `Unsupported` otherwise.

**Note** that technologies like AMT do not provide any info about in progress operations, nor if a reboot is in-progress.
However, The DM Resource Manager can fake the `IN_PROGRESS` statuses until the state in the DMT stack does not
reflect the required operation. For example during the provisioning, the RM can keep the `IN_PROGRESS` state until
the device is connected, move to an error status if the devices does not show up after a given threshold.

**Note2** the behavior of the resource manager is driven by the `DesiredPowerState` and `PowerOffPolicy`, and
internally keeps a timer between soft and hard when `ORDERED_OFF` is set in `PowerOffPolicy`.
The PowerStatus would be updated with this internal behavior in the DM RM, for example: "Soft power off succeeded",
"Soft power off failed after 30s, forcing Hard power off", etc.

**Note3** `AMTSKU` is populated during the device discovery. AMT version decoding is quite complex see
[here][rpc-decoding].
For this reason a string is suggested instead of an Enum.

A new entity called `AMTResource` will be added to Inventory and "connected" to the
[Host](https://github.com/open-edge-platform/infra-core/blob/main/inventory/api/compute/v1/compute.proto#L47) with the
cardinality 0...1.. It will be used mainly to keep track the activation of vPRO/AMT:

- `BaremetalControllerKind` - can be only VPRO
- `DesiredState` ->> {UNSPECIFIED, PROVISION, UNPROVISION, DISCONNECT}
- `CurrentState` ->> {UNSPECIFIED, PROVISION, UNPROVISION, DISCONNECT}
- `AMTStatus` ->> {PROVISIONED, UNPROVISIONED, DISCONNECTED, UNSUPPORTED}

**Note** that technologies like AMT do not provide any info about in progress operations. However, The DM Resource
Manager can fake the `IN_PROGRESS` statuses using `Last*` timestamps exposed by MPS.

Instead, UI or Users can fetch directly from MPS REST APIs (through the MT-GW), using device UUID, the following
additional information such as:

- `SoftwareVersion` - AMT version deployed in the device
- `NetworkState` - the network state of the AMT interface
- `IPAddress` - the IP address of the AMT interface
- `PowerSource` - info about system power source (Battery/Power)
- `Features` - AMT supported features and in-use
- `PowerCapabilities` - AMT power capabilities
- `LastUpdate` - Last change push to the device
- `LastConnected` - Last time the device was connected
- `LastSeen` - Last time the device was seen
- `LastDisconnected` - Last time the device was disconnected

Inventory should not allow the removal of an host if one of the baremetal controller is not nil.

Existing managers used for Onboarding & Provisioning need to be aware of the request to activate vPRO functionality.
A new optional tink-action will be used to enable the feature see [vPRO/AMT/ISM devices activation](./vpro-device.md).

For [automatic (aka nZTP) provisioning][nztp], the Provider resource `infra-onboarding` will not contain any field in
the provider config.

The NA readiness should not consider the `AmtInfo` status, as it is not required to have the node fully operational.

### MVP Requirements

At the time of writing it is expected to support the following User flows:

- User is able to view the Hosts that have activated vPro.
- User is able to select a Host and Power down a device.
- User is able to select a Host and Power up a device.
- The UI/API shall show that the device is Powered up/down.
- User is able to power-cycle the Device.
- User is able to verify if vPRO/ISM is supported or no;
- User is able to recover the device if something goes wrong during the provisioning of the final OS;

Additionally requirements for the e2e user flow:

- User is able to create a vPRO domain profile and upload its provisioning certificate
- User is able to delete a vPRO domain profile

`Activation` and `deactivation` are not user driven flows but will be implemented as part of the lifecycle of the devices.

## Rationale

DMT stack is going to share the same pool of resources of Edge Infrastructure Manager. This includes the usage of some
Foundational Platform Services. Hiding completely MPS/RPS would require proxy the data through the infrastructure
manager and duplicate state.

For example Domain profile and provisioning certificate will be uploaded directly using the RPS REST APIs. AMT Device
information will be fetched directly from MPS without the need to involve Edge Infrastructure Manager.

We will manage through Edge Infrastructure Manager only the features that really matters for the users and that require
additional intelligence and more fine grained control (see later power management).

An alternative data-model design considers AMT and BMC resources integrated in HostResources, however this strategy
makes the lifecycle of the baremetal controllers more complex. For example see the scenario where the host should not
be removed if one of these parts is still present.

Inherently is more clean separate the concerns between Host and its components (for example `BMCResource`).

Status tracking and updates will mostly rely on the state coming from the DMT stack. In this way, we will not need to
rely on any in-band agent and get updates even when the device is in bad state.

To learn more on remote power management see the [Device Management RM proposal](./vpro-rm.md]).

## Affected components and Teams

We report hereafter the affected components and Teams:

- APIs and Inventory (EIM team)

## Implementation plan

Edge Infrastructure Manager team will implement the following functionality to support this design proposal:

- Sanitize Inventory data-model and implement proper migration
- Extend Inventory data-model and implement unit tests
- Tenant controller will be extended to handle properly the removal of the new resources

UI and CLI will not be able to integrate with these APIs and will have to use DMT micro-services
for remote power management. Following items will be delivered in 3.2:

- Extend API and implement integration tests

### Test Plan

Inventory **Unit tests** will be extended accordingly in the affected components.

API integration tests and VIP tests will be improved with additional tests to verify the API/Inventory integration and
to act as smoke/sanity tests for the virtual pipeline.

All the aforementioned tests should include negative and failure scenarios such as failed activations, unsupported
operations.

## Open issues (if applicable)

At the time of writing, the user will not be able to opt-in/opt-out but we commit to add this feature in future
proposals. As explained above activation/deactivation will be integrated in the device lifecycle.

The following requirements are not considered at the time of writing as there is not clarity about their need and
for some of them there will be issues on GNU/Linux OSes because for lack of drivers and compatibility:

- User is able to view the Boot Order of the device.
- User is able to change the Boot Order of the device.
- Support Edge Node that has wired Internet connectivity which requires LAN auth;
- Support Edge Node that has wireless Internet connectivity through WLAN technology;

[rpc-decoding]: https://github.com/device-management-toolkit/rpc-go/blob/d55220bd040807647a1b3a6ce218950de6f90781/internal/local/info.go#L409
[nztp]: https://docs.openedgeplatform.intel.com/edge-manage-docs/main/user_guide/concepts/nztp.html
