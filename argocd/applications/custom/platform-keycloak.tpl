# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

## Custom template for platform-keycloak application
## This file provides environment-specific configuration overrides
## for the Keycloak deployment using official operator

# Operator settings (usually no customization needed)
operator:
  enabled: true
  namespace: keycloak-system

# Keycloak instance configuration
keycloak:
  instanceName: platform-keycloak
  instanceNamespace: orch-platform
  instances: 1
  
  # Bootstrap admin credentials from the secret created by deploy.go
  bootstrapAdmin:
    user:
      secret: platform-keycloak
  
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
    database: orch-platform-platform-keycloak
    usernameSecret:
      name: platform-keycloak-local-postgresql
      key: PGUSER
    passwordSecret:
      name: platform-keycloak-local-postgresql
      key: PGPASSWORD

  # Additional options including proxy configuration
  additionalOptions:
    - name: KC_PROXY_HEADERS
      value: xforwarded
    - name: KC_HOSTNAME_STRICT
      value: "false"
    - name: KC_HOSTNAME_STRICT_HTTPS  
      value: "false"
    {{- if .Values.argo.proxy.httpsProxy }}
    - name: HTTPS_PROXY
      value: http://proxy-dmz.intel.com:912
    {{- else }}
    - name: HTTPS_PROXY
      value: http://proxy-dmz.intel.com:912
    {{- end }}
    {{- if .Values.argo.proxy.httpProxy }}
    - name: HTTP_PROXY
      value: http://proxy-dmz.intel.com:912
    {{- else }}
    - name: HTTP_PROXY
      value: http://proxy-dmz.intel.com:912
    {{- end }}
    {{- if .Values.argo.proxy.noProxy }}
    - name: NO_PROXY
      value: localhost,svc,cluster.local,default,internal,caas.intel.com,certificates.intel.com,localhost,127.0.0.0/8,10.0.0.0/8,192.168.0.0/16,172.16.0.0/12,169.254.169.254,orch-platform,orch-app,orch-cluster,orch-infra,orch-database,cattle-system,orch-secret,s3.amazonaws.com,s3.us-west-2.amazonaws.com,ec2.us-west-2.amazonaws.com,eks.amazonaws.com,elb.us-west-2.amazonaws.com,dkr.ecr.us-west-2.amazonaws.com,espd.infra-host.com,pid.infra-host.com,espdqa.infra-host.com,argocd-repo-server
    {{- else }}
    - name: NO_PROXY
      value: localhost,svc,cluster.local,default,internal,caas.intel.com,certificates.intel.com,localhost,127.0.0.0/8,10.0.0.0/8,192.168.0.0/16,172.16.0.0/12,169.254.169.254,orch-platform,orch-app,orch-cluster,orch-infra,orch-database,cattle-system,orch-secret,s3.amazonaws.com,s3.us-west-2.amazonaws.com,ec2.us-west-2.amazonaws.com,eks.amazonaws.com,elb.us-west-2.amazonaws.com,dkr.ecr.us-west-2.amazonaws.com,espd.infra-host.com,pid.infra-host.com,espdqa.infra-host.com,argocd-repo-server
    {{- end }}

  # Ingress configuration - disabled since no ingress controller is available
  ingress:
    enabled: false

  # Network Policy configuration - disabled to avoid needing networkpolicies RBAC
  networkPolicy:
    enabled: false

  # Resource configuration
  resources:
    requests:
      cpu: 200m
      memory: 512Mi
    limits:
      cpu: 500m
      memory: 1Gi

# Service alias configuration
service:
  enabled: true
  name: platform-keycloak
  namespace: orch-platform
  port: 8080

# Configuration CLI - customize realm configuration with cluster-specific URLs
configCli:
  enabled: true
  image: curlimages/curl:8.4.0
  
  # Authentication using the platform-keycloak secret
  auth:
    secretName: platform-keycloak
    usernameKey: username
    passwordKey: password
  
  resources:
    requests:
      cpu: 100m
      memory: 256Mi
    limits:
      cpu: 500m
      memory: 512Mi
  
  # Realm configuration with cluster-specific URLs
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