# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Cluster specific values applied to root-app only
root:
  useLocalValues: false
  clusterValues:
    - orch-configs/profiles/enable-platform.yaml
    - orch-configs/profiles/enable-multitenancy.yaml
{{- if .Values.enableObservability }}
    - orch-configs/profiles/enable-o11y.yaml
{{- end }}
{{- if .Values.enableKyverno }}
    - orch-configs/profiles/enable-kyverno.yaml
{{- end }}
    - orch-configs/profiles/enable-app-orch.yaml
    - orch-configs/profiles/enable-cluster-orch.yaml
{{- if .Values.enableEdgeInfra }}
    - orch-configs/profiles/enable-edgeinfra.yaml
{{- end }}
    - orch-configs/profiles/enable-full-ui.yaml
{{- if .Values.enableUiDev }}
    - orch-configs/profiles/ui-dev.yaml
{{- end }}
    - orch-configs/profiles/enable-dev.yaml
{{- if .Values.enableObservability }}
    - orch-configs/profiles/enable-sre.yaml
{{- end }}
{{- if .Values.enableAutoProvision }}
    - orch-configs/profiles/enable-autoprovision.yaml
{{- end }}
    # proxy group should be specified as the first post-"enable" profile
{{- if (not (eq .Values.proxyProfile "" )) }}
    - orch-configs/profiles/proxy-{{ .Values.name }}.yaml
{{- else }}
    - orch-configs/profiles/proxy-none.yaml
{{- end }}
    - orch-configs/profiles/profile-{{ .Values.deployProfile }}.yaml
{{- if .Values.enableAutoCert }}
    - orch-configs/profiles/profile-autocert.yaml
{{- end }}
    - orch-configs/profiles/artifact-rs-production-noauth.yaml
{{- if .Values.enableObservability }}
    - orch-configs/profiles/o11y-dev.yaml
    - orch-configs/profiles/alerting-emails-dev.yaml
{{- end }}
{{- if .Values.enableSquid }}
    - orch-configs/profiles/enable-explicit-proxy.yaml
{{- end }}
    - orch-configs/profiles/resource-default.yaml
    - orch-configs/clusters/{{ .Values.name }}.yaml
    # # rate limit is applicable to each cluster.
    # # please see https://doc.traefik.io/traefik/middlewares/http/ratelimit/
    # # if you enable default traefik rate limit, do not specify custom rate limit
{{- if .Values.enableDefaultTraefikRateLimit }}
    - orch-configs/profiles/default-traefik-rate-limit.yaml
{{- end }}


# Values applied to both root app and shared among all child apps
argo:
  ## Basic cluster information
  project: {{ .Values.id }}
  namespace: {{ .Values.id }}
  clusterName: {{ .Values.id }}
  # Base domain name for all Orchestrator services. This base domain will be concatenated with a service's subdomain
  # name to produce the service's domain name. For example, given the domain name of `orchestrator.io`, the Web UI
  # service will be accessible via `web-ui.orchestrator.io`. Not to be confused with the K8s cluster domain.
  clusterDomain: kind.internal

{{- if and .Values.enableAutocert .Values.staging }}
  autoCert:
    production: false
{{- end }}

{{- if .Values.nameServers }}
  infra-onboarding:
    nameservers:
{{- range .Values.nameServers }}
      - {{ . }}
{{- end }}
{{- end }}

  ## Argo CD configs
  deployRepoURL: "https://gitea-http.gitea.svc.cluster.local/argocd/edge-manageability-framework"
  deployRepoRevision: main

  targetServer: "https://kubernetes.default.svc"
  autosync: true
{{ if .Values.enableObservability }}
  o11y:
    sre:
      customerLabel: local
{{- if and .Values.enableCoder }}
      externalSecretsEnabled: true
      providerSecretName: sre-secret
{{- end }}
{{- end }}
{{ if or .Values.enableAsm .Values.enableCoder }}
  aws: {}
    # Account ID and region will be set by deploy.go
    # region: ""
    # account: ""
{{- end }}
  # # rate limit is applicable to each cluster.
  # # please see https://doc.traefik.io/traefik/middlewares/http/ratelimit/
  # # if you specify custom traefik rate limit, do not enable the default one.
  # traefik:
  #   rateLimit:
  #     # When rateLimit section is not specified or average is set to 0 (default setting), rate limiting will be disabled.
  #     average: 5
  #     # period, in combination with average, defines the actual maximum rate: average / period
  #     period: 1m
  #     # burst is the maximum number of requests allowed to go through in the same arbitrarily small period of time.
  #     burst: 20
  #     # If depth is specified, excludedIPs is ignored.
  #     ipStrategyDepth: 1
  #     # Contrary to what the name might suggest, this option is not about excluding an IP from the rate limiter,
  #     # and therefore cannot be used to deactivate rate limiting for some IPs.
  #     excludedIps:
  #       - 10.244.0.1
{{- if .Values.traefik }}
  traefik:
{{ toYaml .Values.traefik | indent 4 }}
{{- end }}

orchestratorDeployment:
  targetCluster: {{ .Values.targetCluster }}
  enableMailpit: {{ .Values.enableMailpit }}
  dockerCache: "{{ .Values.dockerCache }}"
{{- if and .Values.dockerCacheCert }}  
  dockerCacheCert: |
{{ .Values.dockerCacheCert | indent 4 }}
{{- end }}

# Post custom template overwrite values should go to /root-app/environments/<env>/<appName>.yaml
# This is a placeholder to prevent error when there isn't any overwrite needed
{{- if .Values.enableTraefikLogs }}
postCustomTemplateOverwrite:
  traefik:
    logs:
      general:
        level: DEBUG
{{- else}}
postCustomTemplateOverwrite: {}
{{- end }}
