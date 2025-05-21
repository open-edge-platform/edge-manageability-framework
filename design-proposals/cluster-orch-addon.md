# Design Proposal: Simplified Default Addon

Author(s): Hyunsun Moon, Madalina Lazar

Last updated: 05/21/2025

## Abstract

This proposal aims to streamline the default addons installed in Kubernetes edge environments by retaining only essential components. The document also details planned changes for managing default addons in upcoming releases.

## Proposal

### Default Addons for 3.1 Release
The table below outlines the addons included in the 3.0 release and the planned changes for the 3.1 release, along with relevant notes:

| Addon                      | Default in 3.0 | Default in 3.1 (EMF-Managed) | Default in 3.1 (EMT-S) | Notes                                                                   |
| -------------------------- | -------------- | ---------------------------- | ---------------------- | ----------------------------------------------------------------------- |
| gatekeeper and constraints | Y              | N                            | N                      | Replace with Kubernetes Pod Security Admission Controller and Standards |
| cert-manager               | Y              | N                            | N                      | Move to optional observability package                                  |
| telegraf                   | Y              | N                            | N                      | Move to optional observability package                                  |
| prometheus stack           | Y              | N                            | N                      | Move to optional observability package                                  |
| node-exporter              | Y              | N                            | N                      | Move to optional observability package                                  |
| openebs                    | Y              | N                            | N                      | Move to optional package                                                |
| fluent-bit                 | Y              | N                            | N                      | Move to optional package                                                |
| nfd                        | Y              | Y                            | Y                      |                                                                         |
| network-policies           | Y              | Y                            | Y                      |                                                                         |
| local-path-provisioner     | N              | Y                            | Y                      | Built into K3s                                                          |
| kube-metrics               | N              | N                            | Y                      | Built into K3s                                                          |
| traefik ingress controller | N              | N                            | Y                      | Built into K3s                                                          |
| serviceLB                  | N              | N                            | Y                      | Build into K3s                                                          |
| kubernetes-dashboard       | Y (EMT-S only) | N                            | Y                      |                                                                         |

### Unified Addon Deployment Approach

In prior releases, addon deployment varied between EMF-managed and EMT-S edges. EMF-managed edges utilized the App Deployment Manager (ADM), while EMT-S edges relied on RKE2's built-in Helm controller and addon manifest features.

For 3.1, both EMF-managed and EMT-S edges will leverage K3s/RKE2's built-in Helm controller and addon manifest features for addon deployment. This shift is driven by the reduced complexity of default addons, which now consist of simple applications with minimal configuration requirements. Many of these addons are also built into K3s by default. Consequently, advanced tools like ADM are no longer necessary for managing default addons, reducing operational overhead in both Cluster Orchestration and App Orchestration and ensuring a consistent user experience across different edge types.

### Deployment Strategies for Default Addons

While the unified approach simplifies addon deployment, specific deployment strategies are still required to address edge cases and ensure consistency across environments:

- **Cluster Templates**: Three default cluster templates will continue to be provided—privileged, baseline, and restricted. Each template will have distinct Kubernetes Pod Admission Standards configurations, while other configurations will remain consistent across templates.
- **Built-in Addons**: If an addon is built into K3s or RKE2, it will be utilized directly. For addons not built-in, a HelmChart definition will be added as additional files in the default cluster template to enable automatic deployment by K3s or RKE2.
- **Consistency Across Platforms**: The set of default addons will be identical for both K3s and RKE2. If an addon is built into K3s but not RKE2, it will be included in the addon manifest of the RKE2 default cluster template to ensure parity.

### Upgrade from 3.0 to 3.1

[todo: add plan]

## Affected components and Teams

## Implementation plan

[A description of the implementation plan, who will do them, and when.
This should include a discussion of how the work fits into the product's
quarterly release cycle.]

- replace extensions
- - replace openebs with Rancher's local path provisioner
- - replace gatekeeper with Pod Security Admission rules
- - update network policies for both EMT-managed, EMT-S to the appropriate restriction level

With these extensions replaced and network policies updates CO with RKE2 as a provider should be installed
and working as in 3.0.

- [WIP] package Helm add-ons 
- - package LPP (local path provisioner), NFD, network policies as Helm addons for the RKE2 provider
- - package NFD, network policies as Helm addons for the RKE2 provider

[WIP] Add more details about the packaging process
At this point this change should be tested with both providers (RKE2, K3s) on both EMT-managed, EMT-S edges

- [WIP] mark extensions like openebs, observability as optional

- clean-up base extensions package

## Open issues

Collaborate with the security team to evaluate enabling the Traefik Ingress Controller by default for EMT-S edges.

## Future Enhancements

The following items can be considered for future releases to enhance functionality and user experience:

- Include Kubernetes Dashboard in the default addon list for EMF-managed edges and provide a direct link for accessing the dashboard through EMF.
- Enable kube-metrics to deliver basic cluster metrics accessible via the Kubernetes Dashboard and EMF for EMF-managed edges.
- Develop an interface within EMF to monitor and display the deployment status of addons.
