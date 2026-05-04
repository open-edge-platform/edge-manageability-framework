# Design Proposal: EOM Out-of-Band Management Capability Roadmap

Author(s): Scott Baker

Last updated: 2026-05-04

## Abstract

Edge Out-of-band Manager (EOM) uses DMT (Device Management Toolkit) as its AMT
execution layer and is embedded in EMF's edge infrastructure platform — alongside
OS lifecycle, cluster orchestration, and workload deployment. This proposal
identifies opportunities to extend EOM's OOB capabilities, positioned against the
full Intel AMT product landscape.

### Intel AMT Product Landscape

Intel offers several ways to consume AMT/vPro manageability:

- **Intel EMA** (Endpoint Management Assistant) — self-hosted, proprietary AMT
  management console. Full AMT surface: bulk power, KVM, boot orchestration,
  certificate management, AMT feature configuration, hardware inventory.
  Targets corporate endpoint fleets (laptops, desktops). Latest version 1.14.5.0
  (December 2025).
- **Intel Endpoint Cloud Services** (ECS) — Intel-hosted AMT backend APIs for
  ISV/UEM integration (Intune, Workspace ONE, RMM tools). Same AMT capabilities
  as EMA, exposed as APIs for partners to embed.
- **Intel vPro Fleet Services** — Intel-delivered SaaS for IT admins, primarily
  through Intune. Abstracts AMT activation and basic remote power/KVM. Simplest
  consumption model.
- **DMT** (Device Management Toolkit) — open-source microservices (MPS, RPS, RPC)
  for building custom AMT backends. Execution layer only; no fleet orchestration,
  scheduling, or multi-tenant lifecycle.

### EMA and ECS Limitations

We use EMA as the primary comparison point because it is the most capable and
complete AMT management product Intel offers — it represents the high-water mark
for what fleet-scale AMT management looks like today. ECS is relevant because it
is the cloud-hosted path that UEM vendors (Workspace ONE, Intune) integrate for
AMT capabilities. Both share the same fundamental constraint.

**Intel EMA:**
- **Windows-only.** The EMA agent runs exclusively on Windows 10/11. The EMA
  server requires Windows Server 2019/2022. There is no Linux support — not for
  managed endpoints, not for the management server. Linux support has been
  mentioned as a long-term roadmap item with no committed timeline.
- **Proprietary and closed-source.** EMA cannot be extended, embedded, or
  customized. It is a standalone console, not a platform component.
- **Single-tenant.** EMA serves one enterprise at a time. It has no multi-tenant
  isolation model.
- **Imperative model.** EMA operates on a request-response basis. There is no
  desired-state reconciliation or automatic convergence.

**Intel Endpoint Cloud Services (ECS):**
- **Windows-only.** Intel's own documentation states: "Intel ECS does not
  currently support Linux operating systems." The endpoint agent/driver is
  available for Windows only. Partner integrations (Workspace ONE chip-to-cloud,
  Intune) target Windows Intel vPro PCs exclusively.
- **Proprietary, Intel-hosted SaaS.** ECS is not downloadable or self-hostable.
  It is a multi-tenant Intel cloud service consumed via partner APIs. It is not
  available directly to end customers — only through ISV/OEM integrations.
- **Not a standalone product.** ECS has no UI, no CLI, no direct operator
  interface. It is a backend that UEM vendors embed. You cannot "use ECS" without
  a compatible management platform (Workspace ONE, Intune, etc.).
- **No versioning or release transparency.** Intel does not publish version
  numbers, release dates, or changelogs for ECS. It is treated as an opaque
  cloud service.

**The common constraint:** Neither EMA nor ECS supports Linux endpoints. Intel's
own guidance for "Intel vPro management on Linux" points away from both products
and toward DMT or EMA's future roadmap. Today, there is no Intel-supported
fleet-scale AMT management solution for Linux.

Edge infrastructure runs Linux. Cell towers, retail gateways, factory
controllers, autonomous systems — the vast majority are Linux-based. Neither EMA
nor ECS can manage them.

**EOM fills this gap: it is the fleet-scale AMT management platform for Linux
edge infrastructure, built cloud-native on open-source DMT.**

### Where EOM Adds Value

EOM is to Linux edge infrastructure what EMA is to Windows enterprise endpoints —
but built on fundamentally different architecture:

1. **Linux-native.** EOM manages Linux edge nodes. EMA cannot. This is not a
   feature gap — it is a platform gap. The entire edge infrastructure market is
   unreachable by EMA.
2. **Open source and cloud-native.** EOM runs on Kubernetes, built on open-source
   DMT. EMA is proprietary Windows Server software. Organizations that won't
   deploy Windows Server in their edge management stack cannot use EMA.
3. **Edge-platform integration** — OOB capabilities woven into the same inventory,
   CLI, API, and observability stack that operators already use for OS and workload
   management. No separate AMT console.
4. **Desired-state reconciliation** — declare intent, system converges. EMA is
   imperative.
5. **Multi-tenant isolation** — EOM is built for multi-tenant edge deployments.
   EMA serves one enterprise at a time.
6. **Capabilities that don't exist in any Intel AMT product** — scheduling tied
   to the edge platform's schedule framework, observability-stack integration,
   natural language via MCP, automated diagnostics workflows.

### Opportunity Categories

Because EMA cannot manage Linux endpoints, capabilities that EMA provides for
Windows do not exist for the Linux edge world. Within the Intel AMT ecosystem,
every capability in this proposal is novel for Linux edge infrastructure — no
Intel AMT product serves this market today.

For clarity, we still categorize opportunities relative to the broader Intel AMT
landscape:

- **Gap** — DMT capabilities that EOM does not yet surface. EMA provides
  equivalent functionality for Windows endpoints, but no solution exists for
  Linux. These are not "catch-up" items — they are greenfield for EOM's target
  market.
- **Augmentation** — new value that goes beyond what any Intel AMT product
  provides on any platform, leveraging EOM's edge-platform integration,
  reconciliation model, or observability infrastructure.
- **Gap + Augmentation** — EOM both surfaces a DMT capability and adds value
  beyond what existing products offer (e.g., fleet-scale event log aggregation
  with observability-stack integration).

Each opportunity is grounded in what DMT provides today, what EOM infrastructure
exists to support it, and what new work is required.

## Proposal

### 1. Bulk Power Operations ✅

**Gap + Augmentation:** Power actions are per-device only. No bulk power-off/on/cycle.

**DMT surface:** MPS POST `/power/action/:guid` — per-device.

**EOM infrastructure:** CLI filter flags (`--filter`, `--site`, `--region`),
AIP-160 metadata filtering, CSV import workflows, desired-state reconciliation
in the device controller.

**Opportunity:** Extend the CLI to set `desired_power_state` on filtered host sets.
The reconciliation engine already handles execution, retries, and status. This is
the lowest-effort, highest-impact opportunity — it uses only existing server-side
code.

**Status:** Implemented for 2026.1.

---

### 2. Scheduled Power Operations

**Augmentation:** No way to schedule power actions for a future time or on a recurring basis.

**DMT surface:** MPS provides AMT alarm clock management
(IPS_AlarmClockOccurrence) for hardware-level wake timers, but these are per-device
and only handle wake — not power-off, reset, or cycle.

**EOM infrastructure:** Full scheduling framework already exists — single-run
schedules with start/end times and repeated schedules with cron expressions
(minute, hour, day-of-month, month, day-of-week). Schedules can target a region,
site, or individual host. The schedule proto supports status types (MAINTENANCE,
OS_UPDATE, etc.) but has no power-related action today.

**Opportunity:** Add a power operation action type to the existing schedule
framework. An operator could create a schedule like "power-cycle all hosts at
site-lab-01 every Sunday at 02:00" or "power off hosts matching
metadata.tier=dev at 18:00 on weekdays." The schedule evaluator would set
`desired_power_state` on matching hosts at the scheduled time, and the device
controller handles execution.

This combines two things DMT cannot do: scheduled execution and fleet targeting.

**New work:** Schedule action type for power, schedule evaluator integration with
host power state patching.

---

### 3. AMT Event Log Monitoring and Alerting

**Gap + Augmentation:** EOM does not expose AMT event logs.

**DMT surface:** MPS GET `/log/event/:guid` returns hardware event log entries —
watchdog state changes, authentication failures, boot failures, firmware progress
events. Per-device only.

**EOM infrastructure:** EMF already deploys Loki (log aggregation) and Mimir
(metrics) as part of its observability stack, with Grafana for dashboards and
alerting.

**Opportunity:** Collect AMT event logs across the fleet and surface actionable
alerts. Examples:

- **Watchdog resets** — a device that is repeatedly triggering watchdog events may
  need attention. EOM could flag it.
- **Boot failures** — devices that fail to boot after a power-cycle could be
  detected and reported.
- **Authentication failures** — repeated AMT auth failures could indicate a
  security issue.

**Integration approach:** A periodic collector (in DM-Manager or as a standalone
job) polls MPS `/log/event/:guid` for each connected device, deduplicates against
a per-device high-water mark (last seen event index or timestamp), and pushes new
entries to Loki via its push API. Each entry is labeled with device ID, site,
region, and tenant for scoped querying. Alerting is handled by Grafana alerting
rules on LogQL queries — e.g., "more than 3 watchdog resets on the same device
in 1 hour" — with no custom alerting engine required. Optionally, derived metrics
(watchdog reset rate per site, boot failure count over time) can be recorded to
Mimir for dashboard use.

This means the only new code is the collector. Aggregation, querying, alerting,
and dashboards come from the existing observability stack.

**New work:** Event log collector with per-device high-water mark tracking and
Loki push integration.

---

### 4. Certificate Inventory and Bulk Deployment

**Gap:** EOM does not manage AMT TLS or 802.1X certificates. EMA provides this
for Windows endpoints; no solution exists for Linux edge nodes.

**DMT surface:** MPS provides certificate CRUD — list, add, delete certificates
on a device. This covers TLS client certificates, trusted root CAs, and 802.1X
wireless/wired credential contexts.

**EOM infrastructure:** None for certificates specifically. But the desired-state
model and reconciliation pattern could apply here.

**Scope:** This proposal covers inventory, expiration visibility, and bulk
deployment of certificates. It does not address issuance, key generation/storage,
renewal automation, revocation (CRL/OCSP), or certificate authority integration —
those are orthogonal concerns owned by the organization's PKI infrastructure.
EOM's role is operational visibility and fleet-scale distribution, not acting as
a certificate authority.

**Opportunity:** At fleet scale, certificate operational burden is about
visibility and deployment:

- **Expiration tracking.** Query certificate expiry dates across the fleet and
  alert before certificates expire. A single expired TLS cert can silently break
  CIRA connectivity.
- **Bulk certificate deployment.** Push a new CA root cert to all devices — for
  example, when rotating the MPS TLS certificate.
- **Certificate inventory.** "Which devices still have the old CA cert?"

No AMT certificate management exists for Linux edge nodes. EOM's approach
integrates with the edge platform's inventory and observability stack —
expiration alerts via Grafana, cert state queryable alongside power and AMT
state in the same API.

**New work:** Certificate inventory model (or extension of host resource),
collection service, expiration alerting, bulk deployment workflow.

---

### 5. Boot Orchestration

**Gap:** EOM does not expose boot options beyond what the power action implicitly
uses. EMA provides remote boot/IDE-R for Windows endpoints; no equivalent exists
for Linux edge nodes.

**DMT surface:** MPS supports a wide range of boot targets: HTTPS boot (with URL
and credentials), PXE boot, IDE-R CD/floppy, BIOS setup, diagnostic mode, and
secure erase. It also exposes boot capabilities per device (what the hardware
supports) and available boot sources.

**EOM infrastructure:** The device controller already sets boot configuration as
part of power actions (via `setBootData` in MPS). But there is no EOM API or CLI
for operators to specify boot targets.

**Opportunity:** Expose boot target selection in the EOM API and CLI:

- **HTTPS boot to a specific URL** across a fleet — useful for OS reimaging.
- **PXE boot for reprovisioning** — target a set of hosts for PXE boot.
- **Secure erase** — wipe devices before decommissioning or repurposing.
- **BIOS setup** — boot to BIOS for configuration changes (combined with KVM for
  remote BIOS access).

The individual boot actions are all handled by MPS. EOM adds fleet targeting,
pre-flight capability validation (query boot capabilities before sending a boot
command that the device doesn't support), and workflow integration.

**New work:** Boot target field on desired state or as a separate operation, CLI
flags, capability query integration.

---

### 6. Power Policies and Guardrails

**Gap + Augmentation:** No guardrails on power operations. Any authorized operator
can power off any host.

**DMT surface:** No policy layer — MPS executes whatever it receives.

**Existing Intel AMT coverage:** EMA and ECS configure AMT hardware-level power
profiles (S0/S3/S5 reachability, AC power behavior) for Windows endpoints. These
control what the firmware does, not what operators are allowed to do. No Intel AMT
product on any platform provides operational guardrails (metadata-based protection
rules, approval workflows).

**EOM infrastructure:** The `PowerCommandPolicy` enum (IMMEDIATE/ORDERED) exists
in the proto but is not implemented. OS update policies exist as a pattern. Metadata
on hosts can serve as policy selectors.

**Opportunity:** Add operational guardrails for power actions:

- **Protect production hosts.** Prevent power-off for hosts with
  `metadata.tier=production` unless an override flag is provided.
- **Ordered shutdown.** For hosts running workloads, ensure the OS is notified
  before a hard power-off (graceful shutdown first, hard power-off after timeout).
- **Approval workflows.** Require confirmation or a second operator's approval for
  power actions affecting more than N hosts.

This is particularly valuable for bulk power operations — the more hosts an
operator can affect with one command, the more important guardrails become.

**Enforcement:** Policy evaluation must happen server-side in the device
controller to be a real guardrail — CLI-only enforcement is bypassable through
the API or other clients. The CLI may additionally surface policy warnings for
operator convenience, but the device controller is the enforcement point.

**New work:** Policy resource definition, server-side policy evaluation in the
device controller, metadata-based policy selectors.

---

### 7. Hardware Inventory Enrichment via AMT

**Gap:** EOM's hardware inventory comes from the OS level (HDA via ipmitool) and
AMT feature detection (PMA via rpc amtinfo). It does not query the full AMT
hardware information. EMA reads AMT hardware classes for Windows endpoints; no
equivalent exists for Linux edge nodes.

**DMT surface:** MPS GET `/hardwareInfo/:guid` returns firmware-level hardware
data: BIOS vendor and version, CPU details, memory configuration, storage devices,
chassis info. This differs from OS-level inventory and is available even when the
OS is down.

**EOM infrastructure:** Unified inventory with host resource model.

**Opportunity:** Enrich the host inventory with AMT-sourced hardware data:

- **BIOS version tracking** across the fleet — identify hosts needing BIOS updates.
- **Memory configuration** — detect misconfigurations or failed DIMMs.
- **AMT firmware version** — track which devices are running outdated AMT firmware.
- **Available when OS is down** — hardware info is queryable via AMT even if the
  host OS is unresponsive.

**New work:** Hardware info collection (periodic or on-demand), storage in host
resource or related model, CLI/API exposure.

---

### 8. AMT Feature Configuration at Scale

**Gap:** EOM does not manage AMT feature flags (KVM enabled, SOL enabled, user
consent mode, etc.). EMA provides this as a core function for Windows endpoints;
no equivalent exists for Linux edge nodes. EOM's differentiator is desired-state
drift detection and reconciliation.

**DMT surface:** MPS GET/POST `/features/:guid` — query and set which AMT features
are enabled, and what user consent policy is in effect (None, KVM-only, All).

**EOM infrastructure:** Desired-state model, fleet filtering.

**Opportunity:** Enforce a consistent AMT feature configuration across the fleet:

- **Ensure KVM and SOL are enabled** on all provisioned devices.
- **Set user consent policy** uniformly (e.g., "All" for production, "None" for
  lab environments based on metadata).
- **Detect drift** — if a feature is disabled on a device, flag or remediate.

This follows the same pattern as AMT provisioning: declare desired configuration,
reconcile toward it.

**New work:** Feature configuration in host desired state, reconciliation in
device controller, CLI/API exposure.

---

### 9. OS Power Saving State Management

**Gap + Augmentation:** MPS supports transitioning devices between full power and
OS power saving modes (action codes 500/501), but EOM does not expose this
capability. Neither EMA, ECS, nor Fleet Services expose this either.

**DMT surface:** MPS handles OS power saving state transitions with validation
(checks current state before acting, handles UNKNOWN and UNSUPPORTED states).

**EOM infrastructure:** Power state model in inventory, device controller
reconciliation.

**Opportunity:** Expose OS power saving state as a controllable dimension:

- **Put idle devices into power saving mode** to reduce energy consumption.
- **Wake devices to full power** before scheduled maintenance or workload
  deployment.
- **Energy management at fleet scale** — combined with scheduling, this enables
  "power saving mode on all lab hosts from 20:00-06:00."

**New work:** OS power saving state field in inventory, reconciliation support,
CLI/API exposure.

---

### 10. Natural Language Query and Action via MCP

**Augmentation:** Operators must know CLI syntax, filter expressions, and valid
power state values to manage the fleet. The learning curve is steep and
error-prone.

**Existing Intel AMT coverage:** No Intel AMT product on any platform has a
natural language interface.

**EOM infrastructure:** EOM already exposes a structured API with AIP-160 filter
expressions, host metadata, power states, AMT states, and site/region hierarchy.
This is a well-defined tool surface that an LLM can call.

**Opportunity:** Build an MCP (Model Context Protocol) server that exposes EOM's
API as tools an LLM can invoke. An operator could type natural language queries
and actions:

- "Turn off the edge nodes that have CVE vulnerabilities"
- "Power-cycle all hosts at site-lab-01 that are currently powered on"
- "How many hosts are AMT-provisioned in the Portland region?"
- "Show me hosts that have been in error state for more than 24 hours"

The MCP server would translate natural language intent into the appropriate API
calls — list hosts with filters, set desired power state, query inventory state.
For destructive actions (power-off, power-cycle), the server would present a
confirmation step showing exactly which hosts would be affected before executing.

This makes the entire EOM capability surface accessible without requiring operators
to learn filter syntax or CLI flags, and it composes naturally with all other
opportunities in this document (bulk power, scheduling, boot orchestration, etc.).

**New work:** MCP server exposing EOM API operations as tools (host list, host
patch, power action, AMT state queries). Confirmation flow for destructive actions.

---

### 11. Automated Remote Diagnostics

**Augmentation:** When a host is unresponsive, an operator must manually check
power state, pull event logs, start a KVM session, and optionally boot to BIOS
or diagnostic mode — each as separate actions across different CLI commands or
API calls.

**Existing Intel AMT coverage:** EMA enables manual diagnostic workflows for
Windows endpoints (KVM, boot to BIOS, file transfer). No Intel AMT product
provides automated diagnostics on any platform, and no solution exists for
Linux edge nodes.

**EOM infrastructure:** Power state and KVM/SOL session state in inventory, event
log collection (idea #3), boot target selection via MPS.

**Opportunity:** A "diagnose this host" operation that chains multiple OOB actions
into a single workflow:

1. Query current power state and AMT connection status.
2. Pull recent AMT event log entries (watchdog resets, boot failures, auth errors).
3. Report a diagnostic summary to the operator.
4. Optionally: start a KVM session for visual inspection, or boot to
   BIOS/diagnostic mode for deeper investigation.

This turns EOM from a bag of individual OOB APIs into an integrated diagnostic
tool. The value is strongest for remote and edge locations where physical access
is expensive or impossible — exactly EOM's target environment.

**Dependencies:** Requires event log collection (#3) to be implemented first —
without it, step 2 of the workflow has no data source. The effort estimate
assumes #3 is already in place.

**New work:** Diagnostic workflow orchestration (CLI command or API operation),
integration with event log collection, conditional KVM/boot-mode triggers.

## Rationale

### Prioritization

| # | Opportunity | Category | Existing Coverage (Windows only) | Effort | Value |
|---|---|---|---|---|---|
| 1 | ✅ Bulk power operations | Gap + Augmentation | EMA: native. ECS: via API. Fleet Services: via Intune. | Low | High |
| 2 | Scheduled power operations | Augmentation | None have native scheduling; EMA relies on external tools. | Medium | High |
| 3 | Event log monitoring | Gap + Augmentation | EMA: receives events, no fleet aggregation. ECS: API only. Fleet Services: no. | Medium | High |
| 4 | Certificate inventory & deployment | Gap | EMA: core function. ECS: API for integrators. Fleet Services: abstracted. | Medium | Medium |
| 5 | Boot orchestration | Gap | EMA: remote boot/IDE-R. ECS: API. Fleet Services: not exposed. | Medium | Medium |
| 6 | Power policies | Gap + Augmentation | EMA: AMT power profiles (hardware-level). No operational guardrails in any product. | Medium | Medium |
| 7 | Hardware inventory via AMT | Gap | EMA: reads AMT HW classes. ECS: API. Fleet Services: via UEM. | Low | Low |
| 8 | AMT feature config at scale | Gap | EMA: core function. ECS: API. Fleet Services: automated. | Medium | Low |
| 9 | OS power saving state | Gap + Augmentation | None of the three expose this. | Low | Low |
| 10 | Natural language via MCP | Augmentation | None of the three have this. | Medium | High |
| 11 | Automated remote diagnostics (requires #3) | Augmentation | EMA: manual workflows. ECS: API hooks. Fleet Services: workflow-level. | Medium | High |

**Categories:**
- **Gap** — DMT capability that EOM does not yet surface. EMA provides equivalent
  functionality for Windows endpoints, but no solution exists for Linux edge
  infrastructure. These are greenfield for EOM's target market.
- **Augmentation** — value that goes beyond what any Intel AMT product provides
  on any platform, leveraging EOM's edge-platform integration, reconciliation
  model, or observability infrastructure.
- **Gap + Augmentation** — EOM both surfaces a DMT capability and adds value
  beyond what existing products offer (e.g., bulk power with desired-state
  reconciliation, event logs with Loki/Grafana integration).

**Key takeaway:** Because EMA is Windows-only, no Intel AMT product serves the
Linux edge market today. The "Existing Coverage" column shows what EMA provides
for Windows endpoints; none of it is available for Linux edge nodes. The
highest-impact opportunities combine this greenfield positioning with
architectural advantages unique to EOM: desired-state reconciliation (#1, #6,
#8), platform-native scheduling (#2), observability-stack integration (#3, #4),
and capabilities that don't exist on any platform (#10, #11).

### Rejected Ideas

**AMT Audit Log Aggregation.** MPS exposes per-device AMT firmware audit logs
(provisioning events, TLS changes, ACL modifications, KVM session starts). The
idea was to aggregate these across the fleet for compliance reporting and security
investigation. Rejected because under normal operation all AMT actions flow through
EOM, which already logs them at the orchestrator level. The firmware audit log only
adds value for tamper evidence or detecting direct AMT access that bypassed EOM —
valid security forensics use cases, but not strong enough to justify a new
collection and storage system for the general case.

**Unified Multi-Protocol OOB.** Provide a single desired-state interface for power
control regardless of whether the underlying device uses AMT, Redfish, or IPMI.
HDA already detects IPMI, and real edge deployments have mixed hardware. Rejected
because EOM is specifically an Intel AMT/vPro management product built on DMT —
adding Redfish and IPMI execution would broaden the scope significantly, require
new protocol client libraries, and dilute the vPro value proposition.

## Implementation Plan

To be determined per opportunity. Bulk power operations (#1) is already
implemented for 2026.1.

## Open Issues

- Event log monitoring (#3) depends on validating Loki push integration from
  within the DM-Manager or a standalone collector. Need to confirm Loki is
  accessible from the orch-infra namespace.
- Certificate inventory (#4) requires understanding how AMT cert expiry dates
  are exposed through the MPS certificate API — specifically whether expiry is
  returned in the existing GET response or requires parsing the cert blob.
- Power policies (#6) need a design for the policy resource schema and
  evaluation hooks in the reconciliation path (enforcement will be
  server-side in the device controller).
- Natural language via MCP (#10) requires deciding which LLM provider to
  integrate with and how to handle authentication for the MCP server.
