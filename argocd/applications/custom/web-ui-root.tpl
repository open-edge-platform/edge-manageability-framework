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
# Must match the deployment conditions in web-ui-*.yaml templates  
# These control both runtime config (MFE.APP_ORCH) and nginx proxy configuration
mfe:
  app_orch: {{ and (index .Values.argo.enabled "web-ui-app-orch" | default false) (index .Values.argo.enabled "app-orch-catalog" | default false) }}
  cluster_orch: {{ and (index .Values.argo.enabled "web-ui-cluster-orch" | default false) (or (index .Values.argo.enabled "cluster-manager") (index .Values.argo.enabled "capi-operator") (index .Values.argo.enabled "intel-infra-provider") | default false) }}
  infra: {{ and (index .Values.argo.enabled "web-ui-infra" | default false) (or (index .Values.argo.enabled "infra-manager") (index .Values.argo.enabled "infra-operator") (index .Values.argo.enabled "tinkerbell") (index .Values.argo.enabled "infra-onboarding") (index .Values.argo.enabled "maintenance-manager") | default false) }}
  admin: {{ index .Values.argo.enabled "web-ui-admin" | default false }}

# Nginx rewrites - only include proxy_pass for deployed services
nginx:
  rewrites:
  {{- if and (index .Values.argo.enabled "web-ui-app-orch" | default false) (index .Values.argo.enabled "app-orch-catalog" | default false) }}
    - location: "/mfe/applications"
      rewrite:
        source: "/mfe/applications/(.*)"
        dest: "/$1"
      proxy_pass: "http://{{ .Release.Name }}-app-orch.{{ .Release.Namespace }}.svc:80"
  {{- end }}
  {{- if and (index .Values.argo.enabled "web-ui-cluster-orch" | default false) (or (index .Values.argo.enabled "cluster-manager") (index .Values.argo.enabled "capi-operator") (index .Values.argo.enabled "intel-infra-provider") | default false) }}
    - location: "/mfe/cluster-orch"
      rewrite:
        source: "/mfe/cluster-orch/(.*)"
        dest: "/$1"
      proxy_pass: "http://{{ .Release.Name }}-cluster-orch.{{ .Release.Namespace }}.svc:80"
  {{- end }}
  {{- if and (index .Values.argo.enabled "web-ui-infra" | default false) (or (index .Values.argo.enabled "infra-manager") (index .Values.argo.enabled "infra-operator") (index .Values.argo.enabled "tinkerbell") (index .Values.argo.enabled "infra-onboarding") (index .Values.argo.enabled "maintenance-manager") | default false) }}
    - location: "/mfe/infrastructure"
      rewrite:
        source: "/mfe/infrastructure/(.*)"
        dest: "/$1"
      proxy_pass: "http://{{ .Release.Name }}-infra.{{ .Release.Namespace }}.svc:80"
  {{- end }}
  {{- if index .Values.argo.enabled "web-ui-admin" | default false }}
    - location: "/mfe/admin"
      rewrite:
        source: "/mfe/admin/(.*)"
        dest: "/$1"
      proxy_pass: "http://{{ .Release.Name }}-admin.{{ .Release.Namespace }}.svc:80"
  {{- end }}

{{- with .Values.argo.resources.webUiRoot }}
resources:
  {{- toYaml . | nindent 2 }}
{{- end }}
