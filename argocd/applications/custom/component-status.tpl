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
  matchHost: "Host(`api.{{ .Values.argo.clusterDomain }}`)"
  matchPath: "PathPrefix(`/v1/orchestrator`)"
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
        installed: {{ or (index .Values.argo.enabled "cluster-manager") (index .Values.argo.enabled "capi-operator") (index .Values.argo.enabled "intel-infra-provider") | default false }}
        
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
      # Two deployment profiles: vPRO (with AMT/OOB) and OXM (microvisor-based)
      # Sub-features represent actual user-facing workflows
      edge-infrastructure-manager:
        installed: {{ or (index .Values.argo.enabled "infra-core") (index .Values.argo.enabled "infra-managers") (index .Values.argo.enabled "infra-onboarding") (index .Values.argo.enabled "infra-external") | default false }}
        
        # Onboarding - Device discovery, registration, and enrollment workflow
        # Detection - onboarding-manager is configured and enabled in infra-onboarding
        # Available in both vPRO and OXM profiles
        onboarding:
          installed: {{ if hasKey .Values.argo "infra-onboarding" }}{{ $infraOnboarding := index .Values.argo "infra-onboarding" }}{{ if hasKey $infraOnboarding "onboarding-manager" }}{{ $onboardingMgr := index $infraOnboarding "onboarding-manager" }}{{ $onboardingMgr.enabled | default false }}{{ else }}false{{ end }}{{ else }}false{{ end }}
        
        # OOB (Out-of-Band) - vPRO/AMT management capabilities
        # Detection - infra-external enabled in argo.enabled (OXM profile sets this to false)
        # Only available in vPRO profile, not in OXM (microvisor) profile
        oob:
          installed: {{ if hasKey .Values.argo.enabled "infra-external" }}{{ index .Values.argo.enabled "infra-external" | default false }}{{ else }}false{{ end }}
        
        # Provisioning - OS provisioning workflow capability
        # Detection - provisioning available when infra-onboarding is deployed
        # Available in both vPRO (standard OS) and OXM (microvisor) profiles
        provisioning:
          installed: {{ index .Values.argo.enabled "infra-onboarding" | default false }}
      
      # Orchestrator Observability - Metrics and monitoring for orchestrator platform components
      # Detection - orchestrator-observability application enabled
      orchestrator-observability:
        installed: {{ or (index .Values.argo.enabled "orchestrator-observability") (index .Values.argo.enabled "orchestrator-dashboards") (index .Values.argo.enabled "alerting-monitor") | default false }}
        
        # Metrics collection and monitoring for orchestrator components
        monitoring:
          installed: {{ index .Values.argo.enabled "orchestrator-observability" | default false }}
        
        # Pre-built dashboards for orchestrator metrics
        dashboards:
          installed: {{ index .Values.argo.enabled "orchestrator-dashboards" | default false }}
        
        # Alerting and monitoring rules for orchestrator
        alerting:
          installed: {{ index .Values.argo.enabled "alerting-monitor" | default false }}
      
      # Edge Node Observability - Metrics and monitoring for edge nodes
      # Detection - edgenode-observability application enabled
      edgenode-observability:
        installed: {{ or (index .Values.argo.enabled "edgenode-observability") (index .Values.argo.enabled "edgenode-dashboards") | default false }}
        
        # Metrics collection and monitoring for edge nodes
        monitoring:
          installed: {{ index .Values.argo.enabled "edgenode-observability" | default false }}
        
        # Pre-built dashboards for edge node metrics
        dashboards:
          installed: {{ index .Values.argo.enabled "edgenode-dashboards" | default false }}
      
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
