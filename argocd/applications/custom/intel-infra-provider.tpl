# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

traefikReverseProxy:
  enabled: true
  host:
    grpc:
      name: "cluster-orch-node.{{ .Values.argo.clusterDomain }}"
      secretName: tls-orch
{{- if .Values.argo.traefik }}
      tlsOption: {{ .Values.argo.traefik.tlsOption | default "" | quote }}
{{- end }}

manager:
  image:
    repository: registry-rs.edgeorchestration.intel.com/edge-orch/cluster/capi-provider-intel-manager
southboundApi:
  image:
    repository: registry-rs.edgeorchestration.intel.com/edge-orch/cluster/capi-provider-intel-southbound