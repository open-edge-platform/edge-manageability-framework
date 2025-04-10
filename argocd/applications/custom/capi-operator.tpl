# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

core: cluster-api:v1.9.0
bootstrap: capr-system:rke2:v0.12.0
controlPlane: capr-system:rke2:v0.12.0
env:
  manager:
  {{- if .Values.argo.proxy }}
  {{- if .Values.argo.proxy.httpProxy }}
  - name: http_proxy
    value: "{{ .Values.argo.proxy.httpProxy }}"
  {{- end }}
  {{- if .Values.argo.proxy.httpsProxy }}
  - name: https_proxy
    value: "{{ .Values.argo.proxy.httpsProxy }}"
  {{- end }}
  {{- if .Values.argo.proxy.noProxy }}
  - name: no_proxy
    value: "{{ .Values.argo.proxy.noProxy }}"
  {{- end }}
  {{- end }}
manager:
  featureGates:
    core:
      MachinePool: "true"
      ClusterResourceSet: "true"
      ClusterTopology: "true"
      RuntimeSDK: "false"
      MachineSetPreflightChecks: "true"
      MachineWaitForVolumeDetachConsiderVolumeAttachments: "true"
  containers:
    - args:
        - '--diagnostics-address=:8080'
        - '--insecure-diagnostics=true'
      ports:
        - containerPort: 8080
          name: metrics
          protocol: TCP
configSecret:
  namespace: capi-variables
  name: capi-variables
containerSecurityContext:
  allowPrivilegeEscalation: false
  capabilities:
    drop:
      - ALL
securityContext:
  seccompProfile:
    type: RuntimeDefault
  runAsNonRoot: true
