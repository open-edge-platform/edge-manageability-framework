# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# FIXME: There are references to the same thing in multiple places such as copy-ca-cert
#        Revisit this once the porting is done.
orchSecretName: tls-orch
# internal keycloak JWKS URL should be static but providing a way to modify it here
keycloakJwksUrl: http://platform-keycloak.orch-platform.svc
# internal keycloak JWKS Path should be static but providing a way to modify it here
keycloakJwksPath: /realms/master/protocol/openid-connect/certs
keycloakServicePort: 8080
fleetMatchHost: Host(`fleet.{{ .Values.argo.clusterDomain }}`)
harborOciMatchHost: Host(`registry-oci.{{ .Values.argo.clusterDomain }}`)
observabilityMatchHost: Host(`observability-ui.{{ .Values.argo.clusterDomain }}`)
observabilityAdminMatchHost: Host(`observability-admin.{{ .Values.argo.clusterDomain }}`)
vaultMatchHost: Host(`vault.{{ .Values.argo.clusterDomain }}`)
rootMatchHost: Host(`{{ .Values.argo.clusterDomain }}`)
keycloakMatchHost: Host(`keycloak.{{ .Values.argo.clusterDomain }}`)
connectCSPs:
- "https://keycloak.{{ .Values.argo.clusterDomain }}"
- "wss://vnc.{{ .Values.argo.clusterDomain }}"
- "https://app-service-proxy.{{ .Values.argo.clusterDomain }}"
- "https://app-orch.{{ .Values.argo.clusterDomain }}"
- "https://api.{{ .Values.argo.clusterDomain }}"
- "https://cluster-orch.{{ .Values.argo.clusterDomain }}"
- "https://metadata.{{ .Values.argo.clusterDomain }}"
- "https://alerting-monitor.{{ .Values.argo.clusterDomain }}"
scriptSources:
- "https://app-service-proxy.{{ .Values.argo.clusterDomain }}"
clusterOrchNodeMatchHost: Host(`cluster-orch-node.{{ .Values.argo.clusterDomain }}`)
logsNodeMatchHost: Host(`logs-node.{{ .Values.argo.clusterDomain }}`)
metricsNodeMatchHost: Host(`metrics-node.{{ .Values.argo.clusterDomain }}`)
giteaMatchHost: Host(`gitea.{{ .Values.argo.clusterDomain }}`)

{{- if .Values.argo.traefik }}
{{- if .Values.argo.traefik.rateLimit }}

rateLimit:
  average: {{ .Values.argo.traefik.rateLimit.average | default 0 }}
  period: {{ .Values.argo.traefik.rateLimit.period | default "1s" }}
  burst: {{ .Values.argo.traefik.rateLimit.burst | default 1 }}
  ipStrategyDepth: {{ .Values.argo.traefik.rateLimit.ipStrategyDepth | default 0 }}
  excludedIps: {{ .Values.argo.traefik.rateLimit.excludedIps | default list }}

{{- end}}
tlsOption: {{ .Values.argo.traefik.tlsOption | default "" | quote }}
{{- end}}

{{- if .Values.argo.cors }}
{{- if .Values.argo.cors.enabled }}
cors:
  enabled: {{ .Values.argo.cors.enabled }}
  allowedOrigins:
    - https://{{ .Values.argo.clusterDomain }}
    - https://web-ui.{{ .Values.argo.clusterDomain }}
    - http://localhost:8080
    - http://44.247.62.6:8080
  {{- range $origin := .Values.argo.cors.allowedOrigins }}
    - {{ $origin }}
  {{- end }}
{{- end}}
{{- end}}
