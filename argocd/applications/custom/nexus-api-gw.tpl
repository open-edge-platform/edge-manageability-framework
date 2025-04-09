# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

image:
  registry: {{.Values.argo.containerRegistryURL }}
  repository: common/nexus-api-gw
imagePullSecrets:
  {{- with .Values.argo.imagePullSecrets }}
    {{- toYaml . | nindent 2 }}
  {{- end }}
traefikReverseProxy:
  host:
    grpc:
      name: "api.{{ .Values.argo.clusterDomain }}"
  tlsOption: {{ .Values.argo.traefik.tlsOption | default "" | quote }}

{{- with .Values.argo.resources.nexusApiGw }}
resources:
  {{- toYaml . | nindent 2 }}
{{- end }}

{{- if eq (index .Values "argo" "enable" "audit") true }}
logging:
  level: info
{{- end }}
