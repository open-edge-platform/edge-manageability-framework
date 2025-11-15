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

# Keycloak OIDC server URL - use internal service URL to avoid unnecessary Traefik load
# Backend services access Keycloak directly via cluster DNS, bypassing Traefik
oidc:
  oidc_server_url: "http://platform-keycloak.keycloak-system.svc.cluster.local/realms/master"
