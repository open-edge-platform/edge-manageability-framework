# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

metrics:
  enabled: true

# CAPI providers are managed using the CAPI operator and configured through its CRDs.
# The Helm chart .spec is directly passed to the CRD .spec without modification.

# https://doc.crds.dev/github.com/kubernetes-sigs/cluster-api-operator/operator.cluster.x-k8s.io/CoreProvider/v1alpha2@v0.15.1
core:
  name: cluster-api
  namespace: capi-system
  spec:
    version: v1.9.7
    configSecret:
      namespace: capi-variables
      name: capi-variables
    manager:
      featureGates:
        MachinePool: true
        ClusterResourceSet: true
        ClusterTopology: true
        RuntimeSDK: false
        MachineSetPreflightChecks: true
        MachineWaitForVolumeDetachConsiderVolumeAttachments: true
    deployment:
      containers:
      - name: manager
        args:
          "--insecure-diagnostics": "true"
          "--additional-sync-machine-labels": ".*"
          "--cluster-concurrency": "10"
          "--clustercache-client-burst": "150"
          "--clustercache-client-qps": "100"
          "--kube-api-burst": "150"
          "--kube-api-qps": "100"
          "--machine-concurrency": "10"
    additionalManifests:
      name: core-additional-manifest
      namespace: capi-system


# https://doc.crds.dev/github.com/kubernetes-sigs/cluster-api-operator/operator.cluster.x-k8s.io/BootstrapProvider/v1alpha2@v0.15.1
bootstrap:
  providers:
    - name: k3s
      namespace: capk-system
      spec:
        fetchConfig:
          url: "https://github.com/k3s-io/cluster-api-k3s/releases/v0.3.0/bootstrap-components.yaml"
        configSecret:
          namespace: capi-variables
          name: capi-variables
        additionalManifests:
          name: bootstrap-k3s-additional-manifest
          namespace: capk-system

# https://doc.crds.dev/github.com/kubernetes-sigs/cluster-api-operator/operator.cluster.x-k8s.io/ControlPlaneProvider/v1alpha2@v0.15.1
controlplane:
  providers:
    - name: k3s
      namespace: capk-system
      spec:
        fetchConfig:
          url: "https://github.com/k3s-io/cluster-api-k3s/releases/v0.3.0/control-plane-components.yaml"
        configSecret:
          namespace: capi-variables
          name: capi-variables
        additionalManifests:
          name: controlplane-k3s-additional-manifest
          namespace: capk-system
