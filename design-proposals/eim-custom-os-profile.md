# Design Proposal: Custom OS profile creation in EMF

Author(s): Edge Infrastructure Manager Team

Last updated: 09/22/2025

## Abstract

Provide option to user to create a custom OS profile based on EMT and Ubuntu OS.
Feature should support to create custom Ubuntu OS profile with desired kernel version which is compatible iGPU/dGPU and platform features.
Create a custom EMT OS profile with new OS image version(immutable).
User should be able to host custom Ubuntu or EMT OS images in the internal artifact service with internal CA and OS profile should take into
CA certificate as an input to profile and user is able to provision and use it seemlessly.

## Proposal

1. OS profile should have parameter to pass the TLS CA certificate if OS image is hosted in 
internal artifact service where public CA is not used

2. User should be able to create Custom OS profile using orch-cli in addition to TLS CA certificate

3. Onboard manager should read the CA certificate from the OS profile if user specified in the OS profile.

4. Onboard manager should create tinkerbell workflow by passing ENV variable to nbd_image2disk tinker action.

5. ndb_image2disk tinker action should expose the ENV variable as an argument to pass the TLS CA certificate


```mermaid
sequenceDiagram
%%{wrap}%%
autonumber
    title Custom OS profile support in EMF
    actor us as User

    box rgba(10, 184, 242, 1) Orchestrator Components
        participant api as Infra API
        participant inv as Inventory
        participant om as Onboard Mgr
        participant tb as Tinkerbell     
    end

    box rgba(32, 194, 142, 1) Edge Node with EMT-uOS
        participant tw as Tink worker
        participant nbd as nbd_img2disk
        participant disk as disk
    end

    participant as as artifact service
    us ->> api: Create OS profile with optional TLS CA certificate
    us ->> api: Register the host by using serial no/UUID
    api ->> inv: Store OS profile resource in inventory DB
    om ->> inv: Read the OS profile including OS URL,<br/>TLS CA certificate, sha256sum etc
    om ->> tb: create the tinkerbell workflow by passing URL,<br/> TLS CA certificate to ndb_img2disk
    tw ->> tb: Get the tinkerbell workflow and execute it
    Note over tw,disk: Provisioning flow start
    tw ->> nbd: Execute ndb_img2disk tinker action to stream OS image to disk
    nbd ->> as: use the TLS CA certificate to download the image from artifact service
    nbd ->> disk: Write OS package files to OS parition
    Note over tw,disk: Provisioning flow end

```
