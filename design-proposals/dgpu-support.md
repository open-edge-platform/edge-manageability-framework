# Design Proposal: Support for dGPU

Author(s): Rajeev Ranjan
Last updated: 2025-05-13

## Abstract

This proposal dicusses about improving support for graphics card specifically discrete ones. In the 3.0 release, support for Intel® GPUs is limited to both integrated (iGPU) and discrete (dGPU) Intel® Arc™ graphics, which rely on out-of-tree (OOT) drivers for functionality on Ubuntu 22.04. These OOT drivers are not included in the default kernel and must be manually installed, which can complicate deployment workflows. The Kubernetes device plugins for GPU support are however available in Application Catalog to be deployed on an as-needed basis.

## Proposal

### Requirements

1. New dGPU to support
    - Intel Battlemage B580
    - Nvidia P100 (post 3.1)
   Support to exiting iGPU is implicit here.

1. Avoid maintaining a dedicated OSProfile to support GPU

1. Ease of use - A customer should not have to worry about native OS packages/drivers

Ubuntu 24.04 with kernel (>6.11) should support in-tree GPU drivers which should ease usability. Failing to manage support of the drivers in-tree, using Intel Graphics Preview from Cannonical cannot be an option for release though it is worth using during development cycles. Since enablement is a higher priority, usage of day-2 procedures can continue.

Support for OS drivers in Edge Microvisor Toolkit (EMT) should be included in the immutable images. We do not have an option of installing the driver on day-2 here anyways.

| dGPU(OS)             | OS    | Kernel | Platform | Priority | Kernel cmd | SRIOV    | Workload  | DevicePlugin | Operator | Notes |
|----------------------|-------|--------|----------|----------|------------|----------|-----------|--------------|----------|-------|
| Intel B580 (EMT)     | 3.0   | -      | Xeon     | P0(3.0)  | Required   | Required | Geti, PDD |    Required  | No       |       |
| Intel B580 (Ubuntu)  | 24.04 | >6.11  | Xeon     | P1(3.0)  | Required   | Required | Geti, PDD |    Required  | No       |       |
| Nvidia P100 (EMT)    | 3.0   | -      | Xeon     | P2(>3.1) | Required   | Required | Geti, PDD |    No        | Yes      |       |
| Nvidia P100 (Ubuntu) | 24.04 | >6.11  | Xeon     | P2(>3.1) | Required   | Required | Geti, PDD |    No        | Yes      |       |

### Limitations and Debt of The Current Design

In the previous release (3.0), one of the primary concerns was the inability to enable Secure Boot when running GPU-based workloads. This limitation stemmed from the fact that the i915 drivers, which are essential for Intel integrated and discrete GPUs, were not signed by a trusted package distributor such as Canonical. As a result, Secure Boot, which requires all kernel modules to be signed by a recognized authority, could not be enabled without disabling or bypassing this security feature. This posed a significant challenge for users requiring both GPU acceleration and a secure boot environment.

### Proposed changes

1. **Avoid OSProfile for GPU Support**:
    - Eliminate the need for a dedicated OSProfile for existing Intel and Nvidia GPUs.
    - Rely solely on in-tree GPU drivers available in kernel (>6.11) with Ubuntu 24.04.
    - This approach not only simplifies deployment workflows but also reduces maintenance overhead.

1. **Edge Microvisor Toolkit (EMT) Support**:
   - Include Intel GPU drivers and Nvidia GPU Operator prerequisites(driver,sriov,kernel commandline) in the immutable image.

1. **Kubernetes extensions**:
    - Update existing extensions for Intel GPU (viz. - device-operator,gpu-plugin).
    - Add an extension for Nvidia (viz. - gpu-operator).

1. **SRIOV support**:
    - EMT should have SRIOV support along side the drivers.
    - Kubernetes device plugin/operator support for enabling SRIOV.
    - VMs to levarage kubevirt to access 

1. **Documentation**:
     - Instructions for deploying the Nvidia GPU Operator.
     - Update guidelines for running GPU workloads on Kubernetes.

1. **Monitoring and Metrics**:
   - Integrate GPU monitoring tools (e.g., Nvidia DCGM Exporter, Intel GPU metrics) into the Observability stack.
   - Provide dashboards for monitoring GPU usage and health.

1. **Testing Across Workloads**:
     - Geti
     - Pallet Defect Detection(PDD)

#### Day 2 Workflows:

## Affected components and Teams

## Implementation plan

### Phase 1 (for EMF 3.1 release)

The Intel Battlemage B580 GPU support shall be built initially starting with support in EMT followed by Ubuntu.

### Phase 2 (for EMF 3.2 release)

The Nvidia P100 GPU support can follow starting with support in EMT followed by Ubuntu.

## Open issues (if applicable)

1. Does Ubuntu in-tree kernel 6.11++ support Battelmage B580 as well?

