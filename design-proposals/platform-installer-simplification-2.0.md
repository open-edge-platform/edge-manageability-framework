# Design Proposal: Installer Simplification 2.0

Author(s): Scott Baker

Last updated: 2026-3-30

Revision: 2.1

## Abstract

This ADR describes installer simplification steps to be taken for 2026.1.
This ADR supersedes [platform-installer-simplification.md](platform-installer-simplification.md)

## Changelog

- Revision 2.0
  - Draft of 2026 ADR
- Revision 2.1
  - Added section on autocert and DNS
  - Added section on Helmfile rollback
  - Added discussion of umbrella versus separate helm charts
  - Removed open questions

## Problem Statement

EMF installers remain a source of unintended complexity due to being crafted for different purposes
at different times. The primary goal is to converge on a single unified installer pattern. A secondary
goal is to remove ArgoCD from the mandatory installer architecture, simplifying the deployment path
for customers who bring their own Kubernetes clusters.

## Goals

### Goal #1: Eliminate Installer Sprawl

Reduce the number of distinct installer implementations from four (AWS, OnPrem legacy, OnPrem any-kubernetes, Coder)
to one canonical pattern that works across all deployment scenarios.

### Goal #2: Support "Bring Your Own Kubernetes"

Simplify installation for customers who have already provisioned hardened Kubernetes clusters
and wish to deploy only EMF software components, without infrastructure provisioning or cluster creation.

### Goal #3: Maintain Configurability and Composability

Preserve the ability to selectively enable/disable services (e.g., PostgreSQL, MetalLB, Observability)
to support varied deployment footprints from minimal edge nodes to full platform deployments.

### Goal #4: Remove ArgoCD as a Mandatory Orchestration Layer

Enable EMF deployment without requiring ArgoCD as a continuous reconciliation component, while
preserving the ability for customers to add ArgoCD for GitOps workflows if desired.

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
- AWS, legacy OnPrem, and Coder installers deprecated.
  Users or community can elect to maintain these in a fork or branch as needed.
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
ready to be replaced.

The post-installer does the following:

- Create the cluster.yaml configuration file with environment-specific values
- Set up necessary namespaces and secrets
- Installs ArgoCD
- Installs Gitea (if required for app-orch)
- Invoke ArgoCD to deploy all EMF Helm charts in correct dependency order

#### Migrate Coder Deployments to use the pre-installer / post-installer described above

Coder deployments should use the same pre-installer and post-installer as described above. The
goal is to eliminate unnecessary divergence.

Should Coder deployments use Kind, K3s, or RKE2?

- Kind - Kind offers easy cluster setup and teardown, and supports running multiple
  clusters simultaneously, making it attractive for development. However, Kind diverges
  from production deployments, which are more likely to use K3s or RKE2. This divergence
  means bugs found in Kind may not reproduce in production, and vice versa. There are
  also operational complications, such as the need to push images from the host into the
  Kind environment.

- K3s - K3s is lightweight and closely mirrors the expected production deployment path,
  reducing the risk of environment-specific issues surfacing late. It is smaller and
  easier to install than full Kubernetes, while still providing a representative runtime
  environment.

- RKE2 - RKE2 is a larger Kubernetes distribution with additional security hardening
  (SELinux defaults, CIS benchmarks) and longer startup times. These features are
  unnecessary for a development environment and actively slow down developer iteration
  cycles.

This ADR suggests that the best Kubernetes for Coder development would be K3s due to the
closer alignment with actual deployment scenarios.

**DNS and autocert considerations for K3s migration:**

Coder deployments require two features beyond basic EMF installation: DNS record
creation for service subdomains and automatic TLS certificate provisioning (autocert).

EMF exposes 40+ subdomains (api, mps, web-ui, keycloak, vault, etc.) that must
resolve to the orchestrator. The current Kind-based approach handles this in three
layers:

1. **Route53** creates a top-level record mapping `orch-<ip>.espdqa.infra-host.com`
   to the host IP during Coder workspace provisioning.
2. **A host-side Traefik router** (running in Docker alongside Kind) reads the
   subdomain list from a `kubernetes-docker-internal` ConfigMap and generates
   per-subdomain routing rules.
3. **CoreDNS inside Kind** is configured with a static zone file template
   (`node/kind/coredns-config-map.template`) mapping each subdomain to the
   orchestrator IP.

Moving to K3s changes the DNS and routing approach. K3s exposes services directly
on the host network rather than through Docker networking, so the host-side Traefik
router used by Kind is no longer necessary. The in-cluster ingress controller
(Traefik or HAProxy) handles subdomain routing natively based on hostname. CoreDNS
configuration also differs — K3s manages its own CoreDNS instance, so the Kind zone
file injection must be adapted to K3s's configuration model.

A simplification opportunity exists: a **wildcard DNS record** in Route53
(`*.orch-<ip>.espdqa.infra-host.com`) would eliminate the need to maintain individual
subdomain records at the DNS layer, since the in-cluster ingress already routes by
hostname. This would replace both the CoreDNS zone template and the per-subdomain
Route53 records with a single wildcard entry.

Autocert uses cert-manager with ACME DNS01 validation against Route53 to obtain TLS
certificates from Let's Encrypt. The autocert components — cert-manager,
platform-autocert, cert-synchronizer, and botkube — are Kubernetes-native and require
no changes for K3s. The one area where K3s simplifies the current approach is
certificate persistence. Kind clusters are ephemeral, so the current implementation
saves certificates to AWS Secrets Manager before cluster teardown and restores them
after recreation. K3s clusters persist across reboots, making this save/restore cycle
unnecessary for routine operations. The mechanism should be retained for cases where
a cluster is fully destroyed, but this becomes exceptional rather than routine.

**IP address considerations when using K3s on Coder VMs:**

The on-prem installer requires three distinct IP addresses (`ARGO_IP`,
`TRAEFIK_IP`, `HAPROXY_IP`) for MetalLB to assign to the ArgoCD, Traefik, and
HAProxy LoadBalancer services respectively. Each service listens on port 443, so
they cannot share an IP. The current Kind-based Coder deployment avoids this
problem because MetalLB allocates virtual IPs from the Docker network range. A
Coder VM running K3s has only a single host IP. Possible approaches:

1. **Consolidate to a single ingress controller.** Route all traffic — including
   ArgoCD and Tinkerbell/HAProxy — through Traefik based on hostname/SNI. This
   eliminates the need for three separate LoadBalancer services and three IPs.
   The on-prem installer currently separates them for production fault isolation,
   but this is unnecessary for a Coder dev environment. This is the recommended
   approach as it aligns with the ADR's simplification goals. Note: The reason
   for having a separate HAProxy is due to reduced TLS algorithms being available
   when remote booting due to BIOS limitations. **This approach is not viable
   for that reason**.

2. **Virtual IPs on a dummy interface.** Add secondary IPs to the loopback or a
   dummy interface (e.g., `ip addr add 10.0.0.51/32 dev lo`). MetalLB advertises
   these in L2 mode. They are routable on the local machine but not externally,
   which is acceptable for Coder VMs where external access comes through the
   Coder proxy.

3. **MetalLB L2 mode with the host IP on different ports.** Assign the host IP
   to all three pools. Traefik keeps port 443 since it handles the majority of
   traffic. ArgoCD and HAProxy/Tinkerbell would be exposed on non-standard ports,
   requiring configuration changes for anything that connects to those services.
   Moving ArgoCD would not be difficult. Moving HAProxy/Tinkerbell would require
   documentation updates.

4. **K3s built-in ServiceLB.** K3s ships with a simple LoadBalancer implementation
   (formerly Klipper) that binds services directly to the host's network
   interfaces using different ports. However, `pre-orch-install.sh` currently
   disables built-in K3s components and installs MetalLB, so this would require
   changing that approach. Has the same complications with port sharting that
   approach #3 does.

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

**Failure recovery and rollback:**

Unlike ArgoCD, Helmfile does not provide continuous reconciliation. If `helmfile sync`
fails partway through a deployment or upgrade, recovery depends on the nature of the
failure:

- **Re-run `helmfile sync`.** Helmfile and Helm are idempotent — re-running `helmfile sync`
  will skip releases that are already at the desired state and retry those that failed.
  This is the expected recovery path for transient failures (e.g., image pull errors,
  resource quota limits, temporary network issues).

- **Rollback individual releases.** If a specific chart upgrade introduces a breaking change,
  `helm rollback <release> <revision>` can revert that release to a previous revision
  while leaving other releases untouched. Helm retains release history by default,
  making per-release rollback straightforward.

- **Full rollback.** For a failed upgrade where multiple releases need to be reverted,
  `helmfile sync` can be re-run against the previous version's helmfile configuration.
  Because each release tracks its own revision history, this converges the cluster back
  to the prior known-good state.

- **Database backup/restore.** For upgrades that include schema migrations or data changes,
  the database backup taken before the upgrade (see Upgrades section) provides the
  recovery path if rollback alone is insufficient.

The key operational difference from ArgoCD is that recovery requires explicit operator
action. ArgoCD would automatically reconcile drift; with Helmfile, the operator must
re-run `helmfile sync` or `helm rollback` to recover. This is an acceptable tradeoff
for the simplicity gained by removing the reconciliation control plane, but it must be
documented in the deployment guide with clear runbook procedures.

#### Umbrella Helm Charts and/or Separate Helm Charts with Script

An alternative to Helmfile is to use Helm directly, either by consolidating charts
into umbrella charts, by scripting individual `helm install` commands, or a combination
of both.

**Umbrella Helm Charts:**

An umbrella chart is a parent Helm chart that declares other charts as dependencies.
Instead of installing 40+ charts individually, related charts are grouped into a small
number of umbrella charts (e.g., `emf-platform`, `emf-services`, `emf-ui`) that are
installed as units.

- **Advantages:** Reduces the number of install commands to a handful of umbrella
  releases. Helm manages sub-chart ordering within each umbrella. Configuration values
  can be shared across sub-charts through the parent chart's values file. The customer
  installs a small number of well-defined packages rather than dozens of individual
  charts.

- **Disadvantages:** EMF currently deploys 106 applications across 28 sync wave levels,
  drawing from both internal charts and 18 external chart repositories. Packaging these
  into umbrella charts requires wrapper charts around external dependencies and careful
  management of sub-chart versioning. Upgrading a single component requires releasing a
  new version of the entire umbrella. The conditional enable/disable of services (e.g.,
  PostgreSQL, MetalLB) must be handled through sub-chart conditionals, adding complexity
  to the umbrella chart's values schema. Debugging failures is harder because Helm
  treats the umbrella as a single release — a failure in one sub-chart fails the entire
  umbrella install.

**Separate Helm Charts with Script:**

Each chart is installed individually via scripted `helm install` / `helm upgrade`
commands, with a shell script managing the sequencing and configuration propagation.

- **Advantages:** Minimal dependencies beyond Helm itself. Familiar to Kubernetes-native
  operators. Each chart is an independent release, making targeted upgrades and rollbacks
  straightforward.

- **Disadvantages:** Requires custom dependency management and sequencing logic in shell
  scripts — currently 28 ordering levels that must be encoded manually. Configuration
  values must be propagated across charts through script variables or generated values
  files, which is error-prone. The script becomes the single point of complexity and
  must be maintained alongside the charts themselves.

**Hybrid approach:**

A combination is possible: group tightly coupled charts into a small number of umbrella
charts to reduce the total chart count, then use a script to install the umbrellas in
the correct order. This reduces scripting complexity while keeping umbrella charts
manageable in size.

**Recommendation:** These approaches are **not recommended** as the primary path. Helmfile
provides the benefits of both — declarative sequencing like an umbrella chart, with
per-chart independence like a scripted approach — without the downsides of either.

However, if the vPro profile could be simplified significantly (see the separate ADR on
simplifying this profile), reducing the chart count and dependency complexity, then a
plain Helm approach (umbrella or scripted) may become viable as an alternative.

## Upgrades

The following scenarios are explicitly not supported:

- **Upgrade from a version prior to 2026.1.** The installation process has diverged too much for this
  to be considered a supported feature.

- **Update of Kubernetes.** For example, upgrading from one K3s to another. Kubernetes is considered
  customer infrastructure and outside the scope of this ADR.

- **Switching Kubernetes distributions.** For example, switching from Kind to K3s, or K3s to RKE2.
  These would be disruptive to the software installed on Kubernetes, and would require a full
  reinstallation.

- **Software that is installed by the pre-installer.** In general, software that is installed by
  the pre-installer is *not* "part of EMF". The pre-installer is provided as
  a convenience only and is expected to be hardened by the customer. Upgrade of components
  in the pre-installer is a customer responsibility. Exceptions may be made as necessary.

Upgrade from 2026.1 to subsequent versions must be supported, and must be addressed in both
workstreams described above. This includes:

- **Upgrade of all software installed by the post-installer.** In general, all software installed
  by the post-installer is considered "part of EMF", even if this software is a third party
  dependency.

- **Backup/Restore of databases.** For resilience in case of upgrade failure, as well as a potential
  mechanism to deal with major upgrades to postgres.

## Affected Components and Teams

Platform Team.

## Open Issues and Questions

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

## Implementation Details

A separate ADR will be written to contain implementation details for the Helmfile transition.
See [PR #1656](https://github.com/open-edge-platform/edge-manageability-framework/pull/1656) for details.
