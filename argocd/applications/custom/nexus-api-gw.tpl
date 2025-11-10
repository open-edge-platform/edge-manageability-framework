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

# Keycloak OIDC server URL based on clusterDomain
{{- if or (contains "kind.internal" .Values.argo.clusterDomain) (contains "localhost" .Values.argo.clusterDomain) (eq .Values.argo.clusterDomain "") }}
oidc_server_url: "http://platform-keycloak.orch-platform.svc:8080/realms/master"
{{- else }}
oidc_server_url: "https://keycloak.{{ .Values.argo.clusterDomain }}/realms/master"
{{- end }}
