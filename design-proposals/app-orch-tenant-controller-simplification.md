# Design Proposal: App Orch Tenant Controller Simplification

Author(s): Scott Baker

Last updated: 2025-09-24

## Abstract

This ADR describes a proposed simplification to App Orch Tenant Controller to improve speed and reliability
of creating new projects.

## Problem Statement

App-Orch-Tenant-Controller is a component that is used to bootstrap new projects. When a new project is created,
App-Orch-Tenant-Controller populates AO assets in the project, such as harbor projects, default registries, etc.
One of the things it is responsible for is populating the initial set of deployment packages. These deployment
packages typically contain cluster extensions, and there is about a dozen of them for various features including
examples such as cert-manager, intel-gpu, virtualization, etc.

App-Orch-Tenant-Controller currently downloads a cluster manifest and deployment packages from the release service.
The Cluster Manifest is versioned and names the set of Deployment Packages to download.
The App-Orch-Tenant-Controller Helm chart tells which version of the Cluster Manifest to download.
This version is typically overridden in ArgoCD.
Each deployment package is downloaded as a separate "ORAS LOAD" operation. This has three negative properties:

1. Sometimes the release service is slow, and this has led to slow project creation. Connections have been
   known to stall, and this led to filing multiple bugs on project creation, both internally and from
   external customers.

2. Reliance on the Intel Release Service creates an unnecessary obstacle for customers to customize the
   set of packages that are installed to new projects.

3. There is no real advantage to using the Release Service to store these deployment packages now that
   EMF is now open-source. There are much simpler alternatives.

## Proposal 1: Specify Deployment Packages directly inside the App-Orch-Tenant-Controller Helm Chart

In this proposal, we cease fetching the cluster-manifest and the deployment packages from the
Release Service. We eliminate the Cluster-Manifest entirely.

The App-Orch-Tenant-Controller Helm Chart would be modified to contain a list of deployment packages
to load for new projects. This would be a list inside the values.yaml file. That list would be
realized as a config map, which is then loaded by App-Orch-Tenant-Controller.

This is similar to the solution that Cluster Orchestration uses to load cluster templates.

A disadvantage of this approach is that any change to Cluster-Extensions must be followed by
a change to App-Orch-Tenant-Controller. The two repositories must be kept in sync. If Custer-Extensions was
updated without updating App-Orch-Tenant-Controller, then in the best case new extensions would be ignored.
In the worst case, missing assets could be fetched, leading to project creation failure. Dependencies like
this often lead to the developer cycling back and forth between repos, having to commit one repo before the
other repo can be tested.

## Proposal 2: Specify packages using a Helm Chart in cluster extensions

This is similar to Proposal 1, except that rather than incorporating the list of deployment packages
directly into the app-orch-tenant-controller helm chart, a second helm chart is created in
cluster-extensions that holds the configmap that contains the list of deployment packages.

This approach allows a separation between code and data. The "code" remains in the app-orch-tenant-controller
helm chart whereas the "data" is present in the cluster-extensions helm chart. Updating a new extension
only requires generating the cluster-extensions helm chart, which contains only a set of yaml files.

The advantage of this approach is that changes to Cluster-Extensions do not need to be followed by
changes to App-Orch-Tenant-Controller.

## Rationale

Tentative decision is in favor of Proposal 2. While the initial implementation may be slightly more
complicated, there is significant advantage in maintenance in that new extension releases can be made
without needing to alter the app-orch-tenant-controller helm chart.

## Affected components and Teams

- App Orch Tenant Controller

- Cluster Extensions

## Implementation plan

The following stories must be completed:

1. Create a cluster-extensions helm chart in the cluster-extensions repo, and populate the helm chart with a
   configmap that contains the list of deployment packages.

2. Update app-orch-tenant-controller to use the configmap rather than the release service.

3. Implement logic in app-orch-tenant-controller to detect the configmap change and process updates to the
   extensions. This may be as simple as detecting a configmap change and then restarting the component.

## Decision

Decision is to hold until necessary and/or when time permits to address tech debt.

## Open issues (if applicable)
