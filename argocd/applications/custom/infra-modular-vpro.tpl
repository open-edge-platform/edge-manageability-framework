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

import:
  tenant-config:
    enabled: {{ index .Values.argo "infra-modular-vpro" "tenant-config" "enabled" }}

# APIv2 Custom Configuration
apiv2:
  serviceArgsProxy:
    globalLogLevel: "debug"
    enableTracing: {{ index .Values.argo "infra-modular-vpro" "enableTracing" | default false }}
  serviceArgsGrpc:
    enableTracing: {{ index .Values.argo "infra-modular-vpro" "enableTracing" | default false }}
  {{- if index .Values.argo "infra-modular-vpro" "apiv2" }}
  {{- if index .Values.argo "infra-modular-vpro" "apiv2" "resources" }}
  resources:
  {{- with index .Values.argo "infra-modular-vpro" "apiv2" "resources" }}
    {{- toYaml . | nindent 4 }}
  {{- end}}
  {{- end}}
  {{- end}}
  metrics:
    enabled: {{ index .Values.argo "infra-modular-vpro" "enableMetrics" | default false }}

# Inventory Custom Configuration
inventory:
  inventory:
    enableTracing: {{ index .Values.argo "infra-modular-vpro" "enableTracing" | default false }}
  {{- if index .Values.argo "infra-modular-vpro" "inventory" }}
  {{- if index .Values.argo "infra-modular-vpro" "inventory" "resources" }}
  resources:
  {{- with index .Values.argo "infra-modular-vpro" "inventory" "resources" }}
    {{- toYaml . | nindent 4 }}
  {{- end}}
  {{- end}}
  {{- end}}
  metrics:
    enabled: {{ index .Values.argo "infra-modular-vpro" "enableMetrics" | default false }}

# MPS Custom Configuration
mps:
  serviceArgs:
    enableTracing: {{ index .Values.argo "infra-modular-vpro" "enableTracing" | default false }}
  {{- if index .Values.argo "infra-modular-vpro" "mps" }}
  {{- if index .Values.argo "infra-modular-vpro" "mps" "resources" }}
  resources:
  {{- with index .Values.argo "infra-modular-vpro" "mps" "resources" }}
    {{- toYaml . | nindent 4 }}
  {{- end}}
  {{- end}}
  {{- end}}
  metrics:
    enabled: {{ index .Values.argo "infra-modular-vpro" "enableMetrics" | default false }}

# RPS Custom Configuration
rps:
  serviceArgs:
    enableTracing: {{ index .Values.argo "infra-modular-vpro" "enableTracing" | default false }}
  {{- if index .Values.argo "infra-modular-vpro" "rps" }}
  {{- if index .Values.argo "infra-modular-vpro" "rps" "resources" }}
  resources:
  {{- with index .Values.argo "infra-modular-vpro" "rps" "resources" }}
    {{- toYaml . | nindent 4 }}
  {{- end}}
  {{- end}}
  {{- end}}
  metrics:
    enabled: {{ index .Values.argo "infra-modular-vpro" "enableMetrics" | default false }}

# DM-Manager Custom Configuration
dm-manager:
  serviceArgs:
    enableTracing: {{ index .Values.argo "infra-modular-vpro" "enableTracing" | default false }}
  {{- if index .Values.argo "infra-modular-vpro" "dm-manager" }}
  {{- if index .Values.argo "infra-modular-vpro" "dm-manager" "resources" }}
  resources:
  {{- with index .Values.argo "infra-modular-vpro" "dm-manager" "resources" }}
    {{- toYaml . | nindent 4 }}
  {{- end}}
  {{- end}}
  {{- end}}
  metrics:
    enabled: {{ index .Values.argo "infra-modular-vpro" "enableMetrics" | default false }}

# Tenant Controller Custom Configuration
tenant-controller:
  {{- if index .Values.argo "infra-modular-vpro" "tenant-controller" }}
  {{- if index .Values.argo "infra-modular-vpro" "tenant-controller" "resources" }}
  resources:
  {{- with index .Values.argo "infra-modular-vpro" "tenant-controller" "resources" }}
    {{- toYaml . | nindent 4 }}
  {{- end}}
  {{- end}}
  {{- end}}
