# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

metrics:
  enabled: true

# CAPI providers are managed using the CAPI operator and configured through its CRDs.
# The Helm chart .spec is directly passed to the CRD .spec without modification.

core:
  name: cluster-api
  namespace: capi-system
  spec:
    version: v1.11.5
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

bootstrap:
  providers:
    - name: k3s
      namespace: capk-system
      spec:
        version: v100.0.0-dt
        fetchConfig:
          url: "https://gist.githubusercontent.com/richardcase/d85564c8a8a62615b5e75fd98711dd22/raw/4eb2b29d785d4fdfbd22223517bc14482d0ba2ed/bootstrap-components.yaml"
        configSecret:
          namespace: capi-variables
          name: capi-variables
        additionalManifests:
          name: bootstrap-k3s-additional-manifest
          namespace: capk-system

controlplane:
  providers:
    - name: k3s
      namespace: capk-system
      spec:
        version: v100.0.0-dt
        fetchConfig:
          url: "https://gist.githubusercontent.com/richardcase/d85564c8a8a62615b5e75fd98711dd22/raw/81cbe33ddbda98c625ff1b6e7dd286b821487889/control-plane-components.yaml"
        configSecret:
          namespace: capi-variables
          name: capi-variables
        additionalManifests:
          name: controlplane-k3s-additional-manifest
          namespace: capk-system
