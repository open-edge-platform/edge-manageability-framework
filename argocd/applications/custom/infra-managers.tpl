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

host-manager:
  serviceArgs:
    enableTracing: {{ index .Values.argo "infra-managers" "enableTracing" | default false }}
  traefikReverseProxy:
    host:
      grpc:
        name: Host(`infra-node.{{ .Values.argo.clusterDomain }}`)
{{- if .Values.argo.traefik }}
    tlsOption: {{ .Values.argo.traefik.tlsOption | default "" | quote }}
{{- end }}
  {{- if index .Values.argo "infra-managers" "host-manager" }}
  {{- if index .Values.argo "infra-managers" "host-manager" "resources" }}
  resources:
  {{- with index .Values.argo "infra-managers" "host-manager" "resources" }}
    {{- toYaml . | nindent 4 }}
  {{- end}}
  {{- end}}
  {{- end}}
  metrics:
    enabled: {{ index .Values.argo "infra-managers" "enableMetrics" | default false }}

maintenance-manager:
  mimaintmgr:
    enableTracing: {{ index .Values.argo "infra-managers" "enableTracing" | default false }}
  traefikReverseProxy:
    host:
      grpc:
        name: Host(`update-node.{{ .Values.argo.clusterDomain }}`)
{{- if .Values.argo.traefik }}
    tlsOption: {{ .Values.argo.traefik.tlsOption | default "" | quote }}
{{- end }}
  {{- if index .Values.argo "infra-managers" "maintenance-manager" }}
  {{- if index .Values.argo "infra-managers" "maintenance-manager" "resources" }}
  resources:
  {{- with index .Values.argo "infra-managers" "maintenance-manager" "resources" }}
    {{- toYaml . | nindent 4 }}
  {{- end}}
  {{- end}}
  {{- end}}
  metrics:
    enabled: {{ index .Values.argo "infra-managers" "enableMetrics" | default false }}

networking-manager:
  serviceArgs:
    enableTracing: {{ index .Values.argo "infra-managers" "enableTracing" | default false }}
  {{- if index .Values.argo "infra-managers" "networking-manager" }}
  {{- if index .Values.argo "infra-managers" "networking-manager" "resources" }}
  resources:
  {{- with index .Values.argo "infra-managers" "networking-manager" "resources" }}
    {{- toYaml . | nindent 4 }}
  {{- end}}
  {{- end}}
  {{- end}}
  metrics:
    enabled: {{ index .Values.argo "infra-managers" "enableMetrics" | default false }}

telemetry-manager:
  telemetryMgrArgs:
    enableTracing: {{ index .Values.argo "infra-managers" "enableTracing" | default false }}
  metrics:
    enabled: {{ index .Values.argo "infra-managers" "enableMetrics" | default false }}
  traefikReverseProxy:
    host:
      grpc:
        name: Host(`telemetry-node.{{ .Values.argo.clusterDomain }}`)
{{- if .Values.argo.traefik }}
    tlsOption: {{ .Values.argo.traefik.tlsOption | default "" | quote }}
{{- end }}
  {{- if index .Values.argo "infra-managers" "telemetry-manager" }}
  {{- if index .Values.argo "infra-managers" "telemetry-manager" "resources" }}
  resources:
  {{- with index .Values.argo "infra-managers" "telemetry-manager" "resources" }}
    {{- toYaml . | nindent 4 }}
  {{- end}}
  {{- end}}
  {{- end}}

os-resource-manager:
  autoProvision:
{{- if and (index .Values.argo "infra-external") (index .Values.argo "infra-external" "loca") }}
    enabled: false
{{- else if index .Values.argo "infra-managers" "autoProvision" }}
    enabled: {{ index .Values.argo "infra-managers" "autoProvision" "enabled" }}
    defaultProfile: {{ index .Values.argo "infra-managers" "autoProvision" "defaultProfile" | default "" }}
{{- end }}
  global:
    proxies:
      http_proxy: {{ .Values.argo.proxy.httpProxy }}
      https_proxy: {{ .Values.argo.proxy.httpsProxy }}
      no_proxy: {{ .Values.argo.proxy.noProxy }}
  managerArgs:
    enableTracing: {{ index .Values.argo "infra-managers" "enableTracing" | default false }}
    manualMode: {{ index .Values.argo "infra-managers" "os-resource-manager-manual-mode" | default false }}
    {{- if index .Values.argo "infra-managers" "os-resource-manager" }}
    {{- if index .Values.argo "infra-managers" "os-resource-manager" "enabledProfiles" }}
    enabledProfiles:
    {{- with index .Values.argo "infra-managers" "os-resource-manager" "enabledProfiles" }}
      {{- toYaml . | nindent 4 }}
    {{- end}}
    {{- end}}
    {{- end}}
  metrics:
    enabled: {{ index .Values.argo "infra-managers" "enableMetrics" | default false }}
  {{- if index .Values.argo "infra-managers" "os-resource-manager" }}
  {{- if index .Values.argo "infra-managers" "os-resource-manager" "resources" }}
  resources:
  {{- with index .Values.argo "infra-managers" "os-resource-manager" "resources" }}
    {{- toYaml . | nindent 4 }}
  {{- end}}
  {{- end}}
  {{- end}}

attestationstatus-manager:
  serviceArgs:
    enableTracing: {{ index .Values.argo "infra-managers" "enableTracing" | default false }}
  traefikReverseProxy:
    host:
      grpc:
        name: Host(`attest-node.{{ .Values.argo.clusterDomain }}`)
{{- if .Values.argo.traefik }}
    tlsOption: {{ .Values.argo.traefik.tlsOption | default "" | quote }}
{{- end }}
  {{- if index .Values.argo "infra-managers" "attestationstatus-manager" }}
  {{- if index .Values.argo "infra-managers" "attestationstatus-manager" "resources" }}
  resources:
  {{- with index .Values.argo "infra-managers" "attestationstatus-manager" "resources" }}
    {{- toYaml . | nindent 4 }}
  {{- end}}
  {{- end}}
  {{- end}}
  metrics:
    enabled: {{ index .Values.argo "infra-managers" "enableMetrics" | default false }}
