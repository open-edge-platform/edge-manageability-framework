# Design Proposal: Installer Simplification 2.0

Author(s): Scott Baker

Last updated: 2026-3-17

Revision: 2.0

## Abstract

This ADR describes installer simplification steps to be taken for 2026.1.
This ADR supersedes [platform-installer-simplification.md](platform-installer-simplification.md)

## Changelog

- Revision 2.0
  - TBD

## Problem Statement

EMF installers remain a source of unintended complexity due to being crafted for different purposes
at different times. The primary goal is to converge on a single unified installer pattern. A secondary
goal is to remove ArgoCD from the mandatory installer architecture, simplifying the deployment path
for customers who bring their own Kubernetes clusters.

## Goals

### Goal #1: Eliminate Installer Sprawl

Reduce the number of distinct installer implementations from four (AWS, OnPrem old, OnPrem new, Coder)
to one canonical pattern that works across all deployment scenarios.

### Goal #2: Remove ArgoCD as a Mandatory Orchestration Layer

Enable EMF deployment without requiring ArgoCD as a continuous reconciliation component, while
preserving the ability for customers to add ArgoCD for GitOps workflows if desired.

### Goal #3: Support "Bring Your Own Kubernetes"

Simplify installation for customers who have already provisioned hardened Kubernetes clusters
and wish to deploy only EMF software components, without infrastructure provisioning or cluster creation.

### Goal #4: Maintain Configurability and Composability

Preserve the ability to selectively enable/disable services (e.g., PostgreSQL, MetalLB, Observability)
to support varied deployment footprints from minimal edge nodes to full platform deployments.

## Scope for 2026.1

This proposal encompasses **Workstream 1 and Workstream 2** in full.

## Current State

The current EMF installation landscape includes:

- **AWS Installer**: Provisions EKS cluster and deploys EMF (to be removed)
- **OnPrem Installer (legacy)**: Shell-script based, tightly coupled to specific configurations
- **OnPrem Installer (current)**: Supports K3s, Kind, and RKE2 with improved flexibility
- **Coder Installer**: Deployment-specific variant for Coder environments

Each installer has divergent logic, configuration mechanisms, and maintenance burdens.

## Success Criteria

- Single installer pattern documented and functional across all deployment scenarios
- AWS, legacy OnPrem, and Coder installers deprecated (can be maintained externally if needed)
- Post-installer successfully deploys EMF without ArgoCD as a required component
- Configuration values are centralized and reusable across all deployment paths
- Customers can deploy EMF to pre-existing Kubernetes clusters with minimal friction

## Implementation Plan

### Workstream 1: Deliver a Simplified, Repeatable Installation Process

This workstream converges on a single post-installer without removing the dependency
on ArgoCD.

#### Remove AWS Installer

The AWS installer will be immediately removed. Customers wishing to perform AWS
installations shall maintain a fork of the AWS installer themselves. This eliminates one source
of divergent installers.

#### Eliminate Sources of Redundancy and Confusion

For example, the on-prem-installers/onprem directory contains scripts `pre-orch-install.sh`
and `onprem_pre_install.sh`. This is confusing as both scripts appear to serve the same
purpose.

Each file in the on-prem-installers/ and installer/ directories must be justified.
If a file does not serve a purpose, it shall be removed.

#### Document and provide example DNS configuration

DNS configuration is a prerequisite for installation, as there are several domain names
that must point to the orchestrator.

In production scenarios, the customer is responsible for DNS configuration and will usually
use their existing infrastructure.

An example dnsmasq configuration shall be provided in the deployment guide.

#### Standardize Pre-installer

[/on-prem-installers/onprem/pre-orch-install.sh](/on-prem-installers/onprem/pre-orch-install.sh)

This script accepts three options to configure installation of reference implementations
for K3s, Kind, or RKE2. It is provided as a convenience for customers and for validation,
to establish a repeatable process for creating Kubernetes environments.

It is not intended to serve as a production-quality Kubernetes deployment. Customers
wishing to perform a production installation of EMF should leverage their internal IT
support to create a hardened Kubernetes environment per their requirements.

#### Standardize Post-installer

[/on-prem-installers/onprem/post-orch-install.sh](/on-prem-installers/onprem/post-orch-install.sh)

**Note:** Workstream 2 transitions the post-installer to use Helmfile instead
of ArgoCD for orchestration. Workstream 1 keeps this script as-is until it is
ready to be replaced. This script does the following:

- Create the cluster.yaml configuration file with environment-specific values
- Set up necessary namespaces and secrets
- Installs ArgoCD
- Installs Gitea (if required for app-orch)
- Invoke ArgoCD to deploy all EMF Helm charts in correct dependency order

#### Migrate Coder Deployments to use the pre-installer / post-installer described above

Coder deployments should use the same pre-installer and post-installer as described above. The
goal is to eliminate unnecessary divergence.

**Open question:** Should Coder deployments use Kind, K3s, or RKE2?

Additional steps may be required for Coder deployments. For example, the auto-cert
functionality enables Coder-based orchestrators to be compatible with physical edge nodes.
These integrations will need to be re-established with the new on-prem-based installer.

#### Migrate VIP to use pre-installer / post-installer

VIP will have to be migrated to use the new pre-installer and post-installer.

#### Migrate HIP to use pre-installer / post-installer

HIP will have to be migrated to use the new pre-installer and post-installer.

AWS-based HIP will be dropped when the AWS installer is dropped.

### Workstream 2: Deliver an ArgoCD-less Installation Experience

This workstream modifies the post-installer so that installation can be performed without
ArgoCD.

A complicating factor is that many Helm charts comprise even a simple EMF deployment
with the vPro profile. These charts must be sequenced in a specific order, and
configuration values must be propagated across multiple charts and services.

The ability to disable services such as MetalLB and Postgres must be preserved, as
the customer may bring their own replacements for those services.

There are a few possible options:

#### Helmfile

Helmfile is a declarative YAML-based tool that manages multiple Helm chart deployments
and their dependencies. It allows defining charts, values overrides, repository sources,
and execution order in a single `helmfile.yaml` configuration file. When executed via
`helmfile sync`, it applies all charts in the correct dependency order, replacing the
need for scripted helm install commands or ArgoCD as an orchestration layer.

**Advantages for EMF installer simplification:**

- **Declarative configuration:** All chart deployments and their values are defined in a
  single, human-readable file that can be version-controlled and reviewed as part of the
  installer configuration.

- **Dependency management:** Helmfile can express chart dependencies, ensuring correct
  installation sequencing without manual scripting.

- **Value propagation:** Configuration values can be easily shared across multiple charts
  using templating, reducing duplication and configuration errors.

- **Lightweight:** Helmfile does not require a control plane or reconciliation loop like
  ArgoCD. The post-installer script can simply invoke `helmfile sync` and complete when
  all charts are deployed.

- **Easy to debug and troubleshoot:** Direct helm invocations retain their transparency,
  and the helmfile configuration is straightforward to audit and modify.

- **Existing ecosystem:** Helmfile is already in use in the buildall workflow
  ([buildall/Makefile](../buildall/Makefile)). Reusing it in the installer provides
  consistency across development and deployment paths.

**Implementation approach:**

The post-installer would generate or use a helmfile configuration that
references all necessary EMF charts, applies cluster-specific values overrides (hostname,
domain, credentials, etc.), and then invoke `helmfile sync` to deploy them. This approach
simplifies the installer architecture while maintaining the ability to control chart
ordering and configuration propagation without a runtime reconciliation component.

#### Plain Helm Charts

**Tradeoffs:**

- **Advantages:** Minimal dependencies, familiar to Kubernetes-native operators
- **Disadvantages:** Requires manual sequencing logic in shell scripts,
  error-prone configuration propagation, and difficult-to-maintain
  ordering constraints

**Recommendation:** This approach is **not recommended** as the primary path. It introduces
manual orchestration complexity that Helmfile handles automatically. Reserve this for specific
components that do not fit the Helmfile model.

However, if the vPro-profile could be simplified significantly (see the separate ADR on
simplifying this profile) then the plain helm chart approach may become viable.

## Migration Strategy

### Transition from ArgoCD-based Deployments

- The old installers will be deprecated and the new installers made available in 2026.1
- There is no upgrade path

## Affected Components and Teams

Platform Team.

## Open Issues and Questions

- **Helmfile vs. Kustomize:** Should we consider Kustomize as an
  alternative for configuration management?
- **Coder deployment selection:** Which Kubernetes distribution (Kind, K3s, RKE2) should be the default for Coder?

## Rationale and Alternatives

### Why Helmfile?

Helmfile was selected over alternative approaches because it:

1. **Reduces operational complexity** compared to scripted shell-based helm invocations
2. **Manages sequencing and dependencies** declaratively, eliminating custom ordering logic
3. **Is already established** in the EMF build ecosystem (buildall)
4. **Maintains transparency** - all chart invocations remain visible and debuggable
5. **Supports configuration propagation** through templating without introducing a reconciliation control plane

### Why not Plain Helm + Scripts?

Plain Helm with shell scripts would require:

- Custom dependency management and sequencing logic
- Error-prone configuration variable propagation
- Significantly higher testing and maintenance burden
- Reduced auditability of deployment state

The verbosity and orchestration overhead outweigh any simplicity gains.

### Why not Kustomize?

Kustomize is better suited for template composition than for orchestrating multiple related deployments.
It does not natively handle installation ordering or complex inter-chart configuration propagation.

## Decision

We adopt the following strategy for Installer Simplification 2.0, effective 2026.1:

1. **Helmfile is the primary post-installer orchestration mechanism** for
  "bring your own Kubernetes" and reference implementations
2. **Pre-installers remain independent,** supporting K3s, Kind, and RKE2 for reference environments
3. **AWS and legacy OnPrem installers are deprecated** - customers may fork and maintain externally
4. **Coder deployments will consolidate** onto the OnPrem installer pattern with Coder-specific configuration overlays
5. **All configuration** will be centralized in helmfile.yaml and
  cluster.yaml with clear documentation of all environment variables and
  overrides
