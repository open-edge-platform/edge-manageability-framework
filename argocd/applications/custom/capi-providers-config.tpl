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
  # k3s providers are installed by the Argo app capi-k3s-manifests.
  # cluster-api-operator currently rejects gist URLs for fetchConfig.
  providers: []

controlplane:
  providers: []
