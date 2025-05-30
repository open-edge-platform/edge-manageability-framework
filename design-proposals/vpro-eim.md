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

- `BaremetalControllerKind` ->> {UNSPECIFIED, NONE, IPMI, REDFISH, PDU}

We introduce a new resource to abstract Baseboard Management Controller `BaremetalControllerResource` and "connected"
to the [Host](https://github.com/open-edge-platform/infra-core/blob/main/inventory/api/compute/v1/compute.proto#L47)
with the cardinality 0...M.

- `BaremetalControllerKind` - identifies the type of Device Management
- `BaremetalControllerIP` - the IP of the Device Management controller
- `BaremetalControllerUsername` - the username to access the Device Management controller
- `BaremetalControllerPassword` - the password to access the Device Management controller

Inventory should not allow the removal of an host if one of the baremetal controller is not nil.

[Host](https://github.com/open-edge-platform/infra-core/blob/main/inventory/api/compute/v1/compute.proto#L47) will be
extended as follow:

- `BootIP` - IP address learned during the device discovery
- `BootMac` - Mac address learned during the device discovery
- `DesiredPowerState` ->> {UNSPECIFIED, ON, SLEEP, OFF, HIBERNATE, RESET}
- `CurrentPowerState` ->> {UNSPECIFIED, ON, SLEEP, OFF, HIBERNATE, RESET}
- `PowerStatus` ->> is a modern status with some well known messages {Powered On, Light Sleeping, Deep Sleeping,
Powered Off, Hibernated}
- `PowerCommandPolicy` ->> {IMMEDIATE_OFF, ORDERED_OFF}
- `PowerOnTime` ->> the time of last startup (from which `Uptime` could be calculated by the UI)
- `AMTSKU` ->> a string reporting AMT/ISM version. `Unsupported` otherwise.
- `AMTDesiredState` ->> {UNSPECIFIED, PROVISIONED, UNPROVISIONED, DISCONNECT}
- `AMTCurrentState` ->> {UNSPECIFIED, PROVISIONED, UNPROVISIONED, DISCONNECT}
- `AMTStatus` ->> {PROVISIONED, UNPROVISIONED, DISCONNECTED, UNSUPPORTED}

**Note:** `AMTSKU` is populated during the device discovery. AMT version decoding is quite complex see
[here][rpc-decoding]. For this reason a string is suggested instead of an Enum.

AMT does not provide any info about in progress operations, nor if a reboot is in-progress. However, The DM Resource
Manager can fake the `IN_PROGRESS` statuses until the state in the DMT stack does not reflect the required operation.
For example during the provisioning, the RM can keep the `IN_PROGRESS` state until the device is connected, move to an
error status if the devices does not show up after a given threshold or can fake the `IN_PROGRESS` statuses using
`Last*` timestamps exposed by MPS.

The behavior of the resource manager is driven by the `DesiredPowerState` and `PowerCommandPolicy`, and internally
keeps a timer between soft and hard when `ORDERED_OFF` is set in `PowerCommandPolicy`. The PowerStatus would be updated
with this internal behavior in the DM RM, for example: "Soft power off succeeded", "Soft power off failed after 30s,
forcing Hard power off", etc.

Instead, UI or Users can fetch directly from MPS REST APIs (through the MT-GW), using device UUID, the following
additional information such as:

- `SoftwareVersion` - AMT version deployed in the device
- `NetworkState` - the network state of the AMT interface
- `IPAddress` - the IP address of the AMT interface
- `PowerSource` - info about system power source (Battery/Power)
- `LastUpdate` - Last change push to the device
- `LastConnected` - Last time the device was connected
- `LastSeen` - Last time the device was seen
- `LastDisconnected` - Last time the device was disconnected

Existing managers used for Onboarding & Provisioning need to be aware of the request to activate vPRO functionality.
A new optional tink-action will be used to enable the feature see [vPRO/AMT/ISM devices activation](./vpro-device.md)
and should not allow the removal or the deauth of a device if AMT was provisioned.

Additionally, they will have to consider if the device supports the technology, if the User provided all the required
configurations such as Domain/Provisioning certificate.

For [automatic (aka nZTP) provisioning][nztp], the Provider resource `infra-onboarding` will not contain any field in
the provider config regarding vPRO/AMT.

The NA readiness should not consider the vPRO/AMT status, as it is not required to have the node fully operational.

### MVP Requirements

At the time of writing it is expected to support the following User flows:

- User is able to configure the activation of Edge Node for vPro.
- User is able to view the Hosts that have activated vPro.
- User is able to select a Host and Power down a device via vPRO.
- User is able to select a Host and Power up a device via vPRO.
- The UI/API shall show that the device is Powered up/down.
- User is able to power-cycle the Device via vPRO.
- User is able to verify if vPRO/ISM is supported or no;
- User is able to recover the device if something goes wrong during the provisioning of the final OS;

**Note** recovery is consider as a big red-button in case the provisioning does not complete. In this case, the user
will be able to power-cycle the device and delete it from Edge Infrastructure Manager before start the provisioning
again.

Additionally requirements for the e2e user flow:

- User is able to create a vPRO domain profile and upload its provisioning certificate
- User is able to delete a vPRO domain profile

`Deactivation` is not user driven flows but will be implemented as part of the lifecycle of the devices.

## Rationale

OpenDMT stack is going to share the same pool of resources of Edge Infrastructure Manager. This includes the usage of
some Foundational Platform Services. Hiding completely MPS/RPS would require proxy the data through the infrastructure
manager and duplicate state.

For example Domain profile and provisioning certificate will be uploaded directly using the RPS REST APIs. AMT Device
information will be fetched directly from MPS without the need to involve Edge Infrastructure Manager.

Edge Infrastructure Manager will manage only the features/abstractions that really matters for the users and that
require additional intelligence and more fine grained control (see later power management).

An alternative data-model design considers AMT resource as separated entity, however at the time writing there is no
use case that can really benefit of this approach. On the other side makes the lifecycle of the baremetal controllers
more easy. For example see the scenario where the host should not be removed if one of these parts is still present.

Status tracking and updates will mostly rely on the state coming from the OpenDMT stack. In this way, we will not need
to rely on any in-band agent and get updates even when the device is in bad state.

To learn more on remote power management see the [Device Management RM proposal](./vpro-rm.md]).

## Affected components and Teams

We report hereafter the affected components and Teams:

- APIs and Inventory (Edge Infrastructure Manager team)

## Implementation plan

Edge Infrastructure Manager team will implement the following functionality to support this design proposal:

- Sanitize Inventory data-model and implement proper migration
- Extend Inventory data-model and implement unit tests
- Tenant controller will be extended to handle properly the removal of the new resources
- Extend API and implement integration tests

### Test Plan

Inventory **Unit tests** will be extended accordingly in the affected components.

API integration tests and VIP tests will be improved with additional tests to verify the API/Inventory integration and
to act as smoke/sanity tests for the virtual pipeline.

All the aforementioned tests should include negative and failure scenarios such as failed activations, unsupported
operations.

## Open issues (if applicable)

The following requirements are not considered at the time of writing as there is not clarity about their need and
for some of them there will be issues on GNU/Linux OSes because for lack of drivers and compatibility:

- User is able to view the Boot Order of the device.
- User is able to change the Boot Order of the device.
- Support Edge Node that has wired Internet connectivity which requires LAN auth;
- Support Edge Node that has wireless Internet connectivity through WLAN technology;

[rpc-decoding]: https://github.com/device-management-toolkit/rpc-go/blob/d55220bd040807647a1b3a6ce218950de6f90781/internal/local/info.go#L409
[nztp]: https://docs.openedgeplatform.intel.com/edge-manage-docs/main/user_guide/concepts/nztp.html
