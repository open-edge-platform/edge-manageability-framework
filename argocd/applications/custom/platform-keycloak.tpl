# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

## Custom template for keycloak-operator application
## This file provides environment-specific configuration overrides
## for the Keycloak Operator deployment

# Operator configuration
operator:
  namespace: keycloak-system

# Keycloak instance configuration
keycloak:
  enabled: true
  instanceName: keycloak-operator-instance
  instanceNamespace: orch-platform
  instances: 1
  
  hostname:
    strict: false
  
  http:
    httpEnabled: true
    httpPort: 8080
  
  proxy:
    headers: xforwarded

  # Database configuration - use cluster-specific database settings
  db:
    vendor: postgres
    host: postgresql.orch-database.svc.cluster.local
    port: 5432
    database: orch-platform-keycloak
    usernameSecret:
      name: platform-keycloak-{{ .Values.argo.database.type }}-postgresql
      key: PGUSER
    passwordSecret:
      name: platform-keycloak-{{ .Values.argo.database.type }}-postgresql
      key: PGPASSWORD

  # Additional options including proxy configuration
  additionalOptions:
    - name: KC_BOOTSTRAP_ADMIN_USERNAME
      value: admin
    - name: KC_BOOTSTRAP_ADMIN_PASSWORD
      value: admin
    - name: KC_PROXY_HEADERS
      value: xforwarded
    - name: KC_HOSTNAME_STRICT
      value: "false"
    - name: KC_HOSTNAME_STRICT_HTTPS  
      value: "false"
    {{- if .Values.argo.proxy.httpsProxy }}
    - name: HTTPS_PROXY
      value: {{ .Values.argo.proxy.httpsProxy }}
    {{- end }}
    {{- if .Values.argo.proxy.httpProxy }}
    - name: HTTP_PROXY
      value: {{ .Values.argo.proxy.httpProxy }}
    {{- end }}
    {{- if .Values.argo.proxy.noProxy }}
    - name: NO_PROXY
      value: {{ .Values.argo.proxy.noProxy }}
    {{- end }}

  # Resource configuration
  resources:
    requests:
      cpu: 200m
      memory: 512Mi
    limits:
      cpu: 500m
      memory: 1Gi

  # Configuration CLI - customize realm configuration with cluster-specific URLs
  configCli:
    enabled: true
    
    auth:
      username: admin
      password: admin
    
    resources:
      requests:
        cpu: 100m
        memory: 256Mi
      limits:
        cpu: 500m
        memory: 512Mi
    
    configuration:
      realm-master.json: |
        {
          "realm": "master",
          "accountTheme": "keycloak",
          "displayName": "Keycloak",
          "displayNameHtml": "<img src='https://raw.githubusercontent.com/open-edge-platform/orch-utils/73df5d1e99a81ae333d94b1c47dd9bef7fa03ae9/keycloak/one-edge-platform-login-title.png'></img>",
          "defaultSignatureAlgorithm": "PS512",
          "accessTokenLifespan": 3600,
          "ssoSessionIdleTimeout": 5400,
          "ssoSessionMaxLifespan": 43200,
          "passwordPolicy": "length(14) and digits(1) and specialChars(1) and upperCase(1) and lowerCase(1)",
          "bruteForceProtected": true,
          "permanentLockout": false,
          "maxFailureWaitSeconds": 900,
          "minimumQuickLoginWaitSeconds": 60,
          "waitIncrementSeconds": 300,
          "quickLoginCheckMilliSeconds": 200,
          "maxDeltaTimeSeconds": 43200,
          "failureFactor": 5,
          "clients": [
            {
              "clientId": "system-client",
              "name": "System Client",
              "description": "Client for System Operations",
              "enabled": true,
              "clientAuthenticatorType": "client-secret",
              "secret": "system-client-secret",
              "redirectUris": ["*"],
              "webOrigins": ["*"],
              "serviceAccountsEnabled": true,
              "authorizationServicesEnabled": false,
              "directAccessGrantsEnabled": true,
              "implicitFlowEnabled": false,
              "standardFlowEnabled": true
            },
            {
              "clientId": "web-ui",
              "name": "Web UI Client",
              "description": "Client for Web UI Application",
              "enabled": true,
              "clientAuthenticatorType": "client-secret",
              "secret": "web-ui-client-secret",
              "rootUrl": "https://web-ui.{{ .Values.argo.clusterDomain }}",
              "redirectUris": [
                "https://web-ui.{{ .Values.argo.clusterDomain }}/*",
                "https://app-service-proxy.{{ .Values.argo.clusterDomain }}/app-service-proxy-index.html*",
                "https://vnc.{{ .Values.argo.clusterDomain }}/*"
              ],
              "webOrigins": ["https://web-ui.{{ .Values.argo.clusterDomain }}"],
              "serviceAccountsEnabled": false,
              "authorizationServicesEnabled": false,
              "directAccessGrantsEnabled": false,
              "implicitFlowEnabled": false,
              "standardFlowEnabled": true,
              "publicClient": false
            }
          ]
        }