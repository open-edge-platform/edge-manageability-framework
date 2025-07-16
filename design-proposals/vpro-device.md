# Design Proposal: vPRO/AMT/ISM devices activation

Author(s): Edge Infrastructure Manager Team

Last updated: 06/20/2025

## Abstract

vPRO/Active Management Technology (AMT)/Intel Standard Manageability (ISM) needs to be explicitily activated and
configured in the devices before to be consumable from a cloud deployment.

[Open Device Management Toolkit](https://device-management-toolkit.github.io/docs/2.27/GetStarted/overview/) (Open DMT)
provides an open source stack through which is possible to manage vPRO enabled devices.

![EMF Stack](./images/specs/stack.svg)

This document describes the design proposal for adding the Remote Provisioning Client (RPC) component of DMT stack into
the EN sw and propose a solution on how to manage the device activation during the EN journey.

## Proposal

Remote Provisioning Client (RPC) is integrated as part of the Platform Manageability Agent, which operates within the
final operating system environment.

The Platform Manageability Agent template will follow similar design patterns and conventions established by other
agent templates in the EMF, ensuring consistency in configuration, deployment, and management approaches across all
platform agents.

Upon installation, the Platform Manageability Agent performs AMT eligibility and capability detection on the Edge
Node and reports the findings to the Device Management Resource Manager. The agent comes bundled with the `rpc-go`
utility and necessary drivers (including heci) to enable communication with CSME over PCIe/HECI. Based on the
capability detection results, the agent either enables DMT activation features for vPRO/AMT/ISM capable devices or
reports an error status for non-capable devices.

`Local Manageability Service` (LMS) must be included as it is still required to enable the communication between
RPC and AMT device. Additionally, it offers the support for in-band commands too.

By default, all AMT dependencies including LMS are disabled to prevent service activation timeouts on devices that
do not support vPRO/AMT/ISM. The Platform Manageability Agent will be installed and activated on all devices, but
will only enable AMT-related services after successful capability detection via RPC Info command.

For devices that do not support vPRO/AMT/ISM capabilities, the agent will continue running but will report errors
through the RPC Info command, ensuring proper error handling and status reporting to the Device Management Resource Manager.

**Note**: Caddy integration should be reevaluated to support token-based authentication for RPS WebSocket connections,
particularly for the initial WebSocket establishment during vPRO activation.

In cases where activation is attempted on unsupported or faulty devices, the agent will report errors to Device Management.

[DMT/rpc-go documentation](https://device-management-toolkit.github.io/docs/2.27/Reference/RPC/libraryRPC/#rpc-error-code-charts)

Let us now analyze the device activation, the user must do the following steps before starting AMT activation

- AMT is enabled in BIOS
- PKI DNS domain in BIOS matches domain in pfx certificate
- AMT is unprovisioned before starting the provisioning
- MEBx has been set either to default or pre-provisioned by OXM

```mermaid
graph TD
    subgraph "Orchestrator Components"
        inv([**Inventory**]):::orch
        ps([**Provisioning**]):::orch
        dm([**Device Management**]):::orch
        mps([**Management Presence Server**]):::orch
        rps([**Remote Provisioning Server**]):::orch
    end

    subgraph "Edge Node Components"
        en([**Edge Node**]):::edge
        agent([**Platform Manageability Agent**]):::edge
    end

    us([**User**]) -->|1. Boot device| en
    en -->|2. Device discovery| ps
    ps -->|3. Onboard the device| inv
    ps -->|4. Done| en
    en -->|5. OS installation (Agent RPMs included)| en
    en -->|6. Install/Enable Agent as part of OS| agent
    agent -.->|AMT eligibility and capability introspection performed by Agent after install| agent

    us -->|7. Request activation via API| dm
    dm -->|8. Activate command (based on desired state)| agent
    agent -->|9. vPRO remote configuration| rps
    rps -->|10. Success| agent
    agent -->|11. Report AMT status (Provisioned)| dm
    dm -->|12. Update AMT Status IN_PROGRESS (Connecting)| inv
    dm -->|13. Update AMT CurrentState Provisioned| inv

    agent -.->|7a. Report AMT status (Not Supported)| dm
    dm -.->|7b. Update AMT Status ERROR (Not Supported)| inv
    agent -.->|8a. Report AMT status (Failure)| dm
    dm -.->|8b. Update AMT Status FAILURE| inv

    classDef orch fill:#fff,stroke:#003366,stroke-width:3px,color:#003366,font-weight:bold;
    classDef edge fill:#fff,stroke:#228B22,stroke-width:3px,color:#228B22,font-weight:bold;
    classDef default color:#222,stroke-width:2px;
```

**Note 1** - The user interacts with the Device Management API, and the Device Management Resource
Manager instructs the agent to perform activation/deactivation based on desired states.

**Note 2** - MPS requires the creation of a device before accepting CIRA connections which is part of the 2-way auth
implemented between MPS and AMT;

**Note 3** - Device Management Resource Manager will provide a `staticPassword` profile where the AMT and MEBx
passwords are set to a well know value. Disabling this option the RM will randomly generate a password for each
device (using RPS auto-generation) or it will generate a random password and store as secret.

**Note 4** - Passwords are stored in `Vault` and can be always retrieved either using the Vault APIs or through the
web-ui.

**Note 5** - When a device does not support vPRO/ISM capabilities, this is treated as an error condition that needs
to be surfaced to the user. The UX team will determine the best way to present this information to users.

**Note 6** - The deactivation flow is not captured in the current sequence diagram but will be addressed in future
design iterations. Deactivation will be triggered by device deauth/deletion events and processed through the Platform
Manageability Agent.

**Deactivation Flow**: Deactivation will not be explicit by design. However, if a user performs device deauth or
deletion, that event will be captured by the Device Management Resource Manager, and deactivation will be performed
through the agent. After deactivation, if the user wants to activate AMT again, they will need to onboard the device
again.

**Note**: Users should be clearly informed that in the current release, once activated, deactivation is tied to device
deregistration, and reactivation requires a complete re-onboarding process.

### MVP Requirements

At the time of writing it is expected to support the following User flows:

- User is able to verify if vPRO/ISM is supported or no;
- User is able to configure the activation of Edge Node for vPro;
- User is able to recover the device if something goes wrong during the provisioning of the final OS;

It must be carefully considered the impact on the KPIs as the User will experience worse performance when asking the
activation of Device Management feature.

However, such flow is considered not mandatory and this penalty might be accepted by the user to have in exchange extra
manageability features.

## Affected components and Teams

We report hereafter the affected components and Teams:

- Onboarding Manager and Tinker Actions (Edge Infrastructure Manager team)

## Implementation plan

Hereafter we present as steps the proposed plan to manage the device activation in the release 3.1. Edge Infrastructure
manager will implement the following functionality to support this design proposal:

After provisioning is complete and the final OS is deployed, the Platform Manageability Agent will be installed and
initialized as part of the OS. This includes RPC, LMS and exposing the SB gRPC APIs of the Device Management
Resource Manager.

### Test Plan

To ensure the reliability and functionality of the Edge Infrastructure Manager components, it is crucial to component
testing in isolation and by mocking DMT and other deps. **Unit tests** will be extended accordingly in the affected
components.

The integration plan will be split in two flows: i) VIP tests will be extended to verify e2e flow except successfull
activation which cannot be tested using any Virtual Edge Node flavor; ii) New tests involving hardvware devices will be
written to verify the complete e2e flow.

All the aforementioned tests should include negative and failure scenarios such as failed activations, unsupported
operations.

We expect EMT team to conduct integration tests before releasing EMT images supporting RPC and its deps.

## Limitations

The decision to move away from activating DMT at the micro OS level results in the following limitation that should be
captured as a record:
In the previous design where DMT was activated at the micro OS level, if
something went wrong during the final OS provisioning, users still
had access to DMT capabilities. This provided critical recovery mechanisms
including the ability to remotely reboot the device if provisioning
got stuck, access the device out-of-band for troubleshooting, and
recover from provisioning failures without requiring physical access to the device.
By moving activation to post-OS deployment, we lose all these recovery capabilities during the critical OS provisioning phase.
