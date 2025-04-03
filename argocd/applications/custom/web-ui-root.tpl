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
      url: "http://localhost:4000"
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
  catalog: "http://localhost:5000"
  appDeploymentManger: "http://localhost:5000"
  appResourceManger: "http://localhost:5000"
  clusterOrch: "http://localhost:5000"
  infraManager: "http://localhost:5000"
  metadataBroker: "http://localhost:5000"
  alertManager: "http://localhost:5000"
  tenantManager: "http://localhost:5000"

{{- with .Values.argo.resources.webUiRoot }}
resources:
  {{- toYaml . | nindent 2 }}
{{- end }}
