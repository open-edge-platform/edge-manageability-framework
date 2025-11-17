# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

global:
  registry:
    name: {{ .Values.argo.containerRegistryURL }}
healthcheck:
  registry: {{ .Values.argo.containerRegistryURL }}
image:
    registry: {{ .Values.argo.containerRegistryURL }}
imagePullSecrets:
{{- with .Values.argo.imagePullSecrets }}
{{- toYaml . | nindent 2 }}
{{- end }}
postgres:
  ssl: {{ .Values.argo.database.ssl }}
  secrets: app-orch-catalog-{{ .Values.argo.database.type }}-postgresql
  registry: {{ .Values.argo.containerRegistryURL }}
storage:
  size: {{ .Values.argo.catalog.storageSize }}
traefikReverseProxy:
  matchRoute: Host(`app-orch.{{ .Values.argo.clusterDomain }}`)
{{- if .Values.argo.traefik }}
  tlsOption: {{ .Values.argo.traefik.tlsOption | default "" | quote }}
{{- end }}
openidc:
  external: "https://keycloak.{{ .Values.argo.clusterDomain }}/realms/master"
{{- if .Values.argo.catalog.storageClass }}
storageClassName: {{ .Values.argo.catalog.storageClass }}
{{- end }}
catalogServer:
  # http proxy settings
  {{- if .Values.argo.proxy.httpProxy}}
  httpProxy: "{{ .Values.argo.proxy.httpProxy }}"
  {{- end}}
  {{- if .Values.argo.proxy.httpsProxy}}
  httpsProxy: "{{ .Values.argo.proxy.httpsProxy }}"
  {{- end}}
  {{- if .Values.argo.proxy.noProxy}}
  noProxy: "{{ .Values.argo.proxy.noProxy }}"
  {{- end}}
