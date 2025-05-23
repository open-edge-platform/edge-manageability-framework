# Orchestrator Cloud Infrastructure

## Design philosophy

- Modules should be general and simple which should include single category of
  tasks
- Example: The eks module should only create and manage EKS-releated resources,
  it should not be able to manage other AWS resources which does not directly
  connect to the EKS
- Similar deployments shares the same Terraform configuration, and use variable
  files to manage the difference.

## Repository architecture

### Directory for different categories

- **buckets**: Sets up S3 buckets to store Terraform state
- **orchestrator**: Contains Terraform configuration for orchestrator
  environments
  - `cluster`: Cluster configs, including EKS cluster, storage, database, ...
  - `vpc`: Virtual private cloud configs
  - `orch-load-balancer`: Load balancing components
  - `orch-route53`: Route53 DNS configs
  - `pull-through-cache-proxy`: Proxy that redirects OCI requests to
      corresponding pull through cache path.

### The module directory

`module` directory contains all modules for deployment which can be used by each
deployments.

### The utils directory

This directory contains utilities to manage the deployment.

## Naming convention

Directory, filename: all lower case with `-` as word separator.

Terraform resource, data name: all lower case with `_` as word separator.

Infra resource name(e.g., cluster name, vm name, subnet name): all lower case
with `-` as word separator.
