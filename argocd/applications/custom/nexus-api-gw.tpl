# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

image:
  registry: {{.Values.argo.containerRegistryURL }}
  repository: common/nexus-api-gw
imagePullSecrets:
  {{- with .Values.argo.imagePullSecrets }}
    {{- toYaml . | nindent 2 }}
  {{- end }}
traefikReverseProxy:
  host:
    grpc:
      name: "api.{{ .Values.argo.clusterDomain }}"
  tlsOption: {{ .Values.argo.traefik.tlsOption | default "" | quote }}

{{- with .Values.argo.resources.nexusApiGw }}
resources:
  {{- toYaml . | nindent 2 }}
{{- end }}

{{- if eq (index .Values "argo" "enabled" "audit") true }}
logging:
  level: info
{{- end }}

# Keycloak OIDC server URL - always use external domain to match Keycloak's configured hostname
oidc:
  oidc_server_url: "https://keycloak.{{ .Values.argo.clusterDomain }}/realms/master"
