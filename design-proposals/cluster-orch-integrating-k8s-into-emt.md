# Design Proposal: Integrating Kubernetes into Edge Microvisor Toolkit

Author(s): Hyunsun Moon

Last updated: 5/2/25

## Abstract

The Edge Microvisor Toolkit (EMT) is an operating system specifically designed for hosting Kubernetes, streamlining traditional general-purpose operating systems by including only the essential components needed to run container-based applications. Our experience from previous releases has demonstrated that EMT's design principles—image-based deployment and immutable root filesystem—enhance the reliability and consistency of cluster creation compared to even well-maintained general-purpose operating systems, such as Ubuntu.

This proposal aims to extend these proven approaches to Kubernetes lifecycle management by integrating Kubernetes directly into EMT. The integration involves two key changes: incorporating Kubernetes into the EMT build process and embedding it within the image. While this does not mean that EMT machines will come with Kubernetes pre-installed and running, users will still need to configure and initialize Kubernetes. Given that turning into a Kubernetes node is the primary and only use case for EMT machines, a new option will be introduced during the host registration step to allow for the automatic installation of Kubernetes with pre-configured default settings. This addition will streamline the process, making edge devices ready for use more efficiently.

With this design change, we anticipate reduced and consistent cluster creation times, enhanced reliability by eliminating third-party dependencies, improved security through faster application of CVEs, and an improved user experience by providing a zero-touch option from device onboarding to a cluster-ready state.

## Proposal

### Building Kubernetes

#### What to build and package

Cluster Orchestration supports two flavors of Kubernetes distributions: RKE2 and K3s [todo: add link to k3s design proposal]. In the initial implementation, we will focus on K3s, as it is set to be the default Kubernetes distribution in Cluster Orchestration. K3s is simpler to build compared to RKE2 because it integrates Kubernetes into a single binary, making it a preferred choice for quick implementation.

The build process will focus on creating the first-level of binary and assets, excluding their dependencies. For instance, the K3s build process involves downloading and building containerd, etcd, and other dependencies from the upstream repositories. We will leverage their existing build process without building these nested dependency separately. For a comprehensive list of dependencies that we are not directly building, refer to the [K3s build script](https://github.com/k3s-io/k3s/blob/master/scripts/version.sh). This decision may evolve based on future needs.

Assets to be built and packaged for K3s installation include the following. The list of container image is sourced from https://github.com/k3s-io/k3s/blob/master/scripts/airgap/image-list.txt. In the initial implementation, we will focus on building the K3s binary and eventually extend to building container images for addons.

- k3s (binary)
- k3s-selinux (rpm)
- klipper-helm (container image)
- klipper-lb (container image)
- local-path-provisioner (container image)
- coredns (container image)
- busybox (container image)
- traefik (container image)
- metrics-server (container image)
- pause (container image)
- install.sh (script)

For simplicity, the K3s version will be limited to the latest stable release, and users will not be able to select a specific version. A new EMT image will be released under two additional conditions: when a new K3s version becomes available or when a critical CVE or security patch is required, supplementing the existing release cadence. This decision may evolve based on future user requirements.

#### How to build and package

To build and package K3s, we will leverage the existing EMT build pipeline. As an RPM-based distribution, EMT simplifies the process of building and creating new RPM packages that can be installed on EMT. This involves writing a SPEC file that specifies the source location and build commands, and placing it in the SPECS folder of the repository. This approach offers significant advantages, such as eliminating the need to maintain forks of upstream repositories while providing the flexibility to apply patches and standardizing the build process for various software components with diverse build requirements. And of course, the subsequent step of integrating Kubernetes into the EMT image becomes very straightforward.

Here is an example of SPEC file to build and package K3s binary:

```
...
# This is not a complete SPEC

Source0: https://github.com/k3s-io/k3s/archive/refs/tags/%{version}.tar.gz

BuildRequires: make
BuildRequires: docker

%prep
%setup -q

%build
make local-binary

%install
mkdir -p %{buildroot}/usr/local/bin
install -m 0755 k3s %{buildroot}/usr/local/bin/k3s

mkdir %{buildroot}/opt
install -m 0755  install.sh %{buildroot}/opt/install.sh

%files
/usr/local/bin/k3s
/opt/install.sh
...
```

For container images, we'll approach this in two phases, as it's not straightforward:

Phase 1: Download and package the pre-built airgap image tarball. The configuration of K3s to use this image instead of downloading from the Internet will be discussed in #Cluster Manager Changes.

Here is a modified version of SPEC file with pre-built addon image tarball. 

```
...
# This is not a complete SPEC

Source0: https://github.com/k3s-io/k3s/archive/refs/tags/%{version}.tar.gz
Source1: https://github.com/k3s-io/k3s/releases/download/%{version}/k3s-airgap-images-amd64.tar.zst

BuildRequires: make
BuildRequires: docker

%build
make local-binary

%install
mkdir -p %{buildroot}/usr/local/bin
install -m 0755 bin/k3s %{buildroot}/usr/local/bin/k3s

mkdir %{buildroot}/opt
install -m 0755 install.sh %{buildroot}/opt/install.sh

mkdir -p %{buildroot}/var/lib/rancher/k3s/agent/images
install -m 0644 %{SOURCE1} %{buildroot}/var/lib/rancher/k3s/agent/images/k3s-airgap-images-amd64.tar.zst

%files
/usr/local/bin/k3s
/opt/install.sh
/var/lib/rancher/k3s/agent/images/k3s-airgap-images-amd64.tar.zst
...
```

Phase 2: Create a SPEC file for each image, with contents that may vary based on the source and build requirements. Since the executables packaged as RPMs cannot be directly installed on EMT and need to run as containers, additional stages in the pipeline are necessary beyond the standard build. These stages include constructing the container image using a Dockerfile and publishing the image to an OCI registry.

The following sections assume Phase 1.

### Making Kubernetes part of EMT

Once RPM packages for K3s and addons are ready, integrating them into the EMT image is straightforward, as detailed in updating the image configuration. A new package list file, `toolkit/imageconfigs/packagelists/k3s.json`, containing the k3s and k3s-selinux packages will be created and appended to the `PackageLists` section of all edge image configuration files.

It is important to note that while the K3s binary benefits from the immutability provided by its placement in the read-only partition of EMT, ensuring it cannot be updated without an EMT image update, the same level of immutability is not guaranteed for addon images. Addons, which are essentially Kubernetes Pods, can be updated after their initial creation using images loaded from the embedded tarball.

### Cluster Manager Changes

To ensure that K3s on EMT utilizes the binary and images embedded within EMT, rather than downloading them from the Internet, `INSTALL_K3S_SKIP_DOWNLOAD` environment variable should set to true when bootstrapping K3s. This prevents the bootstrap script from attempting to download components externally.

For configurations using CAPI provider for K3s, the equivalent setup involves specifying the airGapped option in the control plane template. Here is an example configuration:

```yaml
apiVersion: controlplane.cluster.x-k8s.io/v1beta2
kind: KThreesControlPlaneTemplate
metadata:
  name: k3s-control-plane
spec:
  template:
    spec:
      kthreesConfigSpec:
        agentConfig:
          airGapped: true
```

The Cluster Manager should implement logic to determine whether the target host is running EMT using host information retrieved from the Infra Manager. If the host is identified as EMT machine, the `airGapped` configuration should be enabled. Since the Cluster Manager internally uses ClusterClass, the airGapped value can be dynamically patched using a variable defined in the Cluster object.

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: ClusterClass
metadata:
  name: k3s
spec:
  controlPlane:
    ref:
      apiVersion: controlplane.cluster.x-k8s.io/v1beta2
      kind: KThreesControlPlaneTemplate
      name: k3s-control-plane
  patches:
  - name: airGapped
    definitions:
    - selector:
        apiVersion: controlplane.cluster.x-k8s.io/v1beta2
        kind: KThreesControlPlaneTemplate
        matchResources:
          controlPlane: true
      jsonPatches:
      - op: add
        path: /spec/template/spec/kthreesConfigSpec/agentConfig/airGapped
        valueFrom:
          variable: airGapped
  variables:
    - name: airGapped
      required: true
      schema:
        openAPIV3Schema:
          type: boolean
          default: false
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: cluster-on-emt
spec:
  topology:
    ...
    variables:
    - name: airGapped
      value: true
```

### Host Registration Workflow Change

### Day-2 Operations

## Rationale

[A discussion of alternate approaches that have been considered and the trade
offs, advantages, and disadvantages of the chosen approach.]

## Affected components and Teams

Edge Microvisor Toolkit
Cluster Orchestration
UX/UI

## Implementation plan

[A description of the implementation plan, who will do them, and when.
This should include a discussion of how the work fits into the product's
quarterly release cycle.]

### Phase 1

### Phase 2

### Phase 3

### Test Plan

## Open issues (if applicable)

[A discussion of issues relating to this proposal for which the author does not
know the solution. This section may be omitted if there are none.]
