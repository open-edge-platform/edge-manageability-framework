# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

global:
  registry:
    name: "{{ .Values.argo.containerRegistryURL }}/"
    {{- $imagePullSecretsLength := len .Values.argo.imagePullSecrets }}
    {{- if eq $imagePullSecretsLength 0 }}
    imagePullSecrets: []
    {{- else }}
    imagePullSecrets:
    {{- with .Values.argo.imagePullSecrets }}
      {{- toYaml . | nindent 6 }}
    {{- end }}
    {{- end }}

{{- if not (and (index .Values.argo "infra-external") (index .Values.argo "infra-external" "loca")) }}
import:
  loca-manager:
    enabled: false
  loca-metadata-manager:
    enabled: false
  loca-credentials:
    enabled: false
  loca-templates-manager:
    enabled: false

{{- else }}

loca-manager:
  providerConfig:
{{- with index .Values.argo "infra-external" "loca" "providerConfig" }}
  {{- toYaml . | nindent 2 }}
{{- end }}
  {{- if index .Values.argo "infra-external" "loca" "loca-manager" }}
  {{- if index .Values.argo "infra-external" "loca" "loca-manager" "resources" }}
  resources:
  {{- with index .Values.argo "infra-external" "loca" "loca-manager" "resources" }}
    {{- toYaml . | nindent 4 }}
  {{- end}}
  {{- end}}
  {{- end}}
  metrics:
    enabled: {{ index .Values.argo "infra-external" "enableMetrics" | default false }}

loca-metadata-manager:
  providerConfig:
{{- with index .Values.argo "infra-external" "loca" "providerConfig" }}
  {{- toYaml . | nindent 2 }}
{{- end }}
  env:
    clusterDomain: {{ .Values.argo.clusterDomain }}
  {{- if index .Values.argo "infra-external" "loca" "loca-metadata-manager" }}
  {{- if index .Values.argo "infra-external" "loca" "loca-metadata-manager" "resources" }}
  resources:
  {{- with index .Values.argo "infra-external" "loca" "loca-metadata-manager" "resources" }}
    {{- toYaml . | nindent 4 }}
  {{- end}}
  {{- end}}
  {{- end}}
  metrics:
    enabled: {{ index .Values.argo "infra-external" "enableMetrics" | default false }}

loca-templates-manager:
  providerConfig:
{{- with index .Values.argo "infra-external" "loca" "providerConfig" }}
  {{- toYaml . | nindent 2 }}
{{- end }}
  proxies:
    http_proxy: {{ .Values.argo.proxy.httpProxy }}
    https_proxy: {{ .Values.argo.proxy.httpsProxy }}
    no_proxy: {{ .Values.argo.proxy.noProxy }}
  config:
    os_password: {{ index .Values.argo "infra-external" "loca" "osPassword" }}
  metrics:
    enabled: {{ index .Values.argo "infra-external" "enableMetrics" | default false }}
  {{- if index .Values.argo "infra-external" "loca" "loca-templates-manager" }}
  {{- if index .Values.argo "infra-external" "loca" "loca-templates-manager" "resources" }}
  resources:
  {{- with index .Values.argo "infra-external" "loca" "loca-templates-manager" "resources" }}
    {{- toYaml . | nindent 4 }}
  {{- end}}
  {{- end}}
  {{- end}}
{{- end }}

amt:
  mps:
    commonName: "mps-node.{{ .Values.argo.clusterDomain }}"
    traefikReverseProxy:
      host:
        cira:
          name: "mps-node.{{ .Values.argo.clusterDomain }}"
        webport: # Define a new name for the other port
          name: "mps-webport-node.{{ .Values.argo.clusterDomain }}" # Define the name for the new port
  {{- if .Values.argo.traefik }}
    tlsOption: {{ .Values.argo.traefik.tlsOption | default "" | quote }}
  {{- end }}

  rps:
    traefikReverseProxy:
      host:
        grpc:
          name: "rps-node.{{ .Values.argo.clusterDomain }}"
        webport: # Define a new name for the other port
          name: "rps-webport-node.{{ .Values.argo.clusterDomain }}" # Define the name for the new port
  {{- if .Values.argo.traefik }}
    tlsOption: {{ .Values.argo.traefik.tlsOption | default "" | quote }}
  {{- end }}
