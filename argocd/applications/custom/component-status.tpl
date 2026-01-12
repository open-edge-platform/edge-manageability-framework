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
# This configuration reflects which features are ACTUALLY installed in the orchestrator
# Detection method: Checks which profile files are loaded in root-app (source of truth)
componentStatus:
  schema-version: "1.0"
  orchestrator:
    version: {{ .Values.argo.orchestratorVersion | default .Chart.Version | quote }}
    features:
      # Application Orchestration: Enabled when app-orch profile is loaded
      # Detection: enable-app-orch.yaml in root-app valueFiles
      application-orchestration:
        installed: {{ index .Values.argo.enabled "app-orch-catalog" | default false }}
      
      # Cluster Orchestration: Enabled when cluster-orch profile is loaded
      # Detection: enable-cluster-orch.yaml in root-app valueFiles
      cluster-orchestration:
        installed: {{ index .Values.argo.enabled "cluster-manager" | default false }}
      
      # Edge Infrastructure Manager: Enabled when edgeinfra profile is loaded
      # Detection: enable-edgeinfra.yaml in root-app valueFiles
      # Profile enables 4 core apps: infra-core, infra-managers, infra-onboarding, infra-external
      # We report the overall feature as installed if ANY infra app is enabled
      edge-infrastructure-manager:
        installed: {{ or (index .Values.argo.enabled "infra-core") (index .Values.argo.enabled "infra-managers") (index .Values.argo.enabled "infra-onboarding") (index .Values.argo.enabled "infra-external") | default false }}
        infra-core:
          installed: {{ index .Values.argo.enabled "infra-core" | default false }}
        infra-manager:
          installed: {{ index .Values.argo.enabled "infra-managers" | default false }}
        device-provisioning:
          installed: {{ index .Values.argo.enabled "infra-onboarding" | default false }}
      
      # Observability: Enabled when o11y profile is loaded
      # Detection: enable-o11y.yaml in root-app valueFiles
      observability:
        installed: {{ index .Values.argo.enabled "orchestrator-observability" | default false }}
      
      # Web UI: Enabled when full-ui profile is loaded
      # Detection: enable-full-ui.yaml in root-app valueFiles
      web-ui:
        installed: {{ or (index .Values.argo.enabled "web-ui-root") (index .Values.argo.enabled "web-ui-app-orch") (index .Values.argo.enabled "web-ui-cluster-orch") (index .Values.argo.enabled "web-ui-infra") | default false }}
        orchestrator-ui:
          installed: {{ index .Values.argo.enabled "web-ui-root" | default false }}
        application-orchestration-ui:
          installed: {{ index .Values.argo.enabled "web-ui-app-orch" | default false }}
        cluster-orchestration-ui:
          installed: {{ index .Values.argo.enabled "web-ui-cluster-orch" | default false }}
        infrastructure-ui:
          installed: {{ index .Values.argo.enabled "web-ui-infra" | default false }}
      
      # Multitenancy: Tenancy services are part of the platform
      multitenancy:
        installed: {{ index .Values.argo.enabled "tenancy-manager" | default false }}
        default-tenant-only:
          installed: {{ index .Values.argo.enabled "defaultTenancy" | default false }}
      
      # Detection: enable-kyverno.yaml in root-app valueFiles
      kyverno:
        installed: {{ index .Values.argo.enabled "kyverno" | default false }}
