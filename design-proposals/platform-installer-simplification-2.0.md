# Design Proposal: Installer Simplification 2.0

Author(s): TBD

Last updated: TBD

Revision: 1.0

## Abstract

This ADR describes installer simplification steps to be taken for 2026.1.
This ADR supersedes [platform-installer-simplification.md](platform-installer-simplification.md)

## Changelog

- Revision 1.0
  - TBD

## Problem Statement

EMF installers remain a source of unintended complexity due to these installers being crafted
for different purposes at different times. The primary goal remains to converge on a single
post-installer. A secondary goal has been added that is to remove ArgoCD from the installer.

## Current State

The current state is that EMF has the following:

* AWS Installer

* OnPrem Installer (old)

* OnPrem Installer (new, supporting K3s/Kind/RKE2)

* Coder Installer

## Implementation Plan

### Workstream 1 - Eliminate Multiple Installers

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

### Workstream 1: Deliver a Simplified, Repeatable Installation Process

The following scripts shall become the one and only installer.

#### DNS configuration

DNS configuration is a prerequisite for installation, as there are several domain names
that must point to the orchestrator.

In production scenarios, the customer is responsible for DNS configuration and will usually
use their existing infrastructure.

An example dnsmasq configuration shall be provided in the deployment guide.

#### Pre-installer

[/on-prem-installers/onprem/pre-orch-install.sh](/on-prem-installers/onprem/pre-orch-install.sh)

This script accepts three options to configure installation of reference implementations
for K3s, Kind, or RKE2. It is provided as a convenience for customers and for validation,
to establish a repeatable process for creating Kubernetes environments.

It is not intended to serve as a production-quality Kubernetes deployment. Customers
wishing to perform a production installation of EMF should leverage their internal IT
support to create a hardened Kubernetes environment per their requirements.

#### Post-installer

[/on-prem-installers/onprem/post-orch-install.sh](/on-prem-installers/onprem/post-orch-install.sh)

This script creates the cluster.yaml file necessary to configure ArgoCD, sets up any
namespaces and secrets required for ArgoCD, installs ArgoCD, and bootstraps the installation
by installing the ArgoCD root application.

### Migrate Coder Deployments to use the OnPrem Installer

Coder deployments should use the same pre-installer and post-installer as described above. The
goal is to eliminate unnecessary divergence.

**Open question:** Should Coder deployments use Kind, K3s, or RKE2?

Additional steps may be required for Coder deployments. For example, the auto-cert
functionality enables Coder-based orchestrators to be compatible with physical edge nodes.
These integrations will need to be re-established with the new on-prem-based installer.

### Migrate VIP to use pre-installer / post-installer

VIP will have to be migrated to use the new pre-installer and post-installer.

### Migrate HIP to use pre-installer / post-installer

HIP will have to be migrated to use the new pre-installer and post-installer.

AWS-based HIP will be dropped when the AWS installer is dropped.

## Workstream 2: Deliver an ArgoCD-less Installation Experience

This workstream modifies the post-installer so that installation can be performed without
ArgoCD.

A complicating factor is that many Helm charts comprise even a simple EMF deployment
with the vPro profile. These charts must be sequenced in a specific order, and
configuration values must be propagated across multiple charts and services.

There are a few possible options:

### Helmfile

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

### Plain Helm Charts

These may need synchronization with `--wait`, and a solution would need to be found
for configuring them.

## Open Issues

TBD

## Decision

TBD
