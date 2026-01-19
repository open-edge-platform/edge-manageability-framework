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
  middlewares:
    - validate-jwt
    - secure-headers
{{- if .Values.argo.traefik }}
  tlsOption: {{ .Values.argo.traefik.tlsOption | default "" | quote }}
{{- end }}

# Component status configuration
# This configuration reflects which features are ACTUALLY installed in the orchestrator
# Detection method - Checks which profile files are loaded in root-app
componentStatus:
  schema-version: "1.0"
  orchestrator:
    version: {{ .Values.argo.orchestratorVersion | default .Chart.Version | quote }}
    features:
      # Application Orchestration - Enabled when app-orch profile is loaded
      # Detection - enable-app-orch.yaml in root-app valueFiles
      application-orchestration:
        installed: {{ index .Values.argo.enabled "app-orch-catalog" | default false }}
      
      # Cluster Orchestration - Enabled when cluster-orch profile is loaded
      # Detection - enable-cluster-orch.yaml in root-app valueFiles
      cluster-orchestration:
        installed: {{ index .Values.argo.enabled "cluster-manager" | default false }}
        
        # Cluster management core API and lifecycle operations
        cluster-management:
          installed: {{ index .Values.argo.enabled "cluster-manager" | default false }}
        
        # CAPI (Cluster API) integration for declarative cluster management
        capi:
          installed: {{ index .Values.argo.enabled "capi-operator" | default false }}
        
        # Infrastructure provider for Intel platforms
        intel-provider:
          installed: {{ index .Values.argo.enabled "intel-infra-provider" | default false }}
      
      # Edge Infrastructure Manager - Enabled when edge-infra profile is loaded
      # Detection - enable-edgeinfra.yaml in root-app valueFiles
      # Profile enables 4 core apps - infra-core, infra-managers, infra-onboarding, infra-external
      # We report the overall feature as installed if ANY infra app is enabled
      # Sub-features represent workflow-level capabilities
      edge-infrastructure-manager:
        installed: {{ or (index .Values.argo.enabled "infra-core") (index .Values.argo.enabled "infra-managers") (index .Values.argo.enabled "infra-onboarding") (index .Values.argo.enabled "infra-external") | default false }}
        
        # Day2 - Day 2 operations (maintenance, updates, troubleshooting)
        # Detection - maintenance-manager is deployed as part of infra-managers
        day2:
          installed: {{ if and (index .Values.argo.enabled "infra-managers") (index .Values.argo "infra-managers" "maintenance-manager") }}true{{ else }}false{{ end }}
        
        # Onboarding - Device discovery, registration, and enrollment workflow
        # Detection - onboarding-manager is enabled in infra-onboarding
        onboarding:
          installed: {{ if and (index .Values.argo.enabled "infra-onboarding") (index .Values.argo "infra-onboarding" "onboarding-manager" "enabled") }}true{{ else }}false{{ end }}
        
        # OOB (Out-of-Band) - vPRO/AMT management capabilities
        # Detection - AMT configuration exists in infra-external (indicates vPRO/AMT managers deployed)
        oob:
          installed: {{ if and (index .Values.argo.enabled "infra-external") (index .Values.argo "infra-external" "amt") }}true{{ else }}false{{ end }}
        
        # Provisioning - Automatic OS provisioning workflow
        # Detection - autoProvision is enabled in infra-managers (os-resource-manager handles automatic provisioning)
        provisioning:
          installed: {{ if and (index .Values.argo.enabled "infra-managers") (index .Values.argo "infra-managers" "autoProvision" "enabled") }}true{{ else }}false{{ end }}
      
      # Observability - Enabled when o11y profile is loaded
      # Detection - enable-o11y.yaml in root-app valueFiles
      observability:
        installed: {{ index .Values.argo.enabled "orchestrator-observability" | default false }}
        
        # Metrics collection and monitoring for orchestrator components
        orchestrator-monitoring:
          installed: {{ index .Values.argo.enabled "orchestrator-observability" | default false }}
        
        # Metrics collection and monitoring for edge nodes
        edge-node-monitoring:
          installed: {{ index .Values.argo.enabled "edgenode-observability" | default false }}
        
        # Pre-built dashboards for orchestrator metrics
        orchestrator-dashboards:
          installed: {{ index .Values.argo.enabled "orchestrator-dashboards" | default false }}
        
        # Pre-built dashboards for edge node metrics
        edge-node-dashboards:
          installed: {{ index .Values.argo.enabled "edgenode-dashboards" | default false }}
        
        # Alerting and monitoring rules
        alerting:
          installed: {{ index .Values.argo.enabled "alerting-monitor" | default false }}
      
      # Web UI - Enabled when full-ui profile is loaded
      # Detection - enable-full-ui.yaml in root-app valueFiles
      web-ui:
        installed: {{ or (index .Values.argo.enabled "web-ui-root") (index .Values.argo.enabled "web-ui-app-orch") (index .Values.argo.enabled "web-ui-cluster-orch") (index .Values.argo.enabled "web-ui-infra") | default false }}
        orchestrator-ui-root:
          installed: {{ index .Values.argo.enabled "web-ui-root" | default false }}
        application-orchestration-ui:
          installed: {{ index .Values.argo.enabled "web-ui-app-orch" | default false }}
        cluster-orchestration-ui:
          installed: {{ index .Values.argo.enabled "web-ui-cluster-orch" | default false }}
        infrastructure-ui:
          installed: {{ index .Values.argo.enabled "web-ui-infra" | default false }}
      
      # Multitenancy - Tenancy services (tenancy-manager, tenancy-api-mapping, tenancy-datamodel)
      # are always deployed as part of root-app, so multitenancy is always enabled
      # The default-tenant-only sub-feature indicates single-tenant mode (when defaultTenancy profile is loaded)
      multitenancy:
        installed: true
        default-tenant-only:
          installed: {{ index .Values.argo.enabled "defaultTenancy" | default false }}
      
      # Kyverno - Policy engine for Kubernetes admission control and governance
      # Detection - enable-kyverno.yaml in root-app valueFiles
      kyverno:
        installed: {{ index .Values.argo.enabled "kyverno" | default false }}
        
        # Kyverno policy engine core
        policy-engine:
          installed: {{ index .Values.argo.enabled "kyverno" | default false }}
        
        # Pre-defined security and governance policies
        policies:
          installed: {{ index .Values.argo.enabled "kyverno-policy" | default false }}
