# Design Proposal: Support for dGPU

Author(s): Rajeev Ranjan, Sandeep Sharma
Last updated: 2025-05-16

## Abstract

This design proposal outlines a plan to enhance discrete GPU (dGPU) support, addressing current limitations and introducing new functionalities. Key issues identified include inadequate support for newer dGPUs like Intel Battlemage, reliance on out-of-tree drivers, challenges with Secure Boot due to unsigned i915 drivers, and inefficiencies from redundant OSProfiles for GPU-equipped systems. The proposal aims to streamline driver integration as extensions, enable Single Root I/O Virtualization (SRIOV), and improve overall system compatibility and user efficiency for GPU workloads on EdgeNodes.

## Limitations and Debt of The Current Design
In the earlier release (3.0), a key issue was the inability to activate Secure Boot for GPU-based workloads. This limitation arose because the i915 drivers, crucial for Intel's integrated and discrete GPUs, lacked signatures from a trusted package distributor like Canonical. Consequently, Secure Boot, which mandates that all kernel modules be signed by an authorized entity, couldn’t be engaged without compromising or circumventing this security feature. This created a substantial challenge for users who needed both GPU acceleration and a secure boot environment.

### Goals/Requirements (release 3.1 onwards)


1. **Support for additional GPUs**
   * Intel Battlemage B580
   * NVIDIA P100
2. **OSProfile agnostic GPU enablement**: This aims to simplify deployment by not requiring separate OS profiles for systems with GPUs.
3. **One-click enablement/installation of GPUs**:Streamlining the GPU setup process for users
4. **Component-Platform compatibility metrics**:

| GPU | OS | Kernel | Platform | Priority | Kernel cmd | SRIOV | DevicePlugin | Operator | Notes |
|----|----|----|----|----|----|----|----|----|----|
| Intel iGPU | EMT 3.0 | - | Xeon, Core | P0(3.1) | Required | Required | Required | - |    |
| Intel iGPU | Ubuntu 24.04 | >6.11 | Xeon, Core | P0(3.1) | Required | Required | Required | - |    |
| Intel B580 | EMT 3.0 | - | Xeon | P0(3.1) | Required | 3.2 | Required | - |    |
| Intel B580 | Ubuntu 24.04 | >6.11 | Xeon | P1(3.1) | Required | 3.2 | Required | - | In-tree driver verification WIP |
| Nvidia P100 | EMT 3.0 | - | Xeon | P2(3.2) | Required | Required | - | Required |    |
| Nvidia P100 | Ubuntu 24.04 | >6.11 | Xeon | P0(3.1) | Required | - | - | Required | Needs a specific kernel 6.11.x |


### Proposed changes


1. **Avoid OSProfile for GPU Support**:
   * The plan is to eliminate the need for dedicated OSProfiles for Intel and NVIDIA GPUs. Instead, the system will rely on in-tree GPU drivers available in kernel versions greater than 6.11 with Ubuntu 24.04. This is expected to simplify deployment and reduce maintenance.
2. **Ensure in-tree drivers (critical for secure boot)**:
   Ubuntu 24.04, featuring kernel versions later than 6.11, is expected to include in-tree GPU drivers, enhancing overall usability. If in-tree driver support cannot be achieved, utilizing Intel Graphics Preview from Canonical, while beneficial during development cycles, will not be feasible for the official release. Given the priority of enablement, the continuation of day-2 procedures remains essential.
3. **Edge Microvisor Toolkit (EMT) Support**:
   * For EMT, Intel GPU drivers and prerequisites for the NVIDIA GPU Operator (including drivers, SR-IOV settings, and kernel command line parameters) will be included in the immutable OS image.
4. **ITEP Application Extensions**:

   Develop/update the cluster extensions supported by the platform to ease the consumption of GPUs.
   https://github.com/open-edge-platform/cluster-extensions/blob/main/README.md
   * **Intel**
     * Existing extensions, such as the device-operator and gpu-plugin, will be updated. The Intel GPU device plugin for Kubernetes facilitates access to Intel discrete and integrated GPUs, registering resources like gpu.intel.com/i915 and gpu.intel.com/xe within a Kubernetes cluster
   * **NVIDIA**
     * A new extension will be created to configure and install the NVIDIA GPU Operator using its Helm chart. The NVIDIA GPU Operator automates the management of all NVIDIA software components required to provision GPUs in Kubernetes, including drivers, the Kubernetes device plugin, the NVIDIA Container Runtime, and monitoring tools.
        ![gpu-operator-extension](images/nvidia-gpu-operator-extension-package.png)
     * **For EMT** : pre-installed drivers in the immutable OS will be utilized by the GPU Operator.
     * **For Ubuntu** : the extension package will configure the GPU Operator to detect the OS and install optimal drivers automatically.
5. **SRIOV support**:

   Single Root I/O Virtualization (SRIOV) support is a key enhancement
   * EMT will include SRIOV support alongside the drivers.
   * Kubernetes device plugins or operators will be used to enable SRIOV. The SR-IOV Network Device Plugin for Kubernetes can discover and manage SR-IOV capable network devices . NVIDIA also provides an SR-IOV Network Operator  and documentation for using SR-IOV in Kubernetes.
   * Virtual machines will leverage KubeVirt to access SR-IOV capabilities. Intel offers an "Intel Graphics SR-IOV Enablement Toolkit" with components to facilitate Graphics SR-IOV for applications using KubeVirt, allowing VMs direct access to partitioned GPU resources.
6. **Documentation**:

   Updated documentation will be provided, including:
   * Instructions for deploying the NVIDIA GPU Operator.
   * Revised guidelines for running GPU workloads on Kubernetes.
7. **Monitoring and Metrics**:

   GPU monitoring will be integrated into the observability stack:
   * Tools like the NVIDIA DCGM Exporter and Intel GPU metrics will be incorporated. The NVIDIA Data Center GPU Manager (DCGM) Exporter allows for the collection of GPU metrics in Prometheus format and can be deployed in Kubernetes, often via a Helm chart . It can also be deployed as part of the NVIDIA GPU Operator . dcgm-exporter collects metrics for all GPUs on a node and can associate these metrics with specific Kubernetes pods.
   * Dashboards will be provided for monitoring GPU usage and health.
8. **Testing Across Workloads**:

   The proposed dGPU support will be tested using workloads such as
   * Geti
   * Pallet Defect Detection(PDD)

### Day 2 Workflows:

```
<workflow goes here>
```

## Affected components and Teams

## Implementation plan

The implementation is planned in two phases:

### Phase 1 (for EMF 3.1 release)


1. Maintain support for Intel® Arc™ iGPU and dGPU in Ubuntu
2. Intel Battlemage B580 dGPU support in EMT
3. Intel Battlemage B580 dGPU support in Ubuntu
4. Nvidia P100 dGPU support in Ubuntu

### Phase 2 (for EMF 3.2 release)


1. Nvidia P100 GPU support in EMT

## Open issues


1. Does Ubuntu in-tree kernel 6.11++ support Battlemage B580 as well?

   The last tested version was not working. Tweaks were required. Needs to be verified against latest version.
2. No requirement for iGPU & dGPU together at the moment


