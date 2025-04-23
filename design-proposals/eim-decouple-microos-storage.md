# Design Proposal: Decouple MicroOS storage from the orchestrator

Author(s): Tomasz Osiński

Last updated: 23.04.2025

## Abstract

[A short summary of the proposal with an introduction and the problem being
solved by the proposed change. Include useful persona flows.]

Currently, the MicroOS (HookOS) image is stored on the Kubernetes PV and exposed via the Provisioning Nginx server
for Edge Nodes to download the image via iPXE during EN provisioning. This poses the following issues:

- There is a strong coupling between control plane (the Edge Orchestrator) and storage layer. We should isolate them.
- In a remote, distributed environment the Edge Orchestrator may be far from Edge Nodes, causing high delays and possibly low bandwidth.
  Moreover, the access to PV is rate-limited and may be slow (it can have even bigger impact when onboarding multiple nodes in parallel).
  All of these impacts the provisioning time significantly and we already know that the MicroOS download step takes the biggest portion of time during provisioning.

This proposal aims at solving the above issues by decoupling the MicroOS storage from the orchestrator, i.e. removing a dependency on PV for MicroOS.
This should significantly improve the EN provisioning time.

## Proposal

> **NOTE:** This proposal assumes that HookOS is replaced with a minimal Edge Microvisor Toolkit flavor that is used as a Micro-OS for provisioning.

### Challenge

At a high-level, this design proposal moves the MicroOS image storage from the orchestrator to an external storage, such as Release File Server (e.g., S3 bucket)
or a local storage (e.g., a local, on-prem file server). The orchestrator will no longer be responsible for storing the MicroOS image.
Instead, it will only provide a URL to the image stored in an external storage.

Currently, the HookOS image is curated by DKAM and stored on the PV. The curation includes the following operations:

1. Downloads the pre-built HookOS image from the Release Service 
2. Prepares the HookOS configuration file, that contains a set of orchestrator-specific configuration parameters (e.g., proxy settings, URLs, extra hosts), 
3. Generates secure keys and signs all the components of the image (vmlinuz, initramfs)
4. Compresses the HookOS image into the .tar.gz archive 
5. Reads orchestrator CA certificates and injects into the Micro-OS image during build
6. Injects environment variables into the Micro-OS image during build
7. Re-signs the Micro-OS image after CA certificates and environment variables were injected

The solution proposed in this document assumes that the Micro-OS image is stored **outside** of the orchestrator. It means that
DKAM should not do any of the steps above. Therefore, the main challenge to solve is how to provide environment variables to the Micro-OS image downloaded from an external source,
without sacrificing the security.

### Solution

The solution assumes the following:

- iPXE script along with some configuration files (e.g., CA certificates) are still stored on the orchestrator PV. Since these are small files, the provisioning time would not be impacted.
- The MicroOS image is stored in an external storage (e.g., S3 bucket) and the orchestrator provides a URL to the image. The Micro-OS image can be stored on any other external or local server.
- HookOS is replaced with a minimal Edge Microvisor Toolkit flavor that is used as a Micro-OS for provisioning.

```mermaid
sequenceDiagram
%%{wrap}%%
autonumber

  participant user as User

  participant en as Edge Node

  box rgb(235,255,255) Edge Orchestrator

      participant pa as Provisioning Nginx
      participant om as Onboarding Manager
      participant dkam as DKAM
      participant ic as infra-config
      participant inv as Inventory / API

  end

  box rgb(235,255,255) Edge Microvisor Toolkit Build System
      participant emt-ci as EMT CI/CD Pipeline
  end

  participant rs as Release Service File Server

  rect rgb(191, 223, 255)

  note right of emt-ci: Day -1, Micro-EMT build process

  emt-ci->>emt-ci: Build Micro-EMT with all required agents
  emt-ci->>emt-ci: Sign generated Micro-EMT with the DER security key
  emt-ci->>rs: Publish signed Micro-EMT image along with the DER security key 

  end

  ic->>dkam: Read orchestrator CA certificate and cluster-wide variables

  dkam->>dkam: Generates archive with certs and env variables and signs it

  dkam->>pa: Store signed archive on PV 

  dkam->>dkam: Builds and signs iPXE EFI binary

  dkam->>pa: Store signed iPXE EFI binary

  user->>+rs: Download RS certificate & Micro-OS DER key
  rs->>-user: [RS certificate, Micro-OS DER key]

  user->>en: Load RS certificate & Micro-OS DER key to BIOS

  note over en: EN powered on

  en->>pa: Download iPXE binary
  en->>en: EN boots into iPXE binary

  en->>+rs: Download Micro-OS image
  rs->>-en: [Micro-OS image]

  en->>pa: Download signed archive with certs and env variables

  en->>en: Injects certs and env variables to Micro-OS via initrd

  en->>en: Boots into Micro-OS

  note over user,rs: EN continues Onboarding and Provisioning
```

### How the security is ensured

- The TLS certificate is embedded into the iPXE binary and it is used by Edge Node to verify downloaded MicroOS image (?)


## Rationale

[A discussion of alternate approaches that have been considered and the trade
offs, advantages, and disadvantages of the chosen approach.]

## Affected components and Teams

## Implementation plan

[A description of the implementation plan, who will do them, and when.
This should include a discussion of how the work fits into the product's
quarterly release cycle.]

## Open issues (if applicable)

[A discussion of issues relating to this proposal for which the author does not
know the solution. This section may be omitted if there are none.]
