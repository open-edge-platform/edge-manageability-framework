# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0


# Keycloak instance configuration (Keycloak CRD)
keycloak:
  # Container image (uses upstream version from Chart.yaml)
  image: "quay.io/keycloak/keycloak:26.4.5"
  imagePullPolicy: IfNotPresent

  # Bootstrap admin credentials
  bootstrapAdmin:
    secret: platform-keycloak

  # Database configuration
  database:
    vendor: postgres
    usernameSecret:
      name: platform-keycloak-{{.Values.argo.database.type}}-postgresql
      key: PGUSER
    passwordSecret:
      name: platform-keycloak-{{.Values.argo.database.type}}-postgresql
      key: PGPASSWORD
    # Connection pool settings
    poolInitialSize: 5
    poolMinSize: 5
    poolMaxSize: 50

  # HTTP configuration
  http:
    httpEnabled: true
    httpPort: 8080
    relativeUrl: "/"

  # Proxy headers for reverse proxy
  proxy:
    headers: xforwarded

  # Ingress configuration
  ingress:
    enabled: false

  # Pod resource allocation
  resources:
    requests:
      cpu: 500m
      memory: 512Mi
    limits:
      cpu: 2000m
      memory: 2Gi

  # Runtime configuration options
  additionalOptions:
    # Read database connection details from secret
    - name: db-url-host
      secret:
        name: platform-keycloak-{{.Values.argo.database.type}}-postgresql
        key: PGHOST
    - name: db-url-port
      secret:
        name: platform-keycloak-{{.Values.argo.database.type}}-postgresql
        key: PGPORT
    - name: db-url-database
      secret:
        name: platform-keycloak-{{.Values.argo.database.type}}-postgresql
        key: PGDATABASE
    - name: hostname-strict
      value: "false"
    - name: http-relative-path
      value: "/"
    - name: http-enabled
      value: "true"
    - name: db-url-properties
      value: "?tcpKeepAlives=true&socketTimeout=120&connectTimeout=120"
    - name: http-management-port
      value: "9000"
    - name: spi-login-protocol-openid-connect-legacy-logout-redirect-uri
      value: "true"
    - name: spi-brute-force-protector-default-brute-force-detector-allow-concurrent-requests
      value: "true"
    - name: log-level
      value: "INFO"
    - name: log-console-output
      value: "json"

  # Pod optimization
  startOptimized: true
  instances: 1

  # Pod template security and initialization
  podTemplate:
    # Pod security context
    securityContext:
      runAsUser: 1000
      runAsGroup: 1000
      fsGroup: 1000
      seccompProfile:
        type: RuntimeDefault

    # Init container for building optimized Keycloak image
    initContainers:
      - name: keycloak-builder
        image: "quay.io/keycloak/keycloak:26.4.5"
        imagePullPolicy: IfNotPresent
        command:
          - /bin/bash
        args:
          - -c
          - |
            set -e
            /opt/keycloak/bin/kc.sh build \
              --db=postgres \
              --health-enabled=true \
              --metrics-enabled=true \
              --http-relative-path=/

            mkdir -p /shared/keycloak
            cp -r /opt/keycloak/* /shared/keycloak/

        resources:
          requests:
            cpu: 500m
            memory: 512Mi
          limits:
            cpu: 1500m
            memory: 1Gi

        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: false
          runAsNonRoot: true
          runAsUser: 1000
          runAsGroup: 1000
          capabilities:
            drop:
              - ALL
          seccompProfile:
            type: RuntimeDefault

        volumeMounts:
          - name: shared-keycloak
            mountPath: /shared/keycloak
          - name: tmp
            mountPath: /tmp

    # Main container security context
    containerSecurityContext:
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true
      runAsNonRoot: true
      runAsUser: 1000
      runAsGroup: 1000
      capabilities:
        drop:
          - ALL
      seccompProfile:
        type: RuntimeDefault

    # Container volume mounts
    containerVolumeMounts:
      - name: shared-keycloak
        mountPath: /opt/keycloak
        readOnly: true
      - name: tmp
        mountPath: /tmp
      - name: keycloak-data
        mountPath: /opt/keycloak/data

    # Pod volumes
    volumes:
      - name: shared-keycloak
        emptyDir:
          sizeLimit: 1Gi
      - name: tmp
        emptyDir:
          sizeLimit: 100Mi
      - name: keycloak-data
        emptyDir:
          sizeLimit: 500Mi
{{- if and .Values.argo .Values.argo.resources .Values.argo.resources.platformKeycloak }}
  resources:
    {{- toYaml .Values.argo.resources.platformKeycloak | nindent 4 }}
{{- end }}

## These values are used to configure:
## 1. Realm import configuration (clients, redirect URIs, etc.)
## 2. Keycloak instance deployment parameters

{{- $clusterDomain := .Values.argo.clusterDomain -}}
{{- $extraUiRedirects := index .Values.argo "platform-keycloak" "extraUiRedirects" -}}
{{- $webuiRootUrl := printf "https://web-ui.%s" $clusterDomain -}}
{{- $docsuiRootUrl := printf "https://docs-ui.%s" $clusterDomain -}}
{{- $registryRootUrl := printf "https://registry-oci.%s" $clusterDomain -}}
{{- $telemetryRootUrl := printf "https://observability-ui.%s" $clusterDomain -}}
{{- $clusterMgmtRootUrl := printf "https://cluster-management.%s" $clusterDomain -}}
{{- $webuiRedirects := list (printf "https://web-ui.%s" $clusterDomain) (printf "https://app-service-proxy.%s/app-service-proxy-index.html*" $clusterDomain) (printf "https://vnc.%s/*" $clusterDomain) (printf "https://%s" $clusterDomain) -}}
{{- if $extraUiRedirects -}}
  {{- $webuiRedirects = append $webuiRedirects $extraUiRedirects -}}
{{- end -}}
{{- $docsuiRedirects := list (printf "https://docs-ui.%s" $clusterDomain) (printf "https://docs-ui.%s/" $clusterDomain) -}}
{{- $telemetryRedirects := list (printf "https://observability-admin.%s/login/generic_oauth" $clusterDomain) (printf "https://observability-ui.%s/login/generic_oauth" $clusterDomain) -}}
{{- $clusterMgmtRedirects := list (printf "https://cluster-management.%s" $clusterDomain) (printf "https://cluster-management.%s/" $clusterDomain) -}}

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
          value: "http://platform-keycloak.orch-platform.svc.cluster.local/"
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
      "roles": {
        "realm": [
          {
            "name": "en-agent-rw"
          },
          {
            "name": "secrets-root-role"
          },
          {
            "name": "rs-access-r"
          },
          {
            "name": "rs-proxy-r"
          },
          {
            "name": "app-service-proxy-read-role"
          },
          {
            "name": "app-service-proxy-write-role"
          },
          {
            "name": "app-deployment-manager-read-role"
          },
          {
            "name": "app-deployment-manager-write-role"
          },
          {
            "name": "app-resource-manager-read-role"
          },
          {
            "name": "app-resource-manager-write-role"
          },
          {
            "name": "app-vm-console-write-role"
          },
          {
            "name": "catalog-publisher-read-role"
          },
          {
            "name": "catalog-publisher-write-role"
          },
          {
            "name": "catalog-other-read-role"
          },
          {
            "name": "catalog-other-write-role"
          },
          {
            "name": "catalog-restricted-read-role"
          },
          {
            "name": "catalog-restricted-write-role"
          },
          {
            "name": "clusters-read-role"
          },
          {
            "name": "clusters-write-role"
          },
          {
            "name": "cluster-templates-read-role"
          },
          {
            "name": "cluster-templates-write-role"
          },
          {
            "name": "cluster-artifacts-read-role"
          },
          {
            "name": "cluster-artifacts-write-role"
          },
          {
            "name": "infra-manager-core-read-role"
          },
          {
            "name": "infra-manager-core-write-role"
          },
          {
            "name": "alrt-r"
          },
          {
            "name": "alrt-rw"
          },
          {
            "name": "alrt-rx-rw"
          },
          {
            "name": "ao-m2m-rw"
          },
          {
            "name": "co-m2m-rw"
          },
          {
            "name": "org-read-role"
          },
          {
            "name": "org-write-role"
          },
          {
            "name": "org-update-role"
          },
          {
            "name": "org-delete-role"
          }
        ],
        "client": {
          "alerts-m2m-client": [],
          "host-manager-m2m-client": [],
          "co-manager-m2m-client": [],
          "ktc-m2m-client": [],
          "3rd-party-host-manager-m2m-client": [],
          "edge-manager-m2m-client": [],
          "en-m2m-template-client": [],
          "webui-client": [],
          "docsui-client": [],
          "account": [
            {
              "name": "view-profile",
              "clientRole": true
            },
            {
              "name": "manage-account",
              "clientRole": true
            }
          ],
          "telemetry-client": [
            {
              "name": "admin",
              "clientRole": true
            },
            {
              "name": "viewer",
              "clientRole": true
            }
          ],
          "cluster-management-client": [
            {
              "name": "restricted-role",
              "clientRole": true
            },
            {
              "name": "standard-role",
              "clientRole": true
            },
            {
              "name": "base-role",
              "clientRole": true
            }
          ],
          "registry-client": [
            {
              "name": "registry-admin-role",
              "clientRole": true
            },
            {
              "name": "registry-editor-role",
              "clientRole": true
            },
            {
              "name": "registry-viewer-role",
              "clientRole": true
            }
          ]
        }
      },
      "clients": [
        {
          "clientId": "alerts-m2m-client",
          "name": "Alerts M2M Client",
          "description": "Client for Alerts",
          "surrogateAuthRequired": false,
          "enabled": true,
          "alwaysDisplayInConsole": false,
          "clientAuthenticatorType": "client-secret",
          "notBefore": 0,
          "bearerOnly": false,
          "consentRequired": false,
          "standardFlowEnabled": false,
          "implicitFlowEnabled": false,
          "directAccessGrantsEnabled": false,
          "serviceAccountsEnabled": true,
          "authorizationServicesEnabled": true,
          "publicClient": false,
          "protocol": "openid-connect",
          "attributes": {
            "oidc.ciba.grant.enabled": "false",
            "oauth2.device.authorization.grant.enabled": "false",
            "backchannel.logout.revoke.offline.tokens": "false"
          },
          "fullScopeAllowed": true,
          "defaultClientScopes": [
            "web-origins",
            "acr",
            "profile",
            "roles",
            "email",
            "basic"
          ],
          "optionalClientScopes": [
            "address",
            "phone",
            "offline_access",
            "microprofile-jwt"
          ]
        },
        {
          "clientId": "host-manager-m2m-client",
          "name": "Host Manager Client",
          "description": "Client for the EN Host Manager to use in creating edgenode m2m clients",
          "surrogateAuthRequired": false,
          "enabled": true,
          "alwaysDisplayInConsole": false,
          "clientAuthenticatorType": "client-secret",
          "notBefore": 0,
          "bearerOnly": false,
          "consentRequired": false,
          "standardFlowEnabled": false,
          "implicitFlowEnabled": false,
          "directAccessGrantsEnabled": false,
          "serviceAccountsEnabled": true,
          "authorizationServicesEnabled": true,
          "publicClient": false,
          "frontchannelLogout": true,
          "protocol": "openid-connect",
          "attributes": {
            "oidc.ciba.grant.enabled": "false",
            "oauth2.device.authorization.grant.enabled": "false",
            "backchannel.logout.session.required": "true",
            "backchannel.logout.revoke.offline.tokens": "false"
          },
          "fullScopeAllowed": true,
          "defaultClientScopes": [
            "web-origins",
            "acr",
            "profile",
            "roles",
            "email",
            "basic"
          ],
          "optionalClientScopes": [
            "address",
            "phone",
            "offline_access",
            "microprofile-jwt"
          ]
        },
        {
          "clientId": "co-manager-m2m-client",
          "name": "Cluster Orchestrator Manager M2M Client",
          "description": "Client for cluster-manager to access Keycloak for JWT TTL management",
          "surrogateAuthRequired": false,
          "enabled": true,
          "alwaysDisplayInConsole": false,
          "clientAuthenticatorType": "client-secret",
          "notBefore": 0,
          "bearerOnly": false,
          "consentRequired": false,
          "standardFlowEnabled": false,
          "implicitFlowEnabled": false,
          "directAccessGrantsEnabled": false,
          "serviceAccountsEnabled": true,
          "authorizationServicesEnabled": true,
          "publicClient": false,
          "frontchannelLogout": true,
          "protocol": "openid-connect",
          "attributes": {
            "oidc.ciba.grant.enabled": "false",
            "oauth2.device.authorization.grant.enabled": "false",
            "backchannel.logout.session.required": "true",
            "backchannel.logout.revoke.offline.tokens": "false"
          },
          "fullScopeAllowed": true,
          "defaultClientScopes": [
            "web-origins",
            "acr",
            "profile",
            "roles",
            "email",
            "basic",
            "groups"
          ],
          "optionalClientScopes": [
            "offline_access"
          ]
        },
        {
          "clientId": "ktc-m2m-client",
          "name": "Keycloak Tenant Controller client",
          "description": "Client for the Keycloak Tenant Controller to use in creating Tenant specific roles and groups in Keycloak",
          "surrogateAuthRequired": false,
          "enabled": true,
          "alwaysDisplayInConsole": false,
          "clientAuthenticatorType": "client-secret",
          "notBefore": 0,
          "bearerOnly": false,
          "consentRequired": false,
          "standardFlowEnabled": false,
          "implicitFlowEnabled": false,
          "directAccessGrantsEnabled": false,
          "serviceAccountsEnabled": true,
          "authorizationServicesEnabled": true,
          "publicClient": false,
          "frontchannelLogout": true,
          "protocol": "openid-connect",
          "attributes": {
            "oidc.ciba.grant.enabled": "false",
            "oauth2.device.authorization.grant.enabled": "false",
            "backchannel.logout.session.required": "true",
            "backchannel.logout.revoke.offline.tokens": "false"
          },
          "fullScopeAllowed": true,
          "defaultClientScopes": [
            "web-origins",
            "acr",
            "profile",
            "roles",
            "groups",
            "email",
            "basic"
          ],
          "optionalClientScopes": [
            "address",
            "phone",
            "offline_access",
            "microprofile-jwt"
          ]
        },
        {
          "clientId": "3rd-party-host-manager-m2m-client",
          "name": "3rd Party Host Manager Client",
          "description": "Client for the 3rd party Host Manager to use in creating edgenode m2m clients",
          "surrogateAuthRequired": false,
          "enabled": true,
          "alwaysDisplayInConsole": false,
          "clientAuthenticatorType": "client-secret",
          "notBefore": 0,
          "bearerOnly": false,
          "consentRequired": false,
          "standardFlowEnabled": false,
          "implicitFlowEnabled": false,
          "directAccessGrantsEnabled": false,
          "serviceAccountsEnabled": true,
          "authorizationServicesEnabled": true,
          "publicClient": false,
          "frontchannelLogout": true,
          "protocol": "openid-connect",
          "attributes": {
            "oidc.ciba.grant.enabled": "false",
            "oauth2.device.authorization.grant.enabled": "false",
            "backchannel.logout.session.required": "true",
            "backchannel.logout.revoke.offline.tokens": "false"
          },
          "fullScopeAllowed": true,
          "defaultClientScopes": [
            "web-origins",
            "acr",
            "profile",
            "roles",
            "email",
            "basic"
          ],
          "optionalClientScopes": [
            "address",
            "phone",
            "offline_access",
            "microprofile-jwt"
          ]
        },
        {
          "clientId": "edge-manager-m2m-client",
          "name": "Edge Manager M2M Client",
          "description": "Client for the accessing Orchestrator with Edge-Manager persona",
          "surrogateAuthRequired": false,
          "enabled": true,
          "alwaysDisplayInConsole": false,
          "clientAuthenticatorType": "client-secret",
          "notBefore": 0,
          "bearerOnly": false,
          "consentRequired": false,
          "standardFlowEnabled": false,
          "implicitFlowEnabled": false,
          "directAccessGrantsEnabled": false,
          "serviceAccountsEnabled": true,
          "authorizationServicesEnabled": true,
          "publicClient": false,
          "frontchannelLogout": true,
          "protocol": "openid-connect",
          "attributes": {
            "oidc.ciba.grant.enabled": "false",
            "oauth2.device.authorization.grant.enabled": "false",
            "backchannel.logout.session.required": "true",
            "backchannel.logout.revoke.offline.tokens": "false"
          },
          "fullScopeAllowed": true,
          "defaultClientScopes": [
            "roles",
            "email",
            "groups",
            "basic"
          ],
          "optionalClientScopes": [
            "offline_access",
          ]
        },
        {
          "clientId": "en-m2m-template-client",
          "name": "Edge Node M2M Template Client",
          "description": "Client to use as basis for Roles to assign to new Edge Node M2M clients",
          "surrogateAuthRequired": false,
          "enabled": false,
          "alwaysDisplayInConsole": false,
          "clientAuthenticatorType": "client-secret",
          "notBefore": 0,
          "bearerOnly": false,
          "consentRequired": false,
          "standardFlowEnabled": false,
          "implicitFlowEnabled": false,
          "directAccessGrantsEnabled": false,
          "serviceAccountsEnabled": true,
          "authorizationServicesEnabled": true,
          "publicClient": false,
          "protocol": "openid-connect",
          "attributes": {
            "oidc.ciba.grant.enabled": "false",
            "oauth2.device.authorization.grant.enabled": "false",
            "backchannel.logout.revoke.offline.tokens": "false"
          },
          "fullScopeAllowed": true,
          "defaultClientScopes": [
            "web-origins",
            "acr",
            "profile",
            "roles",
            "email",
            "basic"
          ],
          "optionalClientScopes": [
            "address",
            "phone",
            "offline_access",
            "microprofile-jwt"
          ]
        },
        {
          "clientId": "telemetry-client",
          "name": "Telemetry Client",
          "rootUrl": "{{ $telemetryRootUrl }}",
          "enabled": true,
          "clientAuthenticatorType": "client-secret",
          "redirectUris": {{ $telemetryRedirects | toJson }},
          "webOrigins": [
            "+"
          ],
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
        },
        {
          "clientId": "cluster-management-client",
          "name": "Cluster Management Client",
          "rootUrl": "{{ $clusterMgmtRootUrl }}",
          "adminUrl": "{{ $clusterMgmtRootUrl }}",
          "surrogateAuthRequired": false,
          "enabled": true,
          "clientAuthenticatorType": "client-secret",
          "redirectUris": {{ $clusterMgmtRedirects | toJson }},
          "webOrigins": [
            "+"
          ],
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
            "token.response.type.bearer.lower-case": "false",
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
          "defaultClientScopes": [
            "profile",
            "roles",
            "email",
            "basic"
          ],
          "optionalClientScopes": [
            "groups",
            "offline_access",
          ],
          "authorizationServicesEnabled": false
        },
        {
          "clientId": "webui-client",
          "name": "WebUI Client",
          "rootUrl": "{{ $webuiRootUrl }}",
          "enabled": true,
          "clientAuthenticatorType": "client-secret",
          "redirectUris": {{ $webuiRedirects | toJson }},
          "webOrigins": [
            "+"
          ],
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
        },
        {
          "clientId": "docsui-client",
          "name": "DocsUI Client",
          "rootUrl": "{{ $docsuiRootUrl }}",
          "enabled": true,
          "clientAuthenticatorType": "client-secret",
          "redirectUris": {{ $docsuiRedirects | toJson }},
          "webOrigins": [
            "+"
          ],
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
            "openid",
            "profile",
            "email",
            "groups",
            "roles",
            "basic"
          ],
          "optionalClientScopes": [
            "offline_access"
          ]
        },
        {
          "frontchannelLogout": true,
          "standardFlowEnabled": true,
          "clientId": "registry-client",
          "name": "Registry Client",
          "rootUrl": "{{ $registryRootUrl }}",
          "enabled": true,
          "clientAuthenticatorType": "client-secret",
          "redirectUris": [
            "/c/oidc/callback"
          ],
          "webOrigins": [
            "+"
          ],
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
          "defaultClientScopes": [
            "roles",
            "profile",
            "email",
            "groups",
            "basic"
          ],
          "optionalClientScopes": [
            "offline_access"
          ]
        }
      ],
      "clientScopes": [
        {
          "name": "openid",
          "protocol": "openid-connect",
          "attributes": {
            "include.in.token.scope": "true"
          }
        },
        {
          "name": "profile",
          "protocol": "openid-connect",
          "attributes": {
            "include.in.token.scope": "true"
          }
        },
        {
          "name": "email",
          "protocol": "openid-connect",
          "attributes": {
            "include.in.token.scope": "true"
          }
        },
        {
          "name": "basic",
          "protocol": "openid-connect",
          "attributes": {
            "include.in.token.scope": "true"
          }
        },
        {
          "name": "offline_access",
          "protocol": "openid-connect",
          "attributes": {
            "include.in.token.scope": "true"
          }
        },
        {
          "name": "groups",
          "description": "Groups scope",
          "type": "Optional",
          "protocol": "openid-connect",
          "attributes": {
            "include.in.token.scope": "true",
            "display.on.consent.screen": "true"
          },
          "protocolMappers": [
            {
              "name": "groups",
              "protocol": "openid-connect",
              "protocolMapper": "oidc-group-membership-mapper",
              "consentRequired": false,
              "config": {
                "multivalued": "true",
                "full.path": "false",
                "id.token.claim": "true",
                "access.token.claim": "true",
                "claim.name": "groups",
                "userinfo.token.claim": "true",
                "jsonType.label": "String"
              }
            }
          ]
        },
        {
          "name": "roles",
          "description": "OpenID Connect scope for add user roles to the access token",
          "protocol": "openid-connect",
          "attributes": {
            "include.in.token.scope": "true",
            "display.on.consent.screen": "true",
            "gui.order": "",
            "consent.screen.text": '{{"$"}}{{"{"}}roleScopeConsentText{{"}"}}'
          },
          "protocolMappers": [
            {
              "name": "realm roles",
              "protocol": "openid-connect",
              "protocolMapper": "oidc-usermodel-realm-role-mapper",
              "consentRequired": false,
              "config": {
                "multivalued": "true",
                "userinfo.token.claim": "true",
                "id.token.claim": "true",
                "access.token.claim": "true",
                "claim.name": "realm_access.roles",
                "jsonType.label": "String"
              }
            },
            {
              "name": "client roles",
              "protocol": "openid-connect",
              "protocolMapper": "oidc-usermodel-client-role-mapper",
              "consentRequired": false,
              "config": {
                "multivalued": "true",
                "userinfo.token.claim": "true",
                "id.token.claim": "true",
                "access.token.claim": "true",
                "claim.name": "resource_access.{{"$"}}{{"{"}}client_id{{"}"}}.roles",
                "jsonType.label": "String"
              }
            }
          ]
        }
      ],
      "groups": [
        {
          "name": "registry-app-admin-group",
          "path": "/registry-app-admin-group",
        },
        {
          "name": "registry-app-editor-group",
          "path": "/registry-app-editor-group",
        },
        {
          "name": "registry-app-viewer-group",
          "path": "/registry-app-viewer-group",
        },
        {
          "name": "apps-m2m-service-account",
          "path": "/apps-m2m-service-account",
          "realmRoles": [
            "ao-m2m-rw",
            "co-m2m-rw"
          ]
        },
        {
          "name": "org-admin-group",
          "path": "/org-admin-group",
          "realmRoles": [
            "org-read-role",
            "org-update-role",
            "org-delete-role",
            "org-write-role"
          ]
        },
        {
          "name": "sre-admin-group",
          "path": "/sre-admin-group",
          "realmRoles": [
            "alrt-r"
          ],
          "clientRoles": {
            "account": [
              "view-profile",
              "manage-account"
            ],
            "telemetry-client": [
              "viewer"
            ]
          }
        },
        {
          "name": "iam-admin-group",
          "path": "/iam-admin-group",
          "realmRoles": [
            "admin",
            "secrets-root-role"
          ],
          "clientRoles": {
            "account": [
              "view-profile",
              "manage-account"
            ],
            "master-realm": [
              "view-users",
              "query-users",
              "manage-clients"
            ]
          }
        },
        {
          "name": "service-admin-group",
          "path": "/service-admin-group",
          "realmRoles": [
            "alrt-rx-rw",
            "rs-access-r",
            "infra-manager-core-read-role",
            "infra-manager-core-write-role",
            "alrt-rw"
          ],
          "clientRoles": {
            "account": [
              "view-profile",
              "manage-account"
            ],
            "master-realm": [
              "view-users",
              "query-users",
              "manage-clients"
            ],
            "telemetry-client": [
              "admin"
            ],
            "cluster-management-client": [
              "restricted-role",
              "standard-role",
              "base-role"
            ],
            "registry-client": [
              "registry-admin-role"
            ]
          }
        },
        {
          "name": "edge-manager-group",
          "path": "/edge-manager-group",
          "realmRoles": [
            "app-service-proxy-read-role",
            "app-service-proxy-write-role",
            "app-deployment-manager-read-role",
            "app-deployment-manager-write-role",
            "app-resource-manager-read-role",
            "app-resource-manager-write-role",
            "app-vm-console-write-role",
            "catalog-publisher-read-role",
            "catalog-publisher-write-role",
            "catalog-other-read-role",
            "catalog-other-write-role",
            "catalog-restricted-read-role",
            "catalog-restricted-write-role",
            "clusters-read-role",
            "clusters-write-role",
            "cluster-templates-read-role",
            "cluster-templates-write-role",
            "cluster-artifacts-read-role",
            "cluster-artifacts-write-role",
            "infra-manager-core-read-role",
            "alrt-rw"
          ],
          "clientRoles": {
            "telemetry-client": [
              "viewer"
            ],
            "cluster-management-client": [
              "standard-role",
              "base-role"
            ],
            "registry-client": [
              "registry-editor-role"
            ]
          }
        },
        {
          "name": "edge-operator-group",
          "path": "/edge-operator-group",
          "realmRoles": [
            "app-service-proxy-read-role",
            "app-service-proxy-write-role",
            "app-deployment-manager-read-role",
            "app-deployment-manager-write-role",
            "app-resource-manager-read-role",
            "app-resource-manager-write-role",
            "app-vm-console-write-role",
            "catalog-publisher-read-role",
            "catalog-other-read-role",
            "clusters-read-role",
            "clusters-write-role",
            "cluster-templates-read-role",
            "cluster-artifacts-read-role",
            "cluster-artifacts-write-role",
            "infra-manager-core-read-role",
            "alrt-r"
          ],
          "clientRoles": {
            "telemetry-client": [
              "viewer"
            ],
            "registry-client": [
              "registry-viewer-role"
            ]
          }
        },
        {
          "name": "host-manager-group",
          "path": "/host-manager-group",
          "realmRoles": [
            "infra-manager-core-read-role",
            "infra-manager-core-write-role"
          ],
          "clientRoles": {
            "telemetry-client": [
              "viewer"
            ]
          }
        },
        {
          "name": "sre-group",
          "path": "/sre-group",
          "realmRoles": [
            "alrt-r",
            "clusters-read-role",
            "clusters-write-role",
            "cluster-templates-read-role",
            "infra-manager-core-read-role"
          ],
          "clientRoles": {
            "telemetry-client": [
              "viewer"
            ],
            "cluster-management-client": [
              "base-role",
              "restricted-role"
            ]
          }
        }
      ],
      "users": [
        {
          "username": "service-account-alerts-m2m-client",
          "enabled": true,
          "totp": false,
          "serviceAccountClientId": "alerts-m2m-client",
          "realmRoles": [
            "default-roles-master"
          ],
          "clientRoles": {
            "alerts-m2m-client": [
              "uma_protection"
            ],
            "master-realm": [
              "view-users"
            ]
          },
          "notBefore": 0
        },
        {
          "username": "service-account-host-manager-m2m-client",
          "enabled": true,
          "totp": false,
          "serviceAccountClientId": "host-manager-m2m-client",
          "realmRoles": [
            "default-roles-master",
            "rs-access-r"
          ],
          "clientRoles": {
            "host-manager-m2m-client": [
              "uma_protection"
            ],
            "master-realm": [
              "query-clients",
              "manage-authorization",
              "view-clients",
              "view-users",
              "create-client",
              "manage-users",
              "manage-clients",
              "view-realm"
            ]
          },
          "notBefore": 0
        },
        {
          "username": "service-account-co-manager-m2m-client",
          "enabled": true,
          "totp": false,
          "serviceAccountClientId": "co-manager-m2m-client",
          "realmRoles": [
            "default-roles-master"
          ],
          "clientRoles": {
            "co-manager-m2m-client": [
              "uma_protection"
            ],
            "master-realm": [
              "view-clients",
              "manage-clients"
            ]
          },
          "notBefore": 0,
          "groups": [
            "/edge-manager-group",
            "/apps-m2m-service-account"
          ]
        },
        {
          "username": "service-account-ktc-m2m-client",
          "enabled": true,
          "totp": false,
          "serviceAccountClientId": "ktc-m2m-client",
          "realmRoles": [
            "admin",
            "create-realm",
            "default-roles-master",
            "rs-access-r"
          ],
          "clientRoles": {
            "ktc-m2m-client": [
              "uma_protection"
            ],
            "master-realm": [
              "query-clients",
              "manage-authorization",
              "view-clients",
              "view-users",
              "create-client",
              "manage-users",
              "manage-clients"
            ]
          },
          "notBefore": 0
        },
        {
          "username": "service-account-3rd-party-host-manager-m2m-client",
          "enabled": true,
          "totp": false,
          "serviceAccountClientId": "3rd-party-host-manager-m2m-client",
          "realmRoles": [
            "default-roles-master",
            "rs-access-r"
          ],
          "clientRoles": {
            "3rd-party-host-manager-m2m-client": [
              "uma_protection"
            ],
            "master-realm": [
              "query-clients",
              "manage-authorization",
              "view-clients",
              "view-users",
              "create-client",
              "manage-users",
              "manage-clients",
              "view-realm",
            ]
          },
          "notBefore": 0
        },
        {
          "username": "service-account-en-m2m-template-client",
          "enabled": true,
          "totp": false,
          "serviceAccountClientId": "en-m2m-template-client",
          "realmRoles": [
            "default-roles-master",
            "rs-access-r",
            "en-agent-rw"
          ],
          "clientRoles": {
            "en-m2m-template-client": [
              "uma_protection"
            ]
          },
          "notBefore": 0
        },
        {
          "username": "service-account-edge-manager-m2m-client",
          "enabled": true,
          "totp": false,
          "serviceAccountClientId": "edge-manager-m2m-client",
          "realmRoles": [
            "default-roles-master"
          ],
          "clientRoles": {
            "edge-manager-m2m-client": [
              "uma_protection"
            ]
          },
          "notBefore": 0,
          "groups": [
            "/edge-manager-group",
            "/apps-m2m-service-account"
          ]
        },
      ],
      "components": {
        "org.keycloak.keys.KeyProvider": [
          {
            "name": "fallback-PS512",
            "providerId": "rsa-generated",
            "subComponents": {},
            "config": {
              "keySize": [
                "4096"
              ],
              "active": [
                "true"
              ],
              "priority": [
                "-100"
              ],
              "enabled": [
                "true"
              ],
              "algorithm": [
                "PS512"
              ]
            }
          }
        ]
      }
    }
