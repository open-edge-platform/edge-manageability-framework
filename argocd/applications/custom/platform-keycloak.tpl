# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

## Cluster-Specific values for Keycloak Operator
## These values are used to configure:
## 1. Realm import configuration (clients, redirect URIs, etc.)
## 2. Keycloak instance deployment parameters

clusterSpecific:
  # WebUI Client configuration
  webuiClientRootUrl: "https://web-ui.{{ .Values.argo.clusterDomain }}"
  webuiRedirectUrls: ["https://web-ui.{{ .Values.argo.clusterDomain }}", "https://app-service-proxy.{{ .Values.argo.clusterDomain }}/app-service-proxy-index.html*", "https://vnc.{{ .Values.argo.clusterDomain }}/*", "https://{{ .Values.argo.clusterDomain }}"{{- if index .Values.argo "platform-keycloak" "extraUiRedirects" -}}, {{- index .Values.argo "platform-keycloak" "extraUiRedirects" -}}{{- end -}}]
  
  # DocsUI Client configuration
  docsuiClientRootUrl: "https://docs-ui.{{ .Values.argo.clusterDomain }}"
  docsuiRedirectUrls: ["https://docs-ui.{{ .Values.argo.clusterDomain }}", "https://docs-ui.{{ .Values.argo.clusterDomain }}/"]
  
  # Registry Client configuration (Harbor)
  registryClientRootUrl: "https://registry-oci.{{ .Values.argo.clusterDomain }}"
  
  # Telemetry Client configuration (Grafana, Observability)
  telemetryClientRootUrl: "https://observability-ui.{{ .Values.argo.clusterDomain }}"
  telemetryRedirectUrls: ["https://observability-admin.{{ .Values.argo.clusterDomain }}/login/generic_oauth", "https://observability-ui.{{ .Values.argo.clusterDomain }}/login/generic_oauth"]
  
  # Cluster Management Client configuration
  clusterManagementClientRootUrl: "https://cluster-management.{{ .Values.argo.clusterDomain }}"
  clusterManagementRedirectUrls: ["https://cluster-management.{{ .Values.argo.clusterDomain }}", "https://cluster-management.{{ .Values.argo.clusterDomain }}/"]

# Keycloak Operator configuration
keycloak:
  # Number of replicas
  instances: 1
  
  # Proxy configuration (for egress through corporate proxies)
  proxy:
    headers: xforwarded

# Database configuration (external PostgreSQL)
database:
  vendor: postgres
  host: orch-database-postgresql.orch-database.svc.cluster.local
  port: 5432
  database: keycloak
  existingSecret: orch-database-postgresql
  existingSecretHostKey: PGHOST
  existingSecretPortKey: PGPORT
  existingSecretUserKey: PGUSER
  existingSecretPasswordKey: PGPASSWORD
  existingSecretDatabaseKey: PGDATABASE
  
  # Connection pool configuration
  {{- if index .Values.argo "platform-keycloak" "db" }}
  poolInitSize: {{ index .Values.argo "platform-keycloak" "db" "poolInitSize" | default "5" }}
  poolMinSize: {{ index .Values.argo "platform-keycloak" "db" "poolMinSize" | default "5" }}
  poolMaxSize: {{ index .Values.argo "platform-keycloak" "db" "poolMaxSize" | default "100" }}
  {{- end }}

# HTTP configuration
http:
  port: 8080
  relativeUrls: true

# Environment proxy settings
envProxy:
  httpsProxy: {{.Values.argo.proxy.httpsProxy}}
  httpProxy: {{.Values.argo.proxy.httpProxy}}
  noProxy: {{.Values.argo.proxy.noProxy}}

# Resource limits
{{- with .Values.argo.resources.platformKeycloak }}
resources:
  {{- toYaml . | nindent 2}}
{{- end }}

# Realm configuration settings (used by realm-import.yaml)
realmConfig:
  # Display name and theme
  displayName: "Keycloak"
  accountTheme: "keycloak"
  displayNameHtml: "<img src='https://raw.githubusercontent.com/open-edge-platform/orch-utils/73df5d1e99a81ae333d94b1c47dd9bef7fa03ae9/keycloak/one-edge-platform-login-title.png'></img>"
  
  # Security settings
  defaultSignatureAlgorithm: "PS512"
  accessTokenLifespan: 3600
  ssoSessionIdleTimeout: 5400
  ssoSessionMaxLifespan: 43200
  passwordPolicy: "length(14) and digits(1) and specialChars(1) and upperCase(1) and lowerCase(1)"
  
  # Brute force protection
  bruteForceProtected: true
  permanentLockout: false
  maxFailureWaitSeconds: 900
  minimumQuickLoginWaitSeconds: 60
  waitIncrementSeconds: 300
  quickLoginCheckMilliSeconds: 200
  maxDeltaTimeSeconds: 43200
  failureFactor: 5
