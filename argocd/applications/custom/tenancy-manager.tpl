# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

image:
  registry: {{.Values.argo.containerRegistryURL }}
  repository: common/tenancy-manager
imagePullSecrets:
  {{- with .Values.argo.imagePullSecrets }}
    {{- toYaml . | nindent 2 }}
  {{- end }}

{{- with .Values.argo.resources.tenancyManager }}
resources:
  {{- toYaml . | nindent 4 }}
{{- end }}

# HTTP REST API server configuration
serviceArgs:
  enableHTTPServer: true
  httpPort: 8080
  enableAuth: true

# Traefik reverse proxy configuration for IngressRoute
traefikReverseProxy:
  enabled: true
  apiHostname: {{ required "A valid orchestratorAPIEndpoint entry required!" .Values.argo.orchestratorAPIEndpoint }}
  tlsOption: "default"
  secretName: "tls-orch"

# OIDC configuration for authentication
oidc:
  serverUrl: "http://platform-keycloak.orch-platform.svc/realms/master"
  tlsInsecureSkipVerify: true
