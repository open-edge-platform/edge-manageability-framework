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

api:
  serviceArgs:
    enableTracing: {{ index .Values.argo "infra-core" "enableTracing" | default false }}
  {{- if index .Values.argo "infra-core" "api" }}
  {{- if index .Values.argo "infra-core" "api" "resources" }}
  resources:
  {{- with index .Values.argo "infra-core" "api" "resources" }}
    {{- toYaml . | nindent 4 }}
  {{- end}}
  {{- end}}
  {{- end}}
  metrics:
    enabled: {{ index .Values.argo "infra-core" "enableMetrics" | default false }}

inventory:
  miinv:
    enableTracing: {{ index .Values.argo "infra-core" "enableTracing" | default false }}
  postgresql:
    ssl: {{ .Values.argo.database.ssl }}
    secrets: inventory-{{ .Values.argo.database.type }}-postgresql
    # Read only replicas enabled only for Cloud deployment with Aurora
    {{- if eq .Values.argo.database.type "aurora" }}
    # Temporary disable reader replicas also in Cloud deployment
    readOnlyReplicasEnabled: false
    {{- else }}
    readOnlyReplicasEnabled: false
    {{- end }}
    readOnlyReplicasSecrets: inventory-reader-{{ .Values.argo.database.type }}-postgresql
  {{- if index .Values.argo "infra-core" "inventory" }}
  {{- if index .Values.argo "infra-core" "inventory" "resources" }}
  resources:
  {{- with index .Values.argo "infra-core" "inventory" "resources" }}
    {{- toYaml . | nindent 4 }}
  {{- end}}
  {{- end}}
  {{- end}}
  metrics:
    enabled: {{ index .Values.argo "infra-core" "enableMetrics" | default false }}

tenant-controller:
  managerArgs:
    enableTracing: {{ index .Values.argo "infra-core" "enableTracing" | default false }}
{{- if and (index .Values.argo "infra-external") (index .Values.argo "infra-external" "loca") }}
  lenovoConfig:
  {{ $clusterDomain := .Values.argo.clusterDomain }}
  {{ range $idx, $provider := index .Values.argo "infra-external" "loca" "providerConfig" }}
    - {{ $provider | toYaml | nindent 6 }}

      {{ if not $provider.instance_tpl }}
      instance_tpl: {{ printf "%v" "intel{{#}}" }}
      {{ end }}

      {{ if not $provider.dns_domain }}
      dns_domain: {{ $clusterDomain }}
      {{ end }}
  {{- end }}
{{- end }}
  {{- if index .Values.argo "infra-core" "tenant-controller" }}
  {{- if index .Values.argo "infra-core" "tenant-controller" "resources" }}
  resources:
  {{- with index .Values.argo "infra-core" "tenant-controller" "resources" }}
    {{- toYaml . | nindent 4 }}
  {{- end}}
  {{- end}}
  {{- end}}
  metrics:
    enabled: {{ index .Values.argo "infra-core" "enableMetrics" | default false }}

exporter:
  {{- if index .Values.argo "infra-core" "exporter" }}
  {{- if index .Values.argo "infra-core" "exporter" "resources" }}
  resources:
  {{- with index .Values.argo "infra-core" "exporter" "resources" }}
    {{- toYaml . | nindent 4 }}
  {{- end}}
  {{- end}}
  {{- end}}
