# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

cluster:
  {{- $defaultResources := dict }}
  {{- if and .Values.argo.postgresql .Values.argo.postgresql.resourcesPreset }}
  {{- $preset := .Values.argo.postgresql.resourcesPreset }}
  {{- if eq $preset "large" }}
  {{- $defaultResources = dict "requests" (dict "memory" "2048Mi" "cpu" "1.0") "limits" (dict "memory" "3072Mi" "cpu" "1.5") }}
  {{- else }}
  {{- $defaultResources = dict "requests" (dict "memory" "256Mi" "cpu" "250m") "limits" (dict "memory" "384Mi" "cpu" "375m") }}
  {{- end }}
  {{- end }}

  # Following resource setting will override resourcesPreset
  {{- $finalResources := $defaultResources }}
  {{- if .Values.argo.resources.postgresql.cluster }}
  {{- $finalResources = .Values.argo.resources.postgresql.cluster }}
  {{- end }}

  {{- if $finalResources }}
  resources:
    {{- toYaml $finalResources | nindent 4 }}
  {{- end }}

  roles:
    {{- range .Values.argo.database.databases }}
    {{- $secretName := printf "%s-%s" .namespace .name }}
    {{- $userName := printf "%s-%s_user" .namespace .name }}
    - ensure: present
      login: true
      name: {{ $userName }}
      passwordSecret:
        name: {{ $secretName }}
    {{- end }}
  postgresql:
    parameters:
      huge_pages: "off"
      {{- if .Values.argo.postgresql }}
      max_connections: {{ .Values.argo.postgresql.maxConnections | default "200" | quote }}
      shared_buffers: {{.Values.argo.postgresql.sharedBuffers | default "24MB"}}
      {{- end }}
  initdb:
    database: postgres
    owner: orch-database-postgresql_user
    localeCType: "en_US.UTF-8"
    localeCollate: "en_US.UTF-8"
    secret:
      name: "orch-database-postgresql"
    postInitSQL:
    {{- range .Values.argo.database.databases }}
    {{- $dbName := printf "%s-%s" .namespace .name }}
    {{- $userName := printf "%s-%s_user" .namespace .name }}
    - CREATE DATABASE "{{ $dbName }}";
    - BEGIN;
    - REVOKE CREATE ON SCHEMA public FROM PUBLIC;
    - REVOKE ALL ON DATABASE "{{ $dbName }}" FROM PUBLIC;
    - CREATE ROLE "{{ $userName }}";
    - GRANT CONNECT ON DATABASE "{{ $dbName }}" TO "{{ $userName }}";
    - GRANT ALL PRIVILEGES ON DATABASE "{{ $dbName }}" TO "{{ $userName }}";
    - ALTER DATABASE "{{ $dbName }}" OWNER TO "{{ $userName }}";
    - COMMIT;
    {{- end }}
