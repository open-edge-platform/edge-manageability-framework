# Design Proposal: Simplify Default Addon

Author(s): Hyunsun Moon, Madalina Lazar

Last updated: 05/21/2025

## Abstract

This proposal aims to streamline the default addons installed in Kubernetes edge environments by retaining only
essential components. The document also details planned changes for managing default addons in upcoming releases.

## Proposal

### Common Default Addons for 3.1 Release

The table below outlines the addons included in the 3.0 release and the planned changes for the 3.1 release, along with
relevant notes:

| Addon                      | Default in 3.0 | Default in 3.1 | Notes                                                                   |
| -------------------------- | -------------- | -------------- | ----------------------------------------------------------------------- |
| gatekeeper and constraints | Y              | N              | Replace with Kubernetes Pod Security Admission Controller and Standards |
| cert-manager               | Y              | N              | Move to optional observability package                                  |
| telegraf                   | Y              | N              | Move to optional observability package                                  |
| prometheus stack           | Y              | N              | Move to optional observability package                                  |
| node-exporter              | Y              | N              | Move to optional observability package                                  |
| openebs                    | Y              | N              | Drop                                                                    |
| fluent-bit                 | Y              | N              | Move to optional observability package                                  |
| nfd                        | Y              | N              | Drop until required                                                     |
| calico                     | Y              | Y              |                                                                         |
| network-policies           | Y              | Y              |                                                                         |
| local-path-provisioner     | N              | Y              | Built into K3s                                                          |

### Default Addons for Different EMT Configurations

The table below outlines additional addons enabled by default for different edge configurations:

| Addon                      | Standard | Maverick Flats | Notes          |
| -------------------------- | -------- | -------------- | -------------- |
| kube-metrics               | Y        | N              | Built into K3s |
| traefik ingress controller | N        | N              | Built into K3s |
| serviceLB                  | N        | N              | Built into K3s |
| Kubevirt                   | N        | Y              | Custom version |
| CDI                        | N        | N              |                |
| MetalLB                    | N        | N              |                |
| GPU device plugin          | N        | Y              |                |
| MF device plugin           | N        | Y              |                |

### Unified Addon Deployment Approach

In prior releases, addon deployment varied between EMF-managed and EMT-S edges. EMF-managed edges utilized the App
Deployment Manager (ADM), while EMT-S edges relied on RKE2's built-in Helm controller and addon manifest features.

For 3.1, both EMF-managed and EMT-S edges will leverage K3s/RKE2's built-in Helm controller and addon manifest features
for addon deployment. This shift is driven by the reduced complexity of default addons, which now consist of simple
applications with minimal configuration requirements. Many of these addons are also built into K3s by default.
Consequently, advanced tools like ADM are no longer necessary for managing default addons, reducing operational overhead
in both Cluster Orchestration and App Orchestration and ensuring a consistent user experience across different edge
types.

### Deployment Strategies for Default Addons

While the unified approach simplifies addon deployment, specific deployment strategies are still required to address
edge cases and ensure consistency across environments:

- **Cluster Templates**: Three default cluster templates will continue to be provided—privileged, baseline, and
  restricted. Each template will have distinct Kubernetes Pod Admission Standards configurations, while other
  configurations will remain consistent across templates.
- **Built-in Addons**: If an addon is built into K3s or RKE2, it will be utilized directly. For addons not built-in, a
  HelmChart definition will be added as additional files in the default cluster template to enable automatic deployment
  by K3s or RKE2.
- **Consistency Across Platforms**: The set of default addons will be identical for both K3s and RKE2. If an addon is
  built into K3s but not RKE2, it will be included in the addon manifest of the RKE2 default cluster template to ensure
  parity.

### Airgap Install Requirement for EMT-S

For EMT-S, ensuring functionality without Internet access is a critical requirement. To meet this need, addon images and
charts not built into K3s, such as Calico, will be embedded into EMT image.

K3s includes a feature that automatically imports container images stored in `/var/lib/rancher/k3s/agent/images` to
`containerd` during bootstrap. We will use this feature and package additional addon images into a tarball and place it
in the same directory. This ensures that all required images for the addons are preloaded from local path during the
bootstrap process.

Also, Helm charts for the addons will be included in the EMT image under `/var/lib/rancher/k3s/server/static/`. This
ensures that the addons can be deployed without requiring Internet access during the bootable image build process,
aligning with airgap installation requirements.

## Future Enhancements

The following items can be considered for future releases to enhance functionality and user experience:

- Include Kubernetes Dashboard in the default addon list for EMF-managed edges and provide a direct link for accessing
  the dashboard through EMF.
- Enable kube-metrics to deliver basic cluster metrics accessible via the Kubernetes Dashboard and EMF for EMF-managed
  edges.
- Develop an interface within EMF to monitor and display the deployment status of addons.
