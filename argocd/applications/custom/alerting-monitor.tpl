# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

{{- if and .Values.argo.o11y .Values.argo.o11y.alertingMonitor .Values.argo.o11y.alertingMonitor.minReplicas }}
minReplicas: {{ .Values.argo.o11y.alertingMonitor.minReplicas | default 2 }}
{{- end }}
{{- if and .Values.argo.o11y .Values.argo.o11y.alertingMonitor .Values.argo.o11y.alertingMonitor.maxReplicas }}
maxReplicas: {{ .Values.argo.o11y.alertingMonitor.maxReplicas | default 5 }}
{{- end }}

image:
  registry: {{.Values.argo.containerRegistryURL}}
management:
  registry: {{.Values.argo.containerRegistryURL}}

imagePullSecrets:
{{- with .Values.argo.imagePullSecrets }}
{{- toYaml . | nindent 2 }}
{{- end }}

{{- if and .Values.argo.o11y .Values.argo.o11y.alertingMonitor .Values.argo.o11y.alertingMonitor.caSecretKey }}
caSecretKey: {{ .Values.argo.o11y.alertingMonitor.caSecretKey }}
{{- end }}

{{- if and .Values.argo.o11y .Values.argo.o11y.alertingMonitor .Values.argo.o11y.alertingMonitor.smtp }}
smtp:
  initialize: {{ index .Values "argo" "o11y" "alertingMonitor" "smtp" "initialize" }}
  configSecret: {{ index .Values "argo" "o11y" "alertingMonitor" "smtp" "configSecret" }}
  userPasswordAuth: {{ ne ( index .Values "argo" "o11y" "alertingMonitor" "smtp" "userPasswordAuth" ) false }}
  {{- if eq (index .Values "argo" "o11y" "alertingMonitor" "smtp" "userPasswordAuth") true }}
  passwordSecret:
    name: {{ index .Values "argo" "o11y" "alertingMonitor" "smtp" "passwordSecret" "name" }}
    key: {{ index .Values "argo" "o11y" "alertingMonitor" "smtp" "passwordSecret" "key" }}
  {{- end }}
  requireTls: {{ ne ( index .Values "argo" "o11y" "alertingMonitor" "smtp" "requireTls" ) false }}
  {{- if eq (index .Values "argo" "o11y" "alertingMonitor" "smtp" "requireTls") true }}
  insecureSkipVerify: {{ index .Values "argo" "o11y" "alertingMonitor" "smtp" "insecureSkipVerify" | default false }}
  {{- end }}
{{- end }}

{{- if and .Values.argo.o11y .Values.argo.o11y.alertingMonitor .Values.argo.o11y.alertingMonitor.initialRules }}
initialRules:
  hostRules: {{ .Values.argo.o11y.alertingMonitor.initialRules.hostRules }}
  appDeploymentRules: {{ .Values.argo.o11y.alertingMonitor.initialRules.appDeploymentRules }}
  clusterRules: {{ .Values.argo.o11y.alertingMonitor.initialRules.clusterRules }}
{{- end }}

authentication:
  oidcServer: https://keycloak.{{ .Values.argo.clusterDomain }}:443
  oidcServerRealm: master
webUIAddress: "https://web-ui.{{ .Values.argo.clusterDomain }}"
observabilityUIAddress: "https://observability-ui.{{ .Values.argo.clusterDomain }}"

traefik:
  matchRoute: Host(`alerting-monitor.{{ .Values.argo.clusterDomain }}`)
{{- if .Values.argo.traefik }}
  tlsOption: {{ .Values.argo.traefik.tlsOption | default "" | quote }}
{{- end }}

database:
  databaseSecret: alerting-{{ .Values.argo.database.type }}-postgresql
  ssl: {{ .Values.argo.database.ssl }}

{{- if .Values.argo.o11y }}
{{- with .Values.argo.o11y.alertingMonitor }}
{{- if or .alertmanager (and .commonConfig .commonConfig.storageClass) }}
alertmanager:
  {{- if and .alertmanager .alertmanager.replicas }}
  replicaCount: {{ .alertmanager.replicas | default 2 }}
  {{- end }}
  {{- if and .commonConfig .commonConfig.storageClass }}
  persistence:
    storageClass: {{ .commonConfig.storageClass }}
  {{- end }}
  {{- with .alertmanager.resources }}
  resources:
    {{- toYaml . | nindent 4 }}
  {{- end }}
  {{- with .alertmanager.configmapReload.resources }}
  configmapReload:
    resources:
      {{- toYaml . | nindent 6 }}
  {{- end }}
{{- end }}
{{- end }}
{{- end }}

multitenantGatewayEnabled: {{ index .Values.argo.enabled "multitenant_gateway" | default false }}
