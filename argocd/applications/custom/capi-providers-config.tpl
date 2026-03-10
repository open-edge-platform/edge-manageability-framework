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
    version: v1.10.7
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
        version: v0.3.0
        fetchConfig:
          url: "https://github.com/k3s-io/cluster-api-k3s/releases/v0.3.0/bootstrap-components.yaml"
        configSecret:
          namespace: capi-variables
          name: capi-variables
        additionalManifests:
          name: bootstrap-k3s-additional-manifest
          namespace: capk-system
        manifestPatches:
          - |
            apiVersion: apps/v1
            kind: Deployment
            metadata:
              name: capi-k3s-bootstrap-controller-manager
              namespace: capk-system
            spec:
              template:
                spec:
                  containers:
                  - name: manager
                    image: ghcr.io/k3s-io/cluster-api-k3s/bootstrap-controller:v0.3.0
                    command: ["/manager"]
                    args:
                      - --metrics-addr=127.0.0.1:8080
                      - --enable-leader-election
                    ports:
                      - containerPort: 9443
                        name: webhook-server
                        protocol: TCP
                    securityContext:
                      allowPrivilegeEscalation: false
                      capabilities:
                        drop:
                          - ALL
                      runAsNonRoot: true
                      seccompProfile:
                        type: RuntimeDefault
                    volumeMounts:
                      - mountPath: /tmp/k8s-webhook-server/serving-certs
                        name: cert
                        readOnly: true
                  - name: kube-rbac-proxy
                    image: quay.io/brancz/kube-rbac-proxy:v0.21.0
                    args:
                      - --secure-listen-address=0.0.0.0:8443
                      - --upstream=http://127.0.0.1:8080/
                      - --v=10
                    ports:
                      - containerPort: 8443
                        name: https
                        protocol: TCP
                    securityContext:
                      allowPrivilegeEscalation: false
                      capabilities:
                        drop:
                          - ALL
                      runAsNonRoot: true
                      seccompProfile:
                        type: RuntimeDefault

controlplane:
  providers:
    - name: k3s
      namespace: capk-system
      spec:
        version: v0.3.0
        fetchConfig:
          url: "https://github.com/k3s-io/cluster-api-k3s/releases/v0.3.0/control-plane-components.yaml"
        configSecret:
          namespace: capi-variables
          name: capi-variables
        additionalManifests:
          name: controlplane-k3s-additional-manifest
          namespace: capk-system
        manifestPatches:
          - |
            apiVersion: apps/v1
            kind: Deployment
            metadata:
              name: capi-k3s-control-plane-controller-manager
              namespace: capk-system
            spec:
              template:
                spec:
                  containers:
                  - name: manager
                    image: ghcr.io/k3s-io/cluster-api-k3s/controlplane-controller:v0.3.0
                    command: ["/manager"]
                    args:
                      - --metrics-addr=127.0.0.1:8080
                      - --enable-leader-election
                    ports:
                      - containerPort: 9443
                        name: webhook-server
                        protocol: TCP
                    securityContext:
                      allowPrivilegeEscalation: false
                      capabilities:
                        drop:
                          - ALL
                      runAsNonRoot: true
                      seccompProfile:
                        type: RuntimeDefault
                    volumeMounts:
                      - mountPath: /tmp/k8s-webhook-server/serving-certs
                        name: cert
                        readOnly: true
                  - name: kube-rbac-proxy
                    image: quay.io/brancz/kube-rbac-proxy:v0.21.0
                    args:
                      - --secure-listen-address=0.0.0.0:8443
                      - --upstream=http://127.0.0.1:8080/
                      - --v=10
                    ports:
                      - containerPort: 8443
                        name: https
                        protocol: TCP
                    securityContext:
                      allowPrivilegeEscalation: false
                      capabilities:
                        drop:
                          - ALL
                      runAsNonRoot: true
                      seccompProfile:
                        type: RuntimeDefault

