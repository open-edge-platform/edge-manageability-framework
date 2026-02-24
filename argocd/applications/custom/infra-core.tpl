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
  credentials:
    enabled: {{ dig "infra-core" "credentials" "enabled" true .Values.argo }}
  api:
    enabled: {{ dig "infra-core" "api" "enabled" true .Values.argo }}
  apiv2:
    enabled: {{ dig "infra-core" "apiv2" "enabled" true .Values.argo }}
  exporter:
    enabled: {{ dig "infra-core" "exporter" "enabled" true .Values.argo }}
  inventory:
    enabled: {{ dig "infra-core" "inventory" "enabled" true .Values.argo }}
  tenant-controller:
    enabled: {{ dig "infra-core" "tenant-controller" "enabled" true .Values.argo }}
  tenant-config:
    enabled: {{ dig "infra-core" "tenant-config" "enabled" false .Values.argo }}

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

apiv2:
  eimScenario: {{ index .Values.argo "infra-core" "eimScenario" | default "fulleim" }}
  serviceArgsProxy:
    globalLogLevel: "debug"
    enableTracing: {{ index .Values.argo "infra-core" "enableTracing" | default false }}
  serviceArgsGrpc:
    globalLogLevel: "debug"
    enableTracing: {{ index .Values.argo "infra-core" "enableTracing" | default false }}
  {{- if index .Values.argo "infra-core" "api" }}
  {{- if index .Values.argo "infra-core" "api" "resources" }}
  resources:
  {{- with index .Values.argo "infra-core" "api" "resources" }}
    {{- toYaml . | nindent 4 }}
  {{- end}}
  {{- end}}
  {{- end}}
  apiv2IngressRoute:
    apiHostname: api.{{ .Values.argo.clusterDomain }}
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
    skipOSProvisioning: {{ index .Values.argo "infra-core" "skipOSProvisioning" | default false }}
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

{{- if index .Values.argo "infra-core" "tenant-config" "enabled" }}
tenant-config:
  config:
    defaultUser: {{ index .Values.argo "infra-core" "tenant-config" "defaultUser" | default "local-admin" }}
    defaultOrganization: {{ index .Values.argo "infra-core" "tenant-config" "defaultOrganization" | default "local-admin" }}
    defaultTenant: {{ index .Values.argo "infra-core" "tenant-config" "defaultTenant" | default "local-admin" }}
{{- end }}
