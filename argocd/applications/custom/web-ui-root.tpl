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
  rps: "https://api.{{ .Values.argo.clusterDomain }}"
  mps: "https://api.{{ .Values.argo.clusterDomain }}"

# MFE Configuration - Controls which UI components are loaded
# Based on whether parent features (AO, CO, EIM) are enabled
mfe:
  enableAppOrch: {{ and (index .Values.argo.enabled "web-ui-app-orch" | default false) (index .Values.argo.enabled "app-orch-catalog" | default false) }}
  enableClusterOrch: {{ and (index .Values.argo.enabled "web-ui-cluster-orch" | default false) (or (index .Values.argo.enabled "cluster-manager") (index .Values.argo.enabled "capi-operator") (index .Values.argo.enabled "intel-infra-provider") | default false) }}
  enableInfra: {{ and (index .Values.argo.enabled "web-ui-infra" | default false) (or (index .Values.argo.enabled "infra-manager") (index .Values.argo.enabled "infra-operator") (index .Values.argo.enabled "tinkerbell") (index .Values.argo.enabled "infra-onboarding") (index .Values.argo.enabled "maintenance-manager") | default false) }}
  enableAdmin: {{ index .Values.argo.enabled "web-ui-admin" | default false }}

{{- with .Values.argo.resources.webUiRoot }}
resources:
  {{- toYaml . | nindent 2 }}
{{- end }}
