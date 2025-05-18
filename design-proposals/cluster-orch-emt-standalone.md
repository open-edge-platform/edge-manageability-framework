# Design Proposal: Kubernetes Installer Enhancement

Author(s): Hyunsun Moon

Last updated: 5/12/25

## Abstract

The Edge Microvisor Toolkit Standalone (EMT-S) provides a simplified approach to deploying an EMT edge without relying on the Edge Manageability Framework (EMF). This solution is particularly suited for quick EMT trials or use cases where connecting to a centralized EMF is either impractical or unnecessary.

This proposal focuses on enhancing the Kubernetes installer for EMT-S to support the same level of Kubernetes configurability as EMF-managed edges, including critical environment-specific configurations to ensure successful cluster creation such as Pod and Service subnet ranges and private registry mirrors. To achieve this, it introduces the ability to use a cluster template as input for building EMT-S bootable images. The installer updates also incorporate recent design decisions, such as the transition from RKE2 to K3s for Kubernetes distribution [todo: add link] and the integration of K3s into the EMT image [todo: add link].

## Proposal

The proposed EMT-S installation process is outlined in the following flowchart:

```mermaid
flowchart TD
    a((start)) -->
    b["Build or download installer"] -->
    c@{shape: lean-r, label: "(output)</br>sen-installation-files.tar.gz"} -->
    d{"Need custom config?"} --> |yes| e@{shape: lean-r, label: "(input)</br>updated config-file"} -->
    f["Create bootable usb"]
    d ---->|No |f -->
    g@{shape: lean-r, label: "(output)</br>bootable usb"} -->
    i["Boot target system</br>from usb"] --> j
    j["Reboot"] -->
    k("Kubernetes installed by cloud-init") -->
    l((end))
    subgraph hook[HookOS os-installer]
        h1["Install os on disk"] -->
        h2["`**Generate kubernetes.cfg cloud-init file**`"] -->
        h3["Install cloud-init cfgs"] -->
        h4["Create user</br>proxy, ssh setting"] -->
        h5["Boot order change to disk"]
    end
    i -.-> hook
    h5 -.-> j
```

#### Kubernetes Configuration in `config-file`

A new Kubernetes Configuration section will be added to the [`config-file`](https://github.com/open-edge-platform/edge-microvisor-toolkit-standalone-node/blob/main/standalone-node/installation_scripts/config-file). This section will align with the EMF default cluster template, which is based on the `spec` part of the [CAPI K3s ConfigSpec Custom Resource Definition](https://github.com/k3s-io/cluster-api-k3s/blob/main/bootstrap/api/v1beta2/kthreesconfig_types.go#L27). Below is an example:

```yaml
kubernetes: 
  kind: "KThreesControlPlaneTemplate"
  apiVersion: "controlplane.cluster.x-k8s.io/v1beta2"
  spec: 
    template: 
      spec: 
        # version is not updatable
        version: "v1.33.0+k3s1"
        files:
          path: "/var/lib/rancher/k3s/agent/etc/containerd/config.toml.tmpl"
          content: |
            {{ template \"base\" . }}

            [plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.'custom']
              runtime_type = "io.containerd.runc.v2"

            [plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.'custom'.options]
              BinaryName = "/usr/bin/custom-container-runtime"
              SystemdCgroup = true
        preK3sCommands: 
            - "mkdir -p /etc/systemd/system/k3s.service.d"
            - "echo '[Service]
EnvironmentFile=/etc/environment' > /etc/systemd/system/k3s.service.d/override.conf"
        agentConfig: 
          kubeletArgs: 
            - "--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"
          # airGapped configurations are not updatable
          airGapped: true
          airGappedInstallScriptPath: "/opt/install.sh"
        serverConfig: 
          disableComponents: 
            - "cloudController"
          kubeAPIServerArg: 
            - "--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"
          kubeControllerManagerArgs: 
          kubeSchedulerArgs: 
          clusterCidr: "10.42.0.0/16"
          serviceCidr: "10.43.0.0/16"
```

Users can modify these values when passing the `config-file` as a parameter for `bootable-usb-prepare.sh`. Most fields from the CAPI K3s Control Plane CRD spec can be configured through the `config-file`, except for the following:

- **Version**: The Kubernetes version is fixed to the K3s version embedded in EMT.
- **Files**: Configuration is limited to `/etc` and `/var` directories.
- **AgentConfig.AirGapped and AgentConfig.SirGappedInstallScriptPath**: These are always set to `true` and pre-defined path to ensure compatibility with K3s assets embedded in EMT image.

#### `config-file` format transition to YAML

The current `config-file` format is raw text. To improve readability and validation, we propose switching to a YAML format, as shown below:

```yaml
# config-file
version: 1.0
operatingSystem:
  users:
    - username: root
      password: cGFzc3dvcmQK
  proxy:
    httpProxy: http://proxy.example.com
    httpsProxy: http://proxy.example.com
    noProxy: localhost,127.0.0.1,10.0.0.0/8,192.168.0.0/16,.example.com
  sshKey:
kubernetes:
  kind: "KThreesControlPlaneTemplate"
  apiVersion: "controlplane.cluster.x-k8s.io/v1beta2"
  spec: 
    ...
```

Additionally, as the `config-file` content expands, the validation logic in `bootable-usb-prepare.sh` will be enhanced to ensure correctness before the file is used. YAML is better suited for this purpose.

#### Processing Kubernetes Configuration

The `config-file` will be processed by Hook OS during the OS installation via the [`os-installer` service](https://github.com/open-edge-platform/edge-microvisor-toolkit-standalone-node/blob/main/standalone-node/hook_os/hook-os.yaml#L159), which runs the [`install-os.sh`](https://github.com/open-edge-platform/edge-microvisor-toolkit-standalone-node/blob/main/standalone-node/hook_os/files/install-os.sh) script. A new step will be added to `install-os.sh` to process the Kubernetes Configuration section and generate a cloud-init configuration (`/mnt/etc/cloud/cloud.cfg.d/kubernetes.cfg`) for the target OS. The static Kubernetes bootstrap command in the default [`cloud-init.yaml`](https://github.com/open-edge-platform/edge-microvisor-toolkit-standalone-node/blob/main/standalone-node/hook_os/files/cloud-init.yaml) will be removed.

#### Cluster Bootstrapper

A new Go-based tool, `cluster-bootstrapper`, will be introduced to generate the cloud-init configuration from the Kubernetes Configuration section of the `config-file`. This tool will leverage the [CAPI K3s bootstrap provider codebase](https://github.com/k3s-io/cluster-api-k3s/tree/main/pkg/cloudinit). The `cluster-bootstrapper` binary will be released as a Docker image and included in the `volume` section in `hook-os.yaml` for use by the `os-installer` service. 

```yaml
# hook-os.yaml
...
services:
  - name: os-installer
    image: debian:12.10
    command: ["/bin/bash", "-c", "/etc/scripts/install-os.sh"]
    capabilities:
      - all
    net: host
    binds.add:
      - /dev:/dev
      - /run:/run
      - /etc/scripts:/etc/scripts/
      - /:/host
      - /var/log/os-installer.log:/var/log/os-installer.log
      - /etc/scripts/install-os.sh:/etc/scripts/install-os.sh
      - bootstrapper:/mnt/bootstrapper
    runtime:
      mkdir:
        - /var/lib/os-installd
    devices:
      - path: all
        type: b
      - path: all
        type: c
volumes:
- name: bootstrapper
  image: registry-rs.edgeorchestration.intel.com/edge-node/cluster-bootstrapper:1.0.0
  readonly: true
```

Example of `cluster-bootstrapper` usage in `install-os.sh`.

```bash
create_cloud_init_for_k8s() {
    /mnt/bootstrapper/cluster-bootstrapper -config-file "$CONFIG_FILE" -out /mnt/etc/cloud/cloud.cfg.d/kubernetes.cfg
    chmod +x /mnt/etc/cloud/cloud.cfg.d/kubernetes.cfg
}
```

#### First Boot Behavior

With these changes, upon the first boot, the cloud-init configuration generated by the `cluster-bootstrapper` tool will bootstrap K3s on the target system.

### Addon

To align with the recent design decision to minimize resource consumption for Kubernetes itself and dedicate more resources to workloads [todo: add link to the ADR], only essential addons will be installed by default with the K3s core. These include:

- CoreDNS
- Kube-router network policy controller
- Local-path-provisioner persistent volume controller
- Traefik ingress controller
- ServiceLB load-balancer controller
- Metrics Server
- Helm controller
- Kubernetes dashboard

Most of these addons are bundled with K3s by default and require no additional effort to enable, except for the Kubernetes dashboard. The Kubernetes dashboard will be installed using K3s's built-in [Helm controller](https://github.com/k3s-io/helm-controller/) and [auto-deploying AddOn manifests](https://docs.k3s.io/installation/packaged-components) features.

Below is an example of how to configure the Kubernetes dashboard installation using a cluster template:

```yaml
kubernetes: 
  kind: "KThreesControlPlaneTemplate"
  apiVersion: "controlplane.cluster.x-k8s.io/v1beta2"
  spec: 
    template: 
      spec: 
        version: "v1.33.0+k3s1"
        files:
          path: /var/lib/rancher/k3s/server/manifests/kubernetes-dashboard.yaml
          content: |
            apiVersion: helm.cattle.io/v1
            kind: HelmChart
            metadata:
              namespace: kube-system
              name: kubernetes-dashboard
            spec:
              targetNamespace: kubernetes-dashboard
              createNamespace: true
              chartContent: # base64 encoded kubernetes-dashboard chart content
              valuesContent: |-
                service:
                  type: LoadBalancer
          owner: root:root
  ...
```

### Access

#### Accessing the Kubernetes API

The Kubernetes API (kube-apiserver) can be accessed either directly from the EMT-S machine or from external devices that can route to the EMT-S machine.

For seamless access to the Kubernetes API from within the EMT-S machine, an additional `postK3sCommand` in the Kubernetes configuration can be added. This ensures that the `kubectl` binary, readily available by K3s after K3s installation, is properly configured for immediate use:

```yaml
# config-file
kubernetes: 
  kind: "KThreesControlPlaneTemplate"
  apiVersion: "controlplane.cluster.x-k8s.io/v1beta2"
  spec: 
    template: 
      spec: 
        postK3sCommand:
          - mkdir -p ~/.kube
          - cp /etc/rancher/k3s/k3s.yaml ~/.kube/config
    ...
```

To access the Kubernetes API from an external machine, users must manually copy the `kubeconfig` file from the EMT-S machine to the client device. After copying, the server address in the `kubeconfig` file should be updated to reflect the external IP address of the EMT-S machine. This can be improved in future releases by introducing CLI tool (see Open Issues).

#### Accessing Application Services

Once an application is deployed, users can access its services through various methods that Kubernetes supports. The recommended approach in EMT-S is to use either a `LoadBalancer`-type Service or an `Ingress`. 

By default, K3s includes ServiceLB and the Traefik Ingress Controller, both of which are configured to expose services using the external IP address of the EMT-S node. For more details, refer to the [K3s Networking Services documentation](https://docs.k3s.io/networking/networking-services).

[todo: Add an example of Kubernetes Dashboard configuration and access with LB]

## Rationale

For K3s configurability, the [K3s config file](https://docs.k3s.io/installation/configuration#configuration-file) can be used to pass custom K3s configurations to a target system instead of the CAPI provider-based approach. K3s config file method may simplify the process by allowing additional configurations to be directly copied to a file under `/etc/rancher/k3s/config.yaml.d/`, not requiring the need for cloud-init conversion. 

However, using a CAPI provider based approach offers several advantages:

- **Enhanced Flexibility**: The template supports advanced configurations that go beyond native K3s configuration capabilities. These include pre- and post-installation commands and the ability to create supplementary files. Such features are particularly useful for tasks like configuring proxies or customizing containerd settings, as demonstrated in EMF-managed edges.
- **Consistency**: It ensures a uniform configuration approach, providing consistent user experience when transitioning EMT-S edge to an EMF-managed edge in the future.
- **Extensibility**: The cluster template design is more scalable, enabling support for other Kubernetes distributions compatible with the Cluster API (CAPI).

## Affected components and Teams

Cluster Orchestration
Edge Microvisor Toolkit Standalone

## Implementation plan

## Test plan

## Open issues

Introducing a CLI tool to replace the current Makefile and multiple shell scripts can further improve user experience. The CLI can provide a unified interface with subcommands for common tasks, including but not limited to:

- **Build Installer**: Simplify the process of building the EMT-S installer by encapsulating all necessary steps into a single command.
- **Create Bootable USB**: Automate the creation of a bootable USB drive with the required configurations, reducing manual intervention.
- **Get Kubeconfig**: Retrieve the `kubeconfig` file from the EMT-S machine and optionally update it with the external IP address for seamless access to the Kubernetes API.

This can be considered in future releases.