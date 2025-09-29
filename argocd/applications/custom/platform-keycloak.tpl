# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# CodeCentric Keycloak Chart Template
# Updated for CodeCentric keycloakx chart instead of Bitnami chart

## Cluster-Specific values for realm configuration
## These values parameterize the realm configuration for different environments
## @param clusterSpecific.webuiClientRootUrl The Keycloak Master realm UI Client's rootUrl value
## @param clusterSpecific.webuiRedirectUrls The Keycloak Master realm UI Client's redirectUrl values
## @param clusterSpecific.registryClientRootUrl The Keycloak Master realm Harbor Client's rootUrl value
## @param clusterSpecific.telemetryClientRootUrl The Keycloak Master realm Grafana Client's rootUrl value
## @param clusterSpecific.telemetryRedirectUrls The Keycloak Master realm Grafana Client's redirectUrl values
clusterSpecific:
  webuiClientRootUrl: "https://web-ui.{{ .Values.argo.clusterDomain }}"
  webuiRedirectUrls: ["https://web-ui.{{ .Values.argo.clusterDomain }}", "https://app-service-proxy.{{ .Values.argo.clusterDomain }}/app-service-proxy-index.html*", "https://vnc.{{ .Values.argo.clusterDomain }}/*", "https://{{ .Values.argo.clusterDomain }}"{{- if index .Values.argo "platform-keycloak" "extraUiRedirects" -}}, {{- index .Values.argo "platform-keycloak" "extraUiRedirects" -}}{{- end -}}]
  registryClientRootUrl: "https://registry-oci.{{ .Values.argo.clusterDomain }}"
  telemetryClientRootUrl: "https://observability-ui.{{ .Values.argo.clusterDomain }}"
  telemetryRedirectUrls: ["https://observability-admin.{{ .Values.argo.clusterDomain }}/login/generic_oauth", "https://observability-ui.{{ .Values.argo.clusterDomain }}/login/generic_oauth"]

## Database configuration is defined in the base config file (platform-keycloak.yaml)
## This template only provides cluster-specific overrides

## Storage configuration (if local registry is used)
{{- if index .Values.argo "platform-keycloak" "localRegistrySize"}}
persistence:
  storageClass: ""
  size: {{index .Values.argo "platform-keycloak" "localRegistrySize"}}
{{- end}}

## Environment variables are defined in the base config file (platform-keycloak.yaml)
## This template only provides cluster-specific overrides

## Resource configuration
{{- with .Values.argo.resources.platformKeycloak }}
resources:
  {{- toYaml . | nindent 2}}
{{- end }}

## Service configuration for compatibility
service:
  type: ClusterIP
  httpPort: 8080

## Network policy (typically disabled for simplicity)
networkPolicy:
  enabled: false

## Pod labels to avoid sidecar injection
podLabels:
  sidecar.istio.io/inject: "false"

## Additional volumes for realm configuration
extraVolumes: |
  - name: keycloak-config
    configMap:
      name: platform-keycloak-config

## Health probes configuration
livenessProbe: |
  httpGet:
    path: /
    port: http
  initialDelaySeconds: 60
  periodSeconds: 30
  timeoutSeconds: 5
  failureThreshold: 3

readinessProbe: |
  httpGet:
    path: /realms/master
    port: http
  initialDelaySeconds: 30
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3

## Enable metrics and health endpoints
metrics:
  enabled: true

health:
  enabled: true
