# Design Proposal: Remote KVM Operations via orch-cli

Author(s): Edge Infrastructure Manager Team

Last updated: 03/27/2026

## Abstract

This document describes the design proposal to support Serial‑over‑LAN (SOL) and Keyboard‑Video‑Mouse (KVM) through Intel® Active Management Technology (AMT), 
provided the target system is vPro‑enabled and provisioned.
The implementation leverages existing MPS infrastructure and maintains full multi-tenancy support.

### SOL (Serial Over LAN) Support
SOL provides a managed text-based console redirection. It is primarily used for troubleshooting at the BIOS level or when the operating system is in a command-line state.

Function: It redirects the serial character stream from the remote device to your management console over the network.

Best For: Accessing BIOS/UEFI settings, interacting with Linux terminal consoles, or viewing BSOD/boot-loader text.

### KVM (Keyboard, Video, Mouse) Support
KVM is the "Remote Desktop" equivalent for out-of-band management. Unlike software-based tools (like TeamViewer), Intel AMT KVM works at the hardware level.

Function: It allows you to see the screen and control the input of a remote system during the entire boot process, including BIOS and OS loading.

Requirements: KVM is only available on Intel vPro® Enterprise platforms. It is generally not available on "Standard Manageability" or "Entry" versions of AMT.

| Feature	| Serial Over LAN (SOL)	| Remote KVM |
|---------|-----------------------|------------|
| 'Interface'	| Text-based (Terminal)	| Graphical (GUI)|
| 'Bandwidth'	| Very Low |	Moderate to High |
| 'BIOS Access' |	Yes (Text-based mode) |	Yes (Full Graphical) |
| 'OS Support' | CLI/ Bootloaders	 | Full Windows/Linux Desktop |
| 'Hardware' |	Most AMT-enabled devices  |	vPro Enterprise only |

### Supported Capabilities
🔹 Serial‑over‑LAN (SOL)
SOL allows text‑based console access (BIOS/boot/OS console) even when the OS is down.
Supports:
1. Remote text console redirection
2. BIOS and boot‑time interaction
3. SOL session start/stop via REST APIs
SOL is available in both Admin Control Mode (ACM) and Client Control Mode (CCM) (consent rules differ)

🔹 KVM (Keyboard‑Video‑Mouse)
 KVM provides full graphical remote control, including BIOS screens.
Supports:
1. Hardware‑based KVM (out‑of‑band)
2. Power‑off and pre‑OS access
3. High‑resolution display support (dependent on AMT version)
4. Browser‑integrated KVM using React/Angular UI components

KVM works even if:
- The OS is crashed
- No OS is installed
- The device is powered off (can power on remotely)

## Proposal

### Hardware & OS Prerequisites
1. Intel® vPro‑enabled CPU (Core i5/i7/i9 vPro or Xeon)
2. Intel® AMT firmware enabled
3. Device reachable via network
4. Network Ports and Firewall  
  | Purpose | Port |
  |-------|-------------|
  | 'AMT WS‑MAN (HTTP)' | 16992 |
  | 'AMT WS‑MAN (HTTPS)' | 16993 |
  | 'KVM / SOL (non‑TLS)' | 16994 |
  | 'KVM / SOL (TLS)' | 16995 |

### Proposed Architecture 
<img width="2443" height="1390" alt="image" src="https://github.com/user-attachments/assets/6b780075-1cfa-41d9-af33-7eb67636ee1b" />

### AMT Redirection

### Component Architecture

The KVM operation involves the following EMF components:

- **infra-core/apiv2**: REST API layer that handles host resource PATCH
  requests with SOL and KVM state changes
- **infra-core/inventory**: PostgreSQL database storing host resources
  including SOL and KVM state fields
- **infra-external/sol_kvm-manager**:
- **/kvmClient** : 
- **mps**: Management Presence Server that generates KVM authorization
  tokens and provides WebSocket endpoints
- **rps**: Remote Provisioning Server that enables KVM during device
  activation

**Authentication Requirements**:

- Keycloak JWT token obtained via `orch-cli login` and stored for
  subsequent commands
- User must belong to tenant that owns the project
- User must have appropriate RBAC permissions for host management


## Implementation Design

### API Design

[KVM operational Flow](https://github.com/open-edge-platform/edge-manageability-framework/blob/678546129823b5b86952bceee16db48142d2f470/design-proposals/vpro-kvm.md)

orch-cli Commands

**Command struct

[SOL operational Flow]

orch-cli Commands

**Command struct

## Affected components

## Test plan

## Architecture Open (if applicable)
