# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

image:
  registry: {{.Values.argo.containerRegistryURL }}
  repository: common/component-status
imagePullSecrets:
  {{- with .Values.argo.imagePullSecrets }}
    {{- toYaml . | nindent 2 }}
  {{- end }}

{{- with .Values.argo.resources.componentStatus }}
resources:
  {{- toYaml . | nindent 2 }}
{{- end }}

# Traefik IngressRoute configuration
# Priority must be > 30 to override nexus-api-gw's generic /v route (priority 30)
traefikRoute:
  enabled: true
  matchHost: Host(`api.{{ .Values.argo.clusterDomain }}`)
  matchPath: PathPrefix(`/v1/orchestrator`)
  priority: 40
  namespace: orch-gateway
  secretName: tls-orch
  # Authentication required - component status contains sensitive installation information
  middlewares:
    - validate-jwt
    - secure-headers
{{- if .Values.argo.traefik }}
  tlsOption: {{ .Values.argo.traefik.tlsOption | default "" | quote }}
{{- end }}

# Component status configuration
# This defines which features are installed in this orchestrator deployment
componentStatus:
  schema-version: "1.0"
  orchestrator:
    version: {{ .Values.argo.orchestratorVersion | default "2026.0" | quote }}
    features:
      application-orchestration:
        installed: {{ index .Values.argo.enabled "app-orch-catalog" | default false }}
      cluster-orchestration:
        installed: {{ index .Values.argo.enabled "cluster-manager" | default false }}
      edge-infrastructure-manager:
        installed: {{ or (index .Values.argo.enabled "infra-core") (index .Values.argo.enabled "infra-managers") (index .Values.argo.enabled "infra-onboarding") | default false }}
        inventory:
          installed: {{ index .Values.argo.enabled "infra-core" | default false }}
        out-of-band-management:
          installed: {{ index .Values.argo.enabled "infra-managers" | default false }}
        device-onboarding:
          installed: {{ index .Values.argo.enabled "infra-onboarding" | default false }}
      observability:
        installed: {{ index .Values.argo.enabled "orchestrator-observability" | default false }}
      multitenancy:
        installed: {{ index .Values.argo.enabled "tenancy-manager" | default false }}
