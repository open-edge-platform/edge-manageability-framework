# Design Proposal: Add suppport for K3s

Author(s): Hyunsun Moon, Andy Bavier, Denisio Togashi

Last updated: 4/28/25

## Abstract

This proposal aims to enhance the Cluster Orchestration by introducing K3s as an additional option for edge cluster deployment. Currently, the framework supports only RKE2. By incorporating K3s, a production-grade, CNCF-certified lightweight and simplified Kubernetes distribution, the framework will allow users to select the most appropriate Kubernetes distribution for their specific use cases and the resource constraints of their environments. This addition provides greater flexibility in managing diverse deployment scenarios.

## Proposal

### User Stories (Need Update)

#### Story 1

As an edge application deployment engineer, I want to transition from Docker to Kubernetes without incurring additional overhead for running Kubernetes itself, so that I can allocate more resources to application workloads and leverage advanced container orchestration capabilities that Kubernetes provides.

#### Story 2

As an edge operator, I want to deploy Kubernetes clusters in environments with limited computational resources, such as remote IoT installations or small-scale edge sites, and efficiently manage and operate Kubernetes-based applications without exceeding resource capacities.

#### Story 3

As an edge operator, I want the flexibility to choose between RKE2 and K3s for Kubernetes deployments in specific environments, allowing me to customize the edge infrastructure to best meet the unique requirements and constraints of various projects.

#### Story 4

As a system reliability enginner, I want to manage edge clusters with minimal overhead, so that I can focus on optimizing application performance and reliability.

#### Story 5

As a product manager, I want to ensure that Edge Managibility Framework and Cluster Orchestration is adaptable to future Kubernetes distributions, so that we can continue to meet evolving user needs and industry standards.


### Design

This section outlines the changes required to integrate K3s as an additional control plane provider within the Cluster Orchestration. The changes involves updates to the Cluster Orchestration API, controllers, southbound handlers, sample cluster templates, and the cluster-api-provider-k3s.

#### API Changes

The proposed integration of K3s does not introduce any breaking changes to the existing Cluster Orchestration API, as defined in the [API spec](https://github.com/open-edge-platform/cluster-manager/blob/release-2.0/api/openapi/openapi.yaml#L904-L909). Specifically, the current API allows for the specification of `controlplaneprovidertype`, with rke2 being the sole valid option. (Note that `kubeadm` is listed, but it is intended for internal testing purposes only.) This proposal will add K3s to the list of valid options and set it as the default control plane provider.

#### Controller Changes

#### Southbound Handler Changes

#### Sample Cluster Templates

Cluster Orchestration provides sample cluster templates that users can employ to create new clusters directly or as a basis for developing customized templates with additional configurations. For example, sample templates for RKE2 clusters are available [here](https://github.com/open-edge-platform/cluster-manager/tree/release-2.0/default-cluster-templates). Similarly, sample cluster templates for K3s, featuring three distinct security levels—restricted, baseline, and privileged—will be provided and installed by default with the orchestrator.

#### Cluster API K3s Provider Changes

## Rationale

Alternative approaches, such as MicroK8s and k0s, were considered but ultimately set aside due to specific limitations. MicroK8s is primarily designed for local development with a focus on single-node setups, which may not meet the scalability needs of edge deployments. On the other hand, k0s, while suitable for both small and large clusters, lacks the popularity and community support that K3s enjoys. Here are more details about advantages of K3s over these alternatives:

**Resource Efficiency**: K3s boasts a compact package size of 68MB (total 556MB) and requires only 1-2 CPUs and 1.7-2GB of RAM, making it particularly well-suited for environments with constrained computational resources. This efficiency enables more resources to be dedicated to application workloads rather than the Kubernetes itself.

**Ease of Deployment**: As a single binary with embedded data store either SQLite or etcd, K3s simplifies the setup process and reduces the complexity often associated with Kubernetes deployments.

**Community and Adoption**: With 29.4k GitHub stars, K3s has achieved wide adoption and boasts a strong community, providing robust support and ongoing development. This popularity underscores its proven track record and reliability across various deployment scenarios.

**Flexibility and Customization**: K3s offers a variety of default add-ons, such as CNI (flannel by default), helm controller, and metrics-server, which can be disabled if not required. This flexibility allows users to customize their deployments to meet specific project requirements and constraints, enhancing adaptability to diverse use cases.


## Affected components and Teams

Cluster Manager
Cluster API Provider Intel
Cluster Tests
Edge Managibility Framework

## Implementation plan

[A description of the implementation plan, who will do them, and when.
This should include a discussion of how the work fits into the product's
quarterly release cycle.]

### Phase 1

### Phase 2 

### Phase 3

### Test Plan

## Open issues (if applicable)

N/A