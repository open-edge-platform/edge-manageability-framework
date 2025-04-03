# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Auth
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
  documentationUrl: "https://edc.intel.com/content/www/us/en/secure/design/confidential/tools/edge-infrastructure-manager/"

# Ingress
service:
  traefik:
    hostname: "Host(`web-ui.{{ .Values.argo.clusterDomain }}`)"
    baseHostname: "Host(`{{ .Values.argo.clusterDomain }}`)"
    options:
      name: {{ .Values.argo.traefik.tlsOption | default "" | quote }}
    enabled: {{ .Values.argo.ui.ingressInfraUi }}

versions:
  orchestrator: {{ .Values.argo.orchestratorVersion | default .Chart.Version }}

{{- with .Values.argo.resources.webUiInfra }}
resources:
  {{- toYaml . | nindent 2 }}
{{- end }}
