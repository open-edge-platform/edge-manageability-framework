# Design Proposal: Installer Simplification

Author(s): Scott Baker

Last updated: 2025-09-08

Revision: 2

## Abstract

This document provides a summary of the current EMF installer approach, highlighting key issues
posed by the approach due to the shift in consumption model from closed source to open source.
It also states the higher order goals to be met by the installation and covers recommended
approach to achieving the goals in a phased rollout manner.

## Changelog

- Revision 2

  - Combine phases 0 and 1, using the name "Workstream 1: Installer Simplification".

  - Phase 2 has been renamed "Workstream 2: Composable Installation".

  - Phase 3 has been renamed "Workstream 3: Argo-less Installation".

  - Added details on supported subsystem selections.

  - Add description of making multitenancy optional using tenant-controller jobs.

  - Added composability notes.

  - Remove deprecation of the AWS pre-installer.

- Revision 2.1

  - Added "Cleanup and migrate configuration tasks from pre-installer to the installer"

  - Add more details on the steps the installer takes, such as installing the ArgoCD helm
    chart and applying configuration to templates.

## Problem Statement

EMF currently is installed using a custom installer that provides full automation of installation
by bundling all the layers together such as provisioning infrastructure, installing and configuring
Kubernetes cluster and deploying EMF microservices. This approach was intended to provide a
platform-as-a-product experience to customers, which naturally led to a curated set of installers
targeting different infrastructure kinds. When EMF was distributed through GSI partners, this approach
had served well. However, with the shift to open sourcing EMF, this approach has already led to and
will continue to lead to sprawl of installers and adoption issues due to the opinionated cluster
approach. Most customers would either already have or would like to have control over their Kubernetes
clusters. It’s critical to provide an installation approach that’s using standards and simplified for
end customer adoption, as well as contribution. To that end, the first goal below is defined to simplify
EMF installation.

Note: The following goals do not necessarily need to be tackled sequentially.
Aspects may be undertaken simultaneously.

### Goal #1 - Standardize and Simplify EMF Installation

The scope of the "installer" must be reduced to the functions necessary to install EMF on any suitable
Kubernetes infrastructure., such as cloud VMs, on-prem machines and managed Kubernetes clusters.
EMF must be installable on this Kubernetes cluster using standard tools and techniques such as
Helm, Kubernetes, ArgoCD, etc. The requirements must be well documented.

### Goal #2 – EMF is deployable as an Edge Application

EMF must be deployable as an edge application on Intel platforms.

Once installation is simplified and standardized, EMF becomes a regular Kubernetes application that can
run on Intel Edge Platforms if one decides to do so. However, the footprint of EMF is still substantial,
with production grade gitops using ArgoCD, Platform observability, multitenancy etc. We must provide
sufficient controls to reduce the footprint, enabling EMF to run on smaller environments, including typical
Intel Edge hardware.

This requires EMF footprint review and optimization including lightweight Kubernetes, tuning resources
requests/limits and skipping some components such as GitOps, Platform Observability and Multitenancy.

### Goal #3 – Simple, reliable, and lightweight upgrades

The upgrade process must become simpler, and its scope must be confined to upgrading the EMF software,
avoiding the complexity of managing and upgrading infrastructure that is outside the scope of EMF.

It must be feasible to perform upgrades not only between major releases, but to integrate patch releases
between major releases. It must be feasible to upgrade individual components as necessary and integrate
hot fixes. These capabilities are expected of modern Helm-based software.

## Current state of installers in EMF 3.1

There are three installers maintained today:

1. AWS installer which provisions a EKS cluster and deploys EMF

2. On-prem installer which provisions an RKE2 cluster and deploys EMF

3. A lightweight development installer which provisions a Kind clusters and deploys EMF

![Current EMF Installers](images/platform-installer-simplification-3-installers.png)

## Problems with the current approach

- Multiple installers – hard to maintain multiple installers each one catering to different infrastructure
  provider

- Infrastructure provisioning – automating customer infrastructure is not our responsibility. Customers
  might want to do this differently and take care of their infrastructure provisioning in public clouds
  and on-prem

- Cluster creation – same as “Infrastructure provisioning” in the previous point

- Shell script – Shell scripting is not the idiomatic approach in the cloud native application ecosystem to
  provision infrastructure, create clusters, and deploy applications. Scripts become brittle and make testing
  and debugging harder. Scripts also lack monitoring, error handling, and reliability.

- Multiple mechanisms to solve identical processes – There are a mix of Magefiles and Shell scripts for such
  actions as configuring a tenant. Sometimes within the Magefiles there are even multiple mechanisms for doing
  the same thing. Duplication leads to additional maintenance effort as well as unintentional divergence of behavior.

## Phased Approach

EMF installation problems listed above will be addressed in multiple workstreams as follows:

- Workstream 1: Installer Simplification

- Workstream 2: Composable Installation

- Workstream 3: Eliminate dependency on ArgoCD

All three Workstreams may be parallelized as necessary.

### Workstream 1: Installer Simplification

Installation is broken into two phases, the installer and the pre-installer. 

![Installer and Pre-Installer Split](images/platform-installer-simplification-split.png)

#### The Pre-Installer is separate from the Installer

The job of the pre-installer is to prepare the environment that the installer will run in. It accepts a set
of Pre-installer Configuration values, and does the following:

1. Creates a Kubernetes environment.

2. Creates any necessary assets to enable that environment (ALBs, NLBs, Databases, etc).

3. Creates or updates the Installer Configuration that may be passed to the Installer.

4. (optionally) Invokes the Installer to complete the installation.

There will be three pre-installers:

1. Development/Preview. Creates a kind-based environment that is not intended to be used in production.

2. OnPrem. Creates an OnPrem Kubernetes on an Ubuntu node.

3. AWS. Creates an EKS Kubernetes on AWS together with the necessary ALBs, NLBs, Aurora, etc.

![Pre-Installers Overview](images/platform-installer-simplification-3-preinstallers.png)

The most important contribution of this task is to document the inputs to the Installer and to break our
user-facing documentation into separate pre-install and install sections. This allows any partner to write
their own pre-installer, or to fork and customize our pre-installers for their production use.

#### The Installer handles configuring and installing the Orchestrator software

The Installer is based on ArgoCD. The installer requires two things to run:

1. A Kubernetes environment. The installer installs EMF into this environment. It also uses the environment to
   run the installer itself.

2. A set of Installer Configuration values, including the credentials to the Kubernetes environment. This may
   include database configuration, root/admin passwords, public IP addresses, repository URLs, etc.

The installer configuration is primarily composed of a set of service profiles and a cluster profile.
These profiles are inputs to the ArgoCD root app, which in turn configures the other applications.

The steps the installer shall take to invoke ArgoCD include:

- Performing any template operations on the cluster.yaml that are necessary to override settings for
  the installation.

- Install the ArgoCD helm chart.

- Install the ArgoCD applications (i.e. Root App, etc)

- At this point ArgoCD begins installing the orchestrator software.

#### Eliminate Gitea as a pre-installer dependency

The current behavior of cloning the EMF repo into a local gitea shall be eliminated. ArgoCD shall point to a
public github source.

- The open-edge-platform/edge-manageability-framework repository is one potential github source that may be
  used. Alternatively, our customers who wish to significantly customize the orchestrator installation in
  ways above and beyond what we support are free to fork the edge-manageability-framework repo and point
  the installer at their fork.

- Local customizations of the installation (i.e. the installer configuration mentioned in Phase 0) may be
  passed to the ArgoCD root app as a valuesObject parameter, overriding default values that are obtained
  from git.

- If we wish to support air-gapped installation, then we will instruct our customers to create a local gitea
  instance of their own withing their airgapped domain. We can provide documentation on how to do this.

- Gitea is still used as an app-orch dependency by Fleet. As such, when Gitea is removed as a pre-installer
  dependency, at the same time, it will be added as an app-orch dependency and managed for app-orch use by
  ArgoCD.

#### Migrate any remaining Helm/Kubernetes services from pre-installer to the installer

The 3.1 installers may contain some helm-based components, such as Postgres, or other services that were
installed prior to the invocation of ArgoCD. These should be moved from these pre-installers into the
installer and handled by ArgoCD. We should have a consistent mechanism for installing Helm charts.

Redundant components in can always be disabled. For example, if a cloud-based database such as Aurora is
used, then we will have a knob that disables installation of Postgres.

#### Cleanup and migrate configuration tasks from pre-installer to the installer

The pre-installer phase contained some tasks, such as applying environment variables to cluster.tpl
files to create a cluster profile yaml specific to the cluster being installed. In 3.1 this was done
using three different mechanisms. As part of installer simplification, we will converge on a single
mechanism for rendering this template, and move the template rendering from the pre-installer to
the installer.

The reason for moving this to the installer is to facilitate bring-your-own-kubernetes situations and
to simplify configuration of the installation.

Some properties of how this configuration shall be done:

1. Configurate shall use a bash script, for example configure-cluster.sh.

2. The bash script shall ingest environment variables. The environment variable names will be the
   same regardless of whether cloud, onprem, or coder installation is performed.

3. Features will default to `ENABLED` and will only be disabled by the presence of an environment
   variable that disables the feature. In the absence of any such environment variables, the
   maximal configuration is apply, i.e. the orchestrator is installed with the same feature set
   that it had in 3.1.

#### Ensure all pre-installers and the installer are noninteractive

The pre-installers and the installer should be fully configured from the configuration values that are passed
to them initially. There should be no interactive prompts given to the user and no pausing of the installer
to solicit additional input.

### Workstream 2 - Composable Installation

In the prior phase, we moved terraform recipes to the pre-installers only, but if for any reason terraform files
still exist within the installer, we will move them in this phase. We continue to remove shell scripts and replace
them with standard helm charts and Kubernetes jobs, all of which are exposed as ArgoCD apps. Pre-installers may
continue to be used internally or distributed as examples.

By the end of this phase, it shall be possible to invoke the installer directly, by the customer bringing their
own Kubernetes, without using a pre-installer.

The following need to be taken care to support this model

- All Kubernetes objects including namespaces, databases, secrets, configs, policies etc must be treated as
  application components. They must be created via Helm and Argo

- Expose proper helm value overrides for customers to provide configuration values as appropriate

#### Make subsystems easy to disable

Make sure the installer configuration allows the following to be individually disabled, to support reduced footprint
for customers who do not require all components. This will allow the following services and their pods to be disabled,
reducing the footprint of the platform:

- Observability

  - Telemetry pods

  - Grafana

  - Loki

  - Mimir

- Cluster Orchestration

  - cluster-connect-gateway

  - cluster-manager

  - intel-infra-provider

  - capi-core-provider

  - cluster-api-k3s-provider

- Application Orchestration

  - app-deployment

  - app-interconnect

  - app-orch-catalog

  - app-orch-tenant-controller

  - app-resource-manager

  - app-service-proxy

  - fleet

  - gitea

Only certain combinations of subsystems are supported. This document refers to these as
_subsystem selections_ (to avoid confusion with the word "configuration" which is generally
use to describe how the subsystems themselves are configured).
Supported configurations begin with EIM, and then progressively add CO and then AO.
Observability may be added to any configuration. This is the set of supported
subsystem selections:

Subsystem selections without observability:

- EIM

- EIM + CO

- EIM + CO + AO

Subsystem selections with observability:

- EIM + Observability

- EIM + CO + Observability

- EIM + CO + AO + Observability

Once a selection is chosen, it may not be changed during an upgrade. For example, if a customer chose to install
only EIM and CO in release 2025.3, then they are not allowed to add AO in 2026.1. If the customer wishes to change
the subsystem selection, then they would need to reinstall.

#### Update CLI to gracefully handle absent subsystems

If a subsystem is absent, the CLI must emit an intuitive error message if the user attempts to execute a command
that uses that subsystem.

#### Update GUI  to gracefully handle absent subsystems

If a subsystem is absent, the GUI must remove all pages and all navigation links associated with the the absent
subsystem. Additionally, the GUI must gracefully degrade any dashboard or other common pages where the missing
subsystem would have displayed data.

#### Add optional single-tenant initialization job

Add an optional Kubernetes job that initializes a single tenant in the tenancy model. This is intended to facilitate
“EMF Lite”. By implementing this as a Kubernetes Job rather than a script, initialization of single-tenant
configuration may be fully encapsulated within the Helm (and ArgoCD) layers, and avoid requiring any external scripts.

Note: There is an existing job that can be re-used and/or extended, as part of the infra-charts repository.

The various tenant-controllers will remain deployed, but will be left in an idle state once the single tenant has been created.

#### Make multitenancy fully optional

_Note: This option is only for customers who wish to commit to a single-tenant / single-project installation
of the orchestrator with no capability to create additional projects. This choice is permanent for the life
of the orchestrator._

Above, we introduced a job that creates a single-tenant. In a truly single-tenant / single-project configuration, there
is no need for the various tenant-controllers to exist. These long-running controllers would be replaced with one-time
jobs. This includes the following components:

- app-orch-tenant-controller

- cluster-manager (part of this service listens to tenant events)

- keycloak-tenant-controller

- observability-tenant-controller

- (eim) tenant-controller

These components cannot simply be excluded, because they perform functions that need to be executed, even in a single
tenant / single project scenario. For example, the app-orch-tenant-controller is needed to load extension deployment
packages. The cluster-manager is needed to load cluster templates.

It will be necessary to take each one of these tenant-controllers and convert it to a job that runs to completion
and exits after one project as been setup. This will require some changes to the tenant controllers:

- Add a command-line option to tell the tenant-controller to exit after a project has been successfully
  completed.
  
  Note: This may not be necessary for cluster-manager, if the tenant controller is part of cluster-
  manager, rather than a separate pod.

- Amend the helm chart to include a job. The same docker image for the tenant controller may be re-used, with
  the option passed to the tenant-controller to tell it to exit on project create completion.

- The appropriate conditionals would need to be added to the helm chart to launch the tenant-controller as
  either a long-running service or as a job depending on customer perference.

#### Validation

Composaibility exposes 6 new subsystem selections that must all be tested using automation. Recommended tests
include:

- Deploy using EIM only and onboard an edge node.

- Deploy using EIM and CO, onboard an edge node, and create a cluster.

- Deploy using EIM, CO, and AO, onboard an edge node, create a cluster, and deploy an application. This selection
  matches the existing VIP, HIP, and Golden Suite validation.

Testing each of the above in both of "with observability" and "without observability" configurations would double
the number of configurations to test from 3 to 6. It would be less resource and time intensive to just pick one
of the above and use it for the "with observability" test and a different one to use for the "without observability"
test.

### Workstream 3 – Argo-less Installation

Support lighter weight deployment by eliminating argocd components. Use techniques such as umbrella helm charts or tools
such as helmfile. We will move away from heavy weight services such as ArgoCD that are always running and consuming resources.
If the customer wishes to use a gitops tool (such as ArgoCD) to maintain their EMF installation, they are free to do so.
Installing EMF should be as simple as installing a helm chart.

#### ArgoCD Syncwave Investigation

ArgoCD uses syncwaves to ensure that some services are installed before other services. To our knowledge, this was largely
determined by observation rather than planning – people noticed that some services did not install properly unless prerequisites
were in place, and adjusted syncwaves to suit. These syncwaves are a cause for additional deployment time, because chart
deployment is delayed.

- We will document the dependencies between components, and justify why the syncwaves exist.

- We will seek alternative solutions to syncwaves, such as modifying components to be more resilient and retry when
  their dependencies are not ready.

- The ultimate goal is to eliminate syncwaves entirely, though this may not be feasible. For example, it is often necessary
  to install CRDs before a service that uses CRDs can be installed.

#### Eliminate ArgoCD

Once the syncwaves have been reduced or eliminated, then it is feasible to eliminate ArgoCD in favor of a simpler tool. We
will explore alternatives such as umbrella charts, the helmfile tool, or other opensource solutions. We may explore repo
and/or chart consolidation to make the helm chart structure simpler.

Eliminating argocd will allow the following pods to be eliminated from the platform:

- argocd-application-controller
- argocd-applicationset-controller
- argocd-notifications-controller
- argocd-redis
- argocd-repo-server
- argocd-server

## Rationale

The rationale for simplification is that it is infeasible to continue to maintain three independent monolithic
installers. Convergence is necessary, with a focus on installing EMF itself rather than provisioning infrastructure,
which differs on a customer-by-customer basis.

## Affected components and Teams

The installer and the team working on the installer will be affected for all workstreams. CLI and GUI teams will
be affected to update the CLI and GUI to tolerate missing subsystems as part of the composability deliverable.

## Implementation plan and Implementation notes

The implementation shall be carried out in the phases outlined in the "Phased Approach" section above.

### Composability Notes

It was determined that AO, CO, and O11y can easily be turned off by excluding their particular `enable-`
profiles from the cluster profile. This yielded an installation with the pods and services for the
excluded subsystems absent, as expected. It was determined that there were two issues with the user
interface:

- The GUI displayed error boxes on pages that were tied to their respective substystems, as well as
  common pages. For example, disabling AO would lead to broken pages for Deployments, Deployment Packages,
  and Applications and also issues with the common dashboard page. Disabling CO has the additional
  impact of affecting the node registration page where cluster templates may be selected.

- The CLI displayed error messages when functionality was unavailable. The error messages did not
  clearly indicate that a subsystem was disabled, for example sometimes only a "500 internal error"
  was displayed.

## Decision

Workstreams 1 and 2 are committed and will be delivered as part of the December release. Workstream 2 may
be delivered in a CLI-only manner as necessary. Workstream 3 is deferred until a future date.

## Open issues (if applicable)

None.
