# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

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
{{- if and .Values.argo .Values.argo.resources .Values.argo.resources.platformKeycloak }}
keycloak:
  resources:
    {{- toYaml .Values.argo.resources.platformKeycloak | nindent 4 }}
{{- end }}

# Override Keycloak Config CLI resources if specified
{{- if and .Values.argo .Values.argo.resources .Values.argo.resources.keycloakConfigCli }}
keycloakConfigCli:
  job:
    container:
      resources:
        {{- toYaml .Values.argo.resources.keycloakConfigCli | nindent 8 }}
{{- end }}

# Override cluster domain if specified (for URL generation)
{{- if and .Values.argo .Values.argo.clusterDomain }}
argo:
  clusterDomain: {{ .Values.argo.clusterDomain | quote }}
{{- end }}

# Override Keycloak Config CLI environment variables to include proxy settings
keycloakConfigCli:
  job:
    container:
      env:
        - name: KEYCLOAK_URL
          value: "http://platform-keycloak.keycloak-system.svc.cluster.local/"
        - name: KEYCLOAK_USER
          value: "admin"
        - name: KEYCLOAK_PASSWORD
          valueFrom:
            secretKeyRef:
              name: platform-keycloak
              key: password
        - name: KEYCLOAK_AVAILABILITYCHECK_ENABLED
          value: "true"
        - name: KEYCLOAK_AVAILABILITYCHECK_TIMEOUT
          value: "120s"
        - name: IMPORT_VARSUBSTITUTION_ENABLED
          value: "true"
        - name: IMPORT_FILES_LOCATIONS
          value: "/config/*"
        - name: IMPORT_MANAGED_GROUP
          value: "no-delete"
        - name: IMPORT_MANAGED_REQUIRED_ACTION
          value: "no-delete"
        - name: IMPORT_MANAGED_ROLE
          value: "no-delete"
        - name: IMPORT_MANAGED_CLIENT
          value: "no-delete"
        - name: IMPORT_REMOTE_STATE_ENABLED
          value: "true"
        - name: LOGGING_LEVEL_ROOT
          value: "INFO"
        - name: LOGGING_LEVEL_KEYCLOAKCONFIGCLI
          value: "DEBUG"
        {{- if .Values.argo.proxy.httpsProxy }}
        - name: HTTPS_PROXY
          value: {{ .Values.argo.proxy.httpsProxy | quote }}
        {{- end }}
        {{- if .Values.argo.proxy.httpProxy }}
        - name: HTTP_PROXY
          value: {{ .Values.argo.proxy.httpProxy | quote }}
        {{- end }}
        {{- if .Values.argo.proxy.noProxy }}
        - name: NO_PROXY
          value: {{ .Values.argo.proxy.noProxy | quote }}
        {{- end }}

# Override realmMaster configuration with properly resolved clusterSpecific values
# This overrides the base config's realmMaster so template variables get resolved
realmMaster: |
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
          "clientId": "telemetry-client",
          "name": "Telemetry Client",
          "rootUrl": "https://observability-ui.{{ .Values.argo.clusterDomain }}",
          "enabled": true,
          "clientAuthenticatorType": "client-secret",
          "redirectUris": ["https://observability-admin.{{ .Values.argo.clusterDomain }}/login/generic_oauth", "https://observability-ui.{{ .Values.argo.clusterDomain }}/login/generic_oauth"],
          "webOrigins": ["+"],
          "protocol": "openid-connect",
          "directAccessGrantsEnabled": true,
          "attributes": {
            "oidc.ciba.grant.enabled": "false",
            "client.secret.creation.time": "1683218404",
            "backchannel.logout.session.required": "true",
            "post.logout.redirect.uris": "+",
            "display.on.consent.screen": "false",
            "use.jwks.url": "false",
            "oauth2.device.authorization.grant.enabled": "false",
            "backchannel.logout.revoke.offline.tokens": "false"
          },
          "fullScopeAllowed": true,
          "defaultClientScopes": ["roles", "profile", "email", "basic"],
          "optionalClientScopes": ["groups", "offline_access"]
        },
        {
          "clientId": "cluster-management-client",
          "name": "Cluster Management Client",
          "rootUrl": "https://cluster-management.{{ .Values.argo.clusterDomain }}",
          "adminUrl": "https://cluster-management.{{ .Values.argo.clusterDomain }}",
          "surrogateAuthRequired": false,
          "enabled": true,
          "clientAuthenticatorType": "client-secret",
          "redirectUris": ["https://cluster-management.{{ .Values.argo.clusterDomain }}", "https://cluster-management.{{ .Values.argo.clusterDomain }}/"],
          "webOrigins": ["+"],
          "directAccessGrantsEnabled": true,
          "serviceAccountsEnabled": false,
          "publicClient": true,
          "frontchannelLogout": true,
          "protocol": "openid-connect",
          "attributes": {
            "oauth2.device.authorization.grant.enabled": "false",
            "backchannel.logout.revoke.offline.tokens": "false",
            "use.refresh.tokens": "true",
            "oidc.ciba.grant.enabled": "false",
            "backchannel.logout.session.required": "true",
            "client_credentials.use_refresh_token": "false",
            "require.pushed.authorization.requests": "false",
            "tls.client.certificate.bound.access.tokens": "false",
            "display.on.consent.screen": "false",
            "token.response.type.bearer.lower-case": "false"
          },
          "fullScopeAllowed": true,
          "protocolMappers": [
            {
              "name": "Group Path",
              "protocol": "openid-connect",
              "protocolMapper": "oidc-group-membership-mapper",
              "consentRequired": false,
              "config": {
                "full.path": "true",
                "id.token.claim": "false",
                "access.token.claim": "false",
                "claim.name": "full_group_path",
                "userinfo.token.claim": "true"
              }
            },
            {
              "name": "Groups Mapper",
              "protocol": "openid-connect",
              "protocolMapper": "oidc-group-membership-mapper",
              "consentRequired": false,
              "config": {
                "full.path": "false",
                "id.token.claim": "false",
                "access.token.claim": "false",
                "claim.name": "groups",
                "userinfo.token.claim": "true"
              }
            },
            {
              "name": "Client Audience",
              "protocol": "openid-connect",
              "protocolMapper": "oidc-audience-mapper",
              "consentRequired": false,
              "config": {
                "included.client.audience": "cluster-management-client",
                "id.token.claim": "false",
                "access.token.claim": "true"
              }
            }
          ],
          "defaultClientScopes": ["profile", "roles", "email", "basic"],
          "optionalClientScopes": ["groups", "offline_access"],
          "authorizationServicesEnabled": false
        },
        {
          "clientId": "webui-client",
          "name": "WebUI Client",
          "rootUrl": "https://web-ui.{{ .Values.argo.clusterDomain }}",
          "enabled": true,
          "clientAuthenticatorType": "client-secret",
          "redirectUris": ["https://web-ui.{{ .Values.argo.clusterDomain }}", "https://app-service-proxy.{{ .Values.argo.clusterDomain }}/app-service-proxy-index.html*", "https://vnc.{{ .Values.argo.clusterDomain }}/*", "https://{{ .Values.argo.clusterDomain }}"{{- if index .Values.argo "platform-keycloak" "extraUiRedirects" -}}, {{- index .Values.argo "platform-keycloak" "extraUiRedirects" -}}{{- end -}}],
          "webOrigins": ["+"],
          "protocol": "openid-connect",
          "directAccessGrantsEnabled": false,
          "attributes": {
            "oidc.ciba.grant.enabled": "false",
            "client.secret.creation.time": "1683218404",
            "backchannel.logout.session.required": "true",
            "post.logout.redirect.uris": "+",
            "display.on.consent.screen": "false",
            "oauth2.device.authorization.grant.enabled": "true",
            "backchannel.logout.revoke.offline.tokens": "false"
          },
          "fullScopeAllowed": true,
          "defaultClientScopes": ["roles", "profile", "email", "basic"],
          "optionalClientScopes": ["groups", "offline_access"]
        },
        {
          "clientId": "docsui-client",
          "name": "DocsUI Client",
          "rootUrl": "https://docs-ui.{{ .Values.argo.clusterDomain }}",
          "enabled": true,
          "clientAuthenticatorType": "client-secret",
          "redirectUris": ["https://docs-ui.{{ .Values.argo.clusterDomain }}", "https://docs-ui.{{ .Values.argo.clusterDomain }}/"],
          "webOrigins": ["+"],
          "protocol": "openid-connect",
          "directAccessGrantsEnabled": false,
          "attributes": {
            "oidc.ciba.grant.enabled": "false",
            "client.secret.creation.time": "1683218404",
            "backchannel.logout.session.required": "true",
            "post.logout.redirect.uris": "+",
            "display.on.consent.screen": "false",
            "oauth2.device.authorization.grant.enabled": "true",
            "backchannel.logout.revoke.offline.tokens": "false"
          },
          "fullScopeAllowed": true,
          "defaultClientScopes": ["roles", "profile", "email", "basic"],
          "optionalClientScopes": ["groups", "offline_access"]
        },
        {
          "frontchannelLogout": true,
          "standardFlowEnabled": true,
          "clientId": "registry-client",
          "name": "Registry Client",
          "rootUrl": "https://registry-oci.{{ .Values.argo.clusterDomain }}",
          "enabled": true,
          "clientAuthenticatorType": "client-secret",
          "redirectUris": ["/c/oidc/callback"],
          "webOrigins": ["+"],
          "protocol": "openid-connect",
          "directAccessGrantsEnabled": true,
          "attributes": {
            "oidc.ciba.grant.enabled": "false",
            "client.secret.creation.time": "1683218404",
            "backchannel.logout.session.required": "true",
            "post.logout.redirect.uris": "+",
            "display.on.consent.screen": "false",
            "use.jwks.url": "false",
            "oauth2.device.authorization.grant.enabled": "false",
            "backchannel.logout.revoke.offline.tokens": "false"
          },
          "fullScopeAllowed": true,
          "defaultClientScopes": ["roles", "profile", "email", "groups", "basic"],
          "optionalClientScopes": ["offline_access"]
        },
        {
          "clientId": "system-client",
          "name": "System Client",
          "surrogateAuthRequired": false,
          "enabled": true,
          "clientAuthenticatorType": "client-secret",
          "redirectUris": [],
          "webOrigins": [],
          "standardFlowEnabled": false,
          "implicitFlowEnabled": false,
          "directAccessGrantsEnabled": true,
          "serviceAccountsEnabled": false,
          "publicClient": true,
          "frontchannelLogout": true,
          "protocol": "openid-connect",
          "attributes": {
            "oidc.ciba.grant.enabled": "false",
            "oauth2.device.authorization.grant.enabled": "true",
            "backchannel.logout.session.required": "true",
            "backchannel.logout.revoke.offline.tokens": "false"
          },
          "fullScopeAllowed": true,
          "defaultClientScopes": [
            "roles",
            "profile",
            "email",
            "basic"
          ],
          "optionalClientScopes": [
            "groups",
            "offline_access"
          ]
        }
      ]
    }
