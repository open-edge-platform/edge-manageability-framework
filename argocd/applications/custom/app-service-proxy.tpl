# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

image:
  registry:
    name: {{ .Values.argo.containerRegistryURL }}
    imagePullSecrets:
    {{- with .Values.argo.imagePullSecrets }}
      {{- toYaml . | nindent 6 }}
    {{- end }}

traefik:
  matchRoute: Host(`app-service-proxy.{{ .Values.argo.clusterDomain }}`)
  matchRouteWs: Host(`app-orch.{{ .Values.argo.clusterDomain }}`) && PathPrefix(`/app-service-proxy`)
{{- if .Values.argo.traefik }}
  tlsOption: {{ .Values.argo.traefik.tlsOption | default "" | quote }}
{{- end }}
agent:
{{- if .Values.argo.releaseService.ociRegistry }}
  image:
    registry:
      name: localhost.internal:9443
{{- end}}
  config:
    proxyDomain: "app-service-proxy.{{ .Values.argo.clusterDomain }}"
    proxyServerURL: "wss://app-orch.{{ .Values.argo.clusterDomain }}/app-service-proxy"

git:
{{- if .Values.argo.git.gitServer }}
  server: {{ .Values.argo.git.gitServer }}
{{- else }}
  server: https://gitea.{{ .Values.argo.clusterDomain }}
{{- end }}
  proxy: {{ .Values.argo.git.gitProxy }}
{{- if .Values.argo.git.fleetGitClientSecret }}
  clientSecret: {{ .Values.argo.git.fleetGitClientSecret }}
{{- end }}

{{- with .Values.argo.resources.appServiceProxy }}
resources:
  {{- toYaml . | nindent 2 }}
{{- end }}
