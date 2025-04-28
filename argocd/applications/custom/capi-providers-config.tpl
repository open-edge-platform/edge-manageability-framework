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
    additionalManifests:
      name: core-additional-manifest
      namespace: capi-system


# https://doc.crds.dev/github.com/kubernetes-sigs/cluster-api-operator/operator.cluster.x-k8s.io/BootstrapProvider/v1alpha2@v0.15.1
bootstrap:
  name: rke2
  namespace: capr-system
  spec:
    version: v0.14.0
    configSecret:
      namespace: capi-variables
      name: capi-variables
    deployment:
      containers:
      - name: manager
        args:
          "--insecure-diagnostics": "true"
    additionalManifests:
      name: bootstrap-additional-manifest
      namespace: capr-system

# https://doc.crds.dev/github.com/kubernetes-sigs/cluster-api-operator/operator.cluster.x-k8s.io/ControlPlaneProvider/v1alpha2@v0.15.1
controlplane:
  name: rke2
  namespace: capr-system
  spec:
    version: v0.14.0
    configSecret:
      namespace: capi-variables
      name: capi-variables
    deployment:
      containers:
      - name: manager
        args:
          "--insecure-diagnostics": "true"
    additionalManifests:
      name: controlplane-additional-manifest
      namespace: capr-system
# example deployment configuration      
#    deployment:
#      containers:
#      - name: manager
#        imageUrl:  docker.io/user/patched-rke2-controlplane:latest
#        args:
#          "-- concurrency":  "5"

