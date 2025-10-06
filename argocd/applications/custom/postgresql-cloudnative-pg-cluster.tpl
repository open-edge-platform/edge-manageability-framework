# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

cluster:
  {{- with .Values.argo.resources.postgresql.cluster }}
  resources:
    {{- toYaml . | nindent 4 }}
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
  storage:
    size: {{ .Values.argo.postgresql.storageSize | default "1Gi" }}
    {{- if and .Values.argo.postgresql.persistence .Values.argo.postgresql.persistence.storageClass }}
    storageClass: {{ .Values.argo.postgresql.persistence.storageClass }}
    {{- end }}
  postgresql:
    parameters:
      huge_pages: "off"
      {{- if .Values.argo.postgresql }}
      max_connections: {{ .Values.argo.postgresql.maxConnections | default "200" }}
      shared_buffers: {{.Values.argo.postgresql.sharedBuffers | default "24MB"}}
      {{- end }}
  initdb:
    database: postgres
    owner: postgres
    postInitSQL:
    {{- range .Values.argo.database.databases }}
    {{- $dbName := printf "%s-%s" .namespace .name }}
    {{- $userName := printf "%s-%s_user" .namespace .name }}
    - CREATE DATABASE "{{ $dbName }}";
    - |-
      BEGIN;
      REVOKE CREATE ON SCHEMA public FROM PUBLIC;
      REVOKE ALL ON DATABASE "{{ $dbName }}" FROM PUBLIC;
      GRANT CONNECT ON DATABASE "{{ $dbName }}" TO "{{ $userName }}";
      GRANT ALL PRIVILEGES ON DATABASE "{{ $dbName }}" TO "{{ $userName }}";
      ALTER DATABASE "{{ $dbName }}" OWNER TO "{{ $userName }}";
      COMMIT;
    {{- end }}
