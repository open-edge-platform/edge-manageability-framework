# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

## Cluster-Specific values
## These values are not part of bitnami helm chart and are used to parameterize substrings in
## the larger keycloakConfigCli.configuration.realm-master.json value.
## @param clusterSpecific.webuiClientRootUrl The Keycloak Master realm UI Client's rootUrl value as a quoted JSON string
## @param clusterSpecific.webuiRedirectUrls The Keycloak Master realm UI Client's reirectUrl values as a JSON array of quoted JSON strings
## @param clusterSpecific.registryClientRootUrl The Keycloak Master realm Harbor Client's rootUrl value as a quoted JSON string
## @param clusterSpecific.telemetryClientRootUrl The Keycloak Master realm Grafana Client's rootUrl value as a quoted JSON string
## @param clusterSpecific.telemetryRedirectUrls The Keycloak Master realm Grafana Client's reirectUrl values as a JSON array of quoted JSON strings
clusterSpecific:
  webuiClientRootUrl: "https://web-ui.{{ .Values.argo.clusterDomain }}"
  webuiRedirectUrls: ["https://web-ui.{{ .Values.argo.clusterDomain }}", "https://app-service-proxy.{{ .Values.argo.clusterDomain }}/app-service-proxy-index.html*", "https://vnc.{{ .Values.argo.clusterDomain }}/*", "https://{{ .Values.argo.clusterDomain }}"{{- if index .Values.argo "platform-keycloak" "extraUiRedirects" -}}, {{- index .Values.argo "platform-keycloak" "extraUiRedirects" -}}{{- end -}}]
  registryClientRootUrl: "https://registry-oci.{{ .Values.argo.clusterDomain }}"
  telemetryClientRootUrl: "https://observability-ui.{{ .Values.argo.clusterDomain }}"
  telemetryRedirectUrls: ["https://observability-admin.{{ .Values.argo.clusterDomain }}/login/generic_oauth", "https://observability-ui.{{ .Values.argo.clusterDomain }}/login/generic_oauth"]

## External PostgreSQL configuration
## All of these values are only used when postgresql.enabled is set to false
## @param externalDatabase.existingSecret Name of an existing secret resource containing the database credentials
## @param externalDatabase.existingSecretHostKey Name of an existing secret key containing the database host name
## @param externalDatabase.existingSecretPortKey Name of an existing secret key containing the database port
## @param externalDatabase.existingSecretUserKey Name of an existing secret key containing the database user
## @param externalDatabase.existingSecretDatabaseKey Name of an existing secret key containing the database name
## @param externalDatabase.existingSecretPasswordKey Name of an existing secret key containing the database credentials
externalDatabase:
  existingSecret: platform-keycloak-{{.Values.argo.database.type}}-postgresql
  existingSecretHostKey: PGHOST
  existingSecretPortKey: PGPORT
  existingSecretUserKey: PGUSER
  existingSecretDatabaseKey: PGDATABASE
  existingSecretPasswordKey: PGPASSWORD

# Use index to handle values with hyphen
{{- if index .Values.argo "platform-keycloak" "localRegistrySize"}}
persistence:
  persistentVolumeClaim:
    registry:
      size: {{index .Values.argo "platform-keycloak" "localRegistrySize"}}
{{- end}}

extraEnvVars:
  - name: HTTPS_PROXY
    value: {{.Values.argo.proxy.httpsProxy}}
  - name: HTTP_PROXY
    value: {{.Values.argo.proxy.httpProxy}}
  - name: NO_PROXY
    value: {{.Values.argo.proxy.noProxy}}
  {{ if index .Values.argo "platform-keycloak" "db" }}
  - name: KC_DB_POOL_INITIAL_SIZE
    value: {{ index .Values.argo "platform-keycloak" "db" "poolInitSize" | default "5" | quote}}
  - name: KC_DB_POOL_MIN_SIZE
    value: {{ index .Values.argo "platform-keycloak" "db" "poolMinSize" | default "5" | quote}}
  - name: KC_DB_POOL_MAX_SIZE
    value: {{ index .Values.argo "platform-keycloak" "db" "poolMaxSize" | default "100" | quote}}
  {{ end }}
  - name: KC_PROXY_HEADERS
    value: "xforwarded"

{{- with .Values.argo.resources.platformKeycloak }}
resources:
  {{- toYaml . | nindent 2}}
{{- end }}
