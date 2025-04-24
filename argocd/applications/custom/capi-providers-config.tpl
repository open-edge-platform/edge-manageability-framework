# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# CAPI providers are deployed via CAPI operator and configured via its CRds
# the helmchart .spec is passed unmodified to the CRD .spec

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

# https://doc.crds.dev/github.com/kubernetes-sigs/cluster-api-operator/operator.cluster.x-k8s.io/BootstrapProvider/v1alpha2@v0.15.1
bootstrap:
  name: rke2
  namespace: capr-system
  spec:
    version: v0.14.0
    configSecret:
      namespace: capi-variables
      name: capi-variables

# https://doc.crds.dev/github.com/kubernetes-sigs/cluster-api-operator/operator.cluster.x-k8s.io/ControlPlaneProvider/v1alpha2@v0.15.1
controlplane:
  name: rke2
  namespace: capr-system
  spec:
    version: v0.14.0
    configSecret:
      namespace: capi-variables
      name: capi-variables
# example deployment configuration      
#    deployment:
#      containers:
#      - name: manager
#        imageUrl:  docker.io/andybavier/cluster-api-provider-rke2-controlplane:latest
#        args:
#          "-- concurrency":  "5"