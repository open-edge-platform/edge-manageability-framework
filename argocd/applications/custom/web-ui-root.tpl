# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

global:
  registry:
    name: {{ .Values.argo.containerRegistryURL }}
    imagePullSecrets:
    {{- with .Values.argo.imagePullSecrets }}
      {{- toYaml . | nindent 6 }}
    {{- end }}
  auth:
    keycloak:
      url: "https://keycloak.{{ .Values.argo.clusterDomain }}"
  observability:
    url: "https://observability-ui.{{ .Values.argo.clusterDomain }}"

# Doc service URL
header:
  documentationUrl: "https://edc.intel.com/content/www/us/en/secure/design/confidential/tools/edge-orchestrator/"

# Ingress
service:
  traefik:
    hostname: "Host(`web-ui.{{ .Values.argo.clusterDomain }}`)"
    baseHostname: "Host(`{{ .Values.argo.clusterDomain }}`)"
{{- if .Values.argo.traefik }}
    options:
      name: {{ .Values.argo.traefik.tlsOption | default "" | quote }}
{{- end }}

versions:
  orchestrator: {{ .Values.argo.orchestratorVersion | default .Chart.Version }}

api:
  catalog: "https://api.{{ .Values.argo.clusterDomain }}"
  appDeploymentManger: "https://api.{{ .Values.argo.clusterDomain }}"
  appResourceManger: "https://api.{{ .Values.argo.clusterDomain }}"
  clusterOrch: "https://api.{{ .Values.argo.clusterDomain }}"
  infraManager: "https://api.{{ .Values.argo.clusterDomain }}"
  metadataBroker: "https://api.{{ .Values.argo.clusterDomain }}"
  alertManager: "https://api.{{ .Values.argo.clusterDomain }}"
  tenantManager: "https://api.{{ .Values.argo.clusterDomain }}"

{{- with .Values.argo.resources.webUiRoot }}
resources:
  {{- toYaml . | nindent 2 }}
{{- end }}
