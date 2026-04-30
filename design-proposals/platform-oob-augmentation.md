# Design Proposal: EOM Out-of-Band Management Capability Roadmap

Author(s): Scott Baker

Last updated: 2026-04-30

## Abstract

Edge Out-of-band Manager (EOM) uses DMT (Device Management Toolkit) as its AMT
execution layer. DMT exposes a rich per-device AMT surface through MPS: power
control, KVM, SOL, IDE-R, audit logs, event logs, alarm clocks, certificates,
hardware info, boot options, network configuration, and general settings. Most of
this surface is consumed directly by callers one device at a time.

EOM's value proposition is to take that per-device surface and make it operational
at fleet scale — with automation, state tracking, and operator workflows. This
proposal identifies opportunities in two categories:

- **EOM Gap** — DMT capabilities that EOM does not yet expose to operators.
- **DMT Augmentation** — new value that goes beyond what DMT provides at any
  scale, such as fleet targeting, scheduling, reconciliation, and alerting.

Each opportunity is grounded in what DMT provides today, what EOM infrastructure
exists to support it, and what new work is required.

## Proposal

### 1. Bulk Power Operations ✅

**Gap:** Power actions are per-device only. No bulk power-off/on/cycle.

**DMT surface:** MPS POST `/power/action/:guid` — per-device.

**EOM infrastructure:** CLI filter flags (`--filter`, `--site`, `--region`),
AIP-160 metadata filtering, CSV import workflows, desired-state reconciliation
in the device controller.

**Opportunity:** Extend the CLI to set `desired_power_state` on filtered host sets.
The reconciliation engine already handles execution, retries, and status. This is
the lowest-effort, highest-impact opportunity — it uses only existing server-side
code.

**Status:** Plan written in PLAN-BULK-POWER.md.

---

### 2. Scheduled Power Operations

**Gap:** No way to schedule power actions for a future time or on a recurring basis.

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

**Gap:** EOM does not expose AMT event logs.

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

### 4. Certificate Lifecycle Management

**Gap:** EOM does not manage AMT TLS or 802.1X certificates.

**DMT surface:** MPS provides full certificate CRUD — list, add, delete
certificates on a device. This covers TLS client certificates, trusted root CAs,
and 802.1X wireless/wired credential contexts.

**EOM infrastructure:** None for certificates specifically. But the desired-state
model and reconciliation pattern could apply here.

**Opportunity:** At fleet scale, certificate management becomes a real operational
burden:

- **Expiration tracking.** Query certificate expiry dates across the fleet and
  alert before certificates expire. A single expired TLS cert can silently break
  CIRA connectivity.
- **Bulk certificate deployment.** Push a new CA root cert to all devices — for
  example, when rotating the MPS TLS certificate.
- **Certificate inventory.** "Which devices still have the old CA cert?"

This is indisputable value: DMT can manage certs one device at a time, but
tracking expiry and pushing updates across hundreds of devices is manual and
error-prone without fleet-level orchestration.

**New work:** Certificate inventory model (or extension of host resource),
collection service, expiration alerting, bulk deployment workflow.

---

### 5. Boot Orchestration

**Gap:** EOM does not expose boot options beyond what the power action implicitly
uses.

**DMT surface:** MPS supports a wide range of boot targets: HTTPS boot (with URL
and credentials), PXE boot, IDE-R CD/floppy, BIOS setup, diagnostic mode, secure
erase, WinRE, PBA. It also exposes boot capabilities per device (what the hardware
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

**Gap:** No guardrails on power operations. Any authorized operator can power off
any host.

**DMT surface:** No policy layer — MPS executes whatever it receives.

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

**New work:** Policy resource definition, policy evaluation in the device
controller or CLI, metadata-based policy selectors.

---

### 7. Hardware Inventory Enrichment via AMT

**Gap:** EOM's hardware inventory comes from the OS level (HDA via ipmitool) and
AMT feature detection (PMA via rpc amtinfo). It does not query the full AMT
hardware information.

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
consent mode, etc.).

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

**Gap:** MPS supports transitioning devices between full power and OS power saving
modes (action codes 500/501), but EOM does not expose this capability.

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

**Gap:** Operators must know CLI syntax, filter expressions, and valid power state
values to manage the fleet. The learning curve is steep and error-prone.

**DMT surface:** Not applicable — DMT has no natural language interface.

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

**Gap:** When a host is unresponsive, an operator must manually check power state,
pull event logs, start a KVM session, and optionally boot to BIOS or diagnostic
mode — each as separate actions across different CLI commands or API calls.

**DMT surface:** MPS provides the individual pieces: power state query, event log
retrieval, KVM session start, boot to BIOS/diagnostic mode. But these are all
independent per-device API calls with no orchestration.

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

**New work:** Diagnostic workflow orchestration (CLI command or API operation),
integration with event log collection, conditional KVM/boot-mode triggers.

## Rationale

### Prioritization

| # | Opportunity | Category | Effort | Value | Key Argument |
|---|---|---|---|---|---|
| 1 | ✅ Bulk power operations | Augmentation | Low | High | CLI-only change, reconciliation exists |
| 2 | Scheduled power operations | Augmentation | Medium | High | Schedule framework exists, add power action |
| 3 | Event log monitoring | Gap + Augmentation | Medium | High | Proactive ops, data exists in AMT |
| 4 | Certificate lifecycle | Gap + Augmentation | Medium | High | Fleet cert expiry is a real operational pain |
| 5 | Boot orchestration | Gap + Augmentation | Medium | Medium | Enables reimaging/reprovisioning workflows |
| 6 | Power policies | Augmentation | Medium | Medium | Essential guardrail for bulk operations |
| 7 | Hardware inventory enrichment | Gap + Augmentation | Low | Medium | Data exists, collection is straightforward |
| 8 | AMT feature configuration | Gap + Augmentation | Medium | Medium | Consistency enforcement, drift detection |
| 9 | OS power saving state | Gap + Augmentation | Low | Low | Energy management, niche use case |
| 10 | Natural language via MCP | Augmentation | Medium | High | Eliminates CLI learning curve, composes with all other features |
| 11 | Automated remote diagnostics | Augmentation | Medium | High | Integrated workflow for unresponsive hosts, high demo value |

**Categories:**
- **EOM Gap** — DMT exposes a per-device capability that EOM does not surface.
- **DMT Augmentation** — value that goes beyond what DMT provides at any scale
  (fleet targeting, scheduling, reconciliation, alerting).

Most ideas are both: an underlying gap (DMT capability not surfaced) plus
augmentation (fleet-scale automation on top). Ideas #1, #2, #6, #10, and #11 are
pure augmentation — DMT doesn't have those capabilities at any scale either.

The strongest opportunities are those where **the data or capability already exists
in AMT firmware, DMT exposes it per-device, and EOM adds fleet-scale automation**
that would be impractical to replicate manually. Certificate lifecycle, scheduled
power operations, and event log monitoring are the clearest examples of
indisputable value — they solve problems that literally cannot be solved by calling
DMT APIs one device at a time.

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
new protocol client libraries, and dilute the vPro value proposition. Rejected
as this is outside the scope of EOM.

## Implementation Plan

To be determined per opportunity. Bulk power operations (#1) is already
implemented for 2026.1.

## Open Issues

- Event log monitoring (#3) depends on validating Loki push integration from
  within the DM-Manager or a standalone collector. Need to confirm Loki is
  accessible from the orch-infra namespace.
- Certificate lifecycle (#4) requires understanding how AMT cert expiry dates
  are exposed through the MPS certificate API — specifically whether expiry is
  returned in the existing GET response or requires parsing the cert blob.
- Power policies (#6) need design input on whether enforcement belongs in the
  device controller (server-side) or the CLI (client-side). Server-side is
  stronger but higher effort.
- Natural language via MCP (#10) requires deciding which LLM provider to
  integrate with and how to handle authentication for the MCP server.
