# Design Proposal: Convert Standalone Edge Nodes to Managed Edge Nodes

Author(s): Tomasz Osiński

Last updated: 18.04.2025

## Abstract

A Customer Journey for Open Edge Platform assumes that customers can manually deploy a set of Standalone
Edge Nodes (SEN) that can be onboarded to the Edge Orchestrator at later stage, once a customer is ready to scale their deployment.
The SENs are converted to managed Edge Nodes that, once onboarded, are fully owned by the Edge Orchestrator - customers
can manage them (e.g., install clusters, applications or perform Day2 OS updates) through the Edge Orchestrator UI and API.

The Customer Journey is as follows:

1. A customer installs one or more Standalone Edge Nodes following the user guides.
2. The customer uses the SEN to deploy K8s clusters, applications, etc.
3. The customer decides to scale out their deployment and onboard the SENs to the Edge Orchestrator.
4. Once SENs are onboarded, the customer starts to use the Edge Orchestrator to manage the SENs. 
   The customer can now use the Edge Orchestrator to manage the SENs, including installing clusters, applications, and performing Day2 OS updates.
5. The customer can now provision additional Edge Nodes via remote provisioning or manually create additional SENs and follow the same workflow to onboard them.

This document describes the design of the onboarding process of Standalone Edge Nodes to managed Edge Nodes (step 3) to allow for all further steps. 

> NOTE: Converting standalone K8s clusters and applications to managed clusters and applications is out of scope for this document. 

## Proposal

### Design goals

This design aims at:

- Providing a solution for onboarding Edge Nodes in a fully automated way, with minimal manual steps.
- Keeping the solution OS-independent, i.e., the solution should work on any OS that is supported by the Edge Orchestrator.
- Avoiding the user interaction (i.e., logging into the OS, injecting USB sticks, etc.) with Edge Node machines.
- Enabling the onboarding process at scale, i.e., the design should enable onboarding of multiple machines at once.

### Assumptions

- Customers will drive the onboarding process from a local developer machine, with desktop, keyboard and mouse.
- The local developer machine will have direct access to ENs via local subnet. Customers can SSH into the ENs.
- The design will re-use the current APIs of Onboarding Manager to drive IO/NIO-based onboarding.
  We may just require small modifications to the Onboarding Manager and the gRPC interface.

### System design

The initial configuration file must provide the following parameters:
- HTTP/HTTPS proxy settings
- Orchestrator certificate
- Orchestrator URL

The following information should be read from Edge Node to map them to EIM resources:

- the OS version from `/etc/os-release` file
- the Secure Boot and Full-Disk Encryption settings
- device identifiers, including UUID, Serial Number, MAC address of the management interface

```mermaid



```

In the registration step, users follow a similar registration process, with several modifications:
- Users should be able to define a local access account. However, the Edge Orchestrator won't import any
  already created users from the EN. OM may need to validate if there is no clash with the local users 
  that may be manually created by customers.





## Rationale

[A discussion of alternate approaches that have been considered and the trade
offs, advantages, and disadvantages of the chosen approach.]

## Affected components and Teams

## Implementation plan

[A description of the implementation plan, who will do them, and when.
This should include a discussion of how the work fits into the product's
quarterly release cycle.]

## Open issues (if applicable)

- The OS version of Standalone ENs deployed by customers may not have a corresponding OS Profile in the Edge Orchestrator.
  For instance, the customer may have deployed a Standalone EN with custom EMT image that have never existed in the Edge Orchestrator.
  Another example is a customer that has deployed a Standalone EN with an old OS image that is not supported by the Edge Orchestrator 
  (which has already been upgraded to use newer versions). There are 2 possible solutions to this:
  - Users should be able to create their own OS Profiles that uses a custom OS image that was used for Standalone ENs.
    This will allow them to scale out with any custom OS image they used, but requires additional steps from the user to "prepare"
    the Edge Orchestrator. Also, there might be a problem of lack of compatibility of new OS Profiles with the old OS images from the same OS family,
    resulting in, for example, failed A/B updates. 
  - Relax the scope of OS Profile. Currently, OS Profiles define the exact version of OS image. We could relax that
    and make OS Profile define "the OS family" - possible A/B updates would only be possible within the same OS family.
    Users may also be able to create their own OS Profiles that define the OS family. In this way, the Standalone EN will only
    be registered to a broad OS family, allow for any OS version within that family. However, it won't allow
    users to provision new ENs with the same OS image that they used for Standalone Edge Node.

- How to map local OS users to local accounts?


[A discussion of issues relating to this proposal for which the author does not
know the solution. This section may be omitted if there are none.]
