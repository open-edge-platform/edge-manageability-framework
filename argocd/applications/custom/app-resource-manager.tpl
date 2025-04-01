# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

config:
  webSocketServer:
    hostName: "vnc.{{ .Values.argo.clusterDomain }}"
    allowedOrigins:
      - "https://{{ .Values.argo.clusterDomain }}"
      - "https://vnc.{{ .Values.argo.clusterDomain }}"
  serviceProxy:
    domainName: "https://app-service-proxy.{{ .Values.argo.clusterDomain }}"
image:
  registry:
    name: {{ .Values.argo.containerRegistryURL }}
    imagePullSecrets:
    {{- with .Values.argo.imagePullSecrets }}
      {{- toYaml . | nindent 6 }}
    {{- end }}
restProxy:
  image:
    registry:
      name: {{ .Values.argo.containerRegistryURL }}
      imagePullSecrets:
      {{- with .Values.argo.imagePullSecrets }}
        {{- toYaml . | nindent 6 }}
      {{- end }}
vncProxy:
  image:
    registry:
      name: {{ .Values.argo.containerRegistryURL }}
      imagePullSecrets:
      {{- with .Values.argo.imagePullSecrets }}
        {{- toYaml . | nindent 6 }}
      {{- end }}
traefikReverseProxy:
  restProxy:
    matchRoute: Host(`app-orch.{{ .Values.argo.clusterDomain }}`)
{{- if .Values.argo.traefik }}
  tlsOption: {{ .Values.argo.traefik.tlsOption | default "" | quote }}
{{- end }}

{{- with .Values.argo.resources.appResourceManager.root }}
resources:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- with .Values.argo.resources.appResourceManager.vncProxyResources }}
vncProxyResources:
  {{- toYaml . | nindent 4 }}
{{- end }}
