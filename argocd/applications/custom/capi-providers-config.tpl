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
    version: v1.9.0
    configSecret:
      namespace: capi-variables
      name: capi-variables
    manifestPatches:
      - |
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          labels:
            cluster.x-k8s.io/provider: cluster-api
            control-plane: controller-manager
          name: capi-controller-manager
          namespace: capi-system
        spec:
          replicas: 1
          selector:
            matchLabels:
              cluster.x-k8s.io/provider: cluster-api
              control-plane: controller-manager
          template:
            metadata:
              labels:
                cluster.x-k8s.io/provider: cluster-api
                control-plane: controller-manager
            spec:
              containers:
                - args:
                    - '--leader-elect'
                    - '--diagnostics-address=:8080'
                    - '--insecure-diagnostics=true'
                    - '--use-deprecated-infra-machine-naming=false'
                    - >-
                      --feature-gates=ClusterResourceSet=true,ClusterTopology=true,MachinePool=true,MachineSetPreflightChecks=true,MachineWaitForVolumeDetachConsiderVolumeAttachments=true,RuntimeSDK=false
                  command:
                    - /manager
                  env:
                    - name: POD_NAMESPACE
                      valueFrom:
                        fieldRef:
                          fieldPath: metadata.namespace
                    - name: POD_NAME
                      valueFrom:
                        fieldRef:
                          fieldPath: metadata.name
                    - name: POD_UID
                      valueFrom:
                        fieldRef:
                          fieldPath: metadata.uid
                  image: registry.k8s.io/cluster-api/cluster-api-controller:v1.9.0
                  imagePullPolicy: IfNotPresent
                  name: manager
                  ports:
                    - containerPort: 9443
                      name: webhook-server
                    - containerPort: 9440
                      name: healthz
                    - containerPort: 8080
                      name: metrics
                  securityContext:
                    allowPrivilegeEscalation: false
                    capabilities:
                      drop:
                        - ALL
                    runAsGroup: 65532
                    runAsUser: 65532
                  volumeMounts:
                    - mountPath: /tmp/k8s-webhook-server/serving-certs
                      name: cert
                      readOnly: true
              securityContext:
                runAsNonRoot: true
                seccompProfile:
                  type: RuntimeDefault
              serviceAccountName: capi-manager
              volumes:
                - name: cert
                  secret:
                    secretName: capi-webhook-service-cert

# https://doc.crds.dev/github.com/kubernetes-sigs/cluster-api-operator/operator.cluster.x-k8s.io/BootstrapProvider/v1alpha2@v0.15.1
bootstrap:
  name: rke2
  namespace: capr-system
  spec:
    version: v0.14.0
    configSecret:
      namespace: capi-variables
      name: capi-variables
    manifestPatches:
      - |
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          labels:
            cluster.x-k8s.io/provider: bootstrap-rke2
            control-plane: controller-manager
          name: rke2-bootstrap-controller-manager
          namespace: capr-system
        spec:
          replicas: 1
          selector:
            matchLabels:
              cluster.x-k8s.io/provider: bootstrap-rke2
              control-plane: controller-manager
          template:
            metadata:
              labels:
                cluster.x-k8s.io/provider: bootstrap-rke2
                control-plane: controller-manager
            spec:
              containers:
                - args:
                    - '--leader-elect'
                    - '--diagnostics-address=:8080'
                    - '--insecure-diagnostics=true'
                    - '--feature-gates=MachinePool=true'
                  command:
                    - /manager
                  image: ghcr.io/rancher/cluster-api-provider-rke2-bootstrap:v0.14.0
                  imagePullPolicy: IfNotPresent
                  name: manager
                  ports:
                    - containerPort: 9443
                      name: webhook-server
                    - containerPort: 9440
                      name: healthz
                    - containerPort: 8080
                      name: metrics
                  securityContext:
                    allowPrivilegeEscalation: false
                    capabilities:
                      drop:
                        - ALL
                    runAsGroup: 65532
                    runAsUser: 65532
                  volumeMounts:
                    - mountPath: /tmp/k8s-webhook-server/serving-certs
                      name: cert
                      readOnly: true
              securityContext:
                runAsNonRoot: true
                seccompProfile:
                  type: RuntimeDefault
              serviceAccountName: rke2-bootstrap-manager
              volumes:
                - name: cert
                  secret:
                    secretName: rke2-bootstrap-webhook-service-cert

# https://doc.crds.dev/github.com/kubernetes-sigs/cluster-api-operator/operator.cluster.x-k8s.io/ControlPlaneProvider/v1alpha2@v0.15.1
controlplane:
  name: rke2
  namespace: capr-system
  spec:
    version: v0.14.0
    configSecret:
      namespace: capi-variables
      name: capi-variables
    manifestPatches:
      - |
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: rke2-control-plane-controller-manager
          namespace: capr-system
        spec:
          template:
            spec:
              containers:
              - name: manager
                args:
                - '--leader-elect'
                - '--diagnostics-address=:8080'
                - '--insecure-diagnostics=true'
                - '--concurrency=5'
                command:
                - /manager
                env:
                - name: POD_NAMESPACE
                  valueFrom:
                    fieldRef:
                      fieldPath: metadata.namespace
                - name: POD_NAME
                  valueFrom:
                    fieldRef:
                      fieldPath: metadata.name
                - name: POD_UID
                  valueFrom:
                    fieldRef:
                      fieldPath: metadata.uid
                image: docker.io/andybavier/cluster-api-provider-rke2-controlplane:latest
                imagePullPolicy: IfNotPresent
                ports:
                - containerPort: 9443
                  name: webhook-server
                - containerPort: 9440
                  name: healthz
                - containerPort: 8080
                  name: metrics
                resources:
                  limits:
                    cpu: 500m
                    memory: 256Mi
                  requests:
                    cpu: 10m
                    memory: 64Mi
                securityContext:
                  allowPrivilegeEscalation: false
                  capabilities:
                    drop:
                    - ALL
                  runAsGroup: 65532
                  runAsUser: 65532
                volumeMounts:
                - mountPath: /tmp/k8s-webhook-server/serving-certs
                  name: cert
                  readOnly: true
              serviceAccountName: rke2-control-plane-manager
              volumes:
              - name: cert
                secret:
                  secretName: rke2-controlplane-webhook-service-cert
