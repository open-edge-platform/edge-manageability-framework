# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

cluster:
  env:
    {{- range .Values.argo.database.databases }}
    - name: {{ .name | upper }}_PASSWORD
      valueFrom:
        secretKeyRef:
          name: passwords
          key: {{ .name }}
    {{- end }}
  storage:
    size: {{ .Values.argo.postgresql.storageSize | default "1Gi" }}
    {{- if and .Values.argo.postgresql.persistence .Values.argo.postgresql.persistence.storageClass }}
    storageClass: {{ .Values.argo.postgresql.persistence.storageClass }}
    {{- end }}
  postgresql:
    parameters:
      huge_pages: 'off'
      {{- if .Values.argo.postgresql }}
      max_connections: {{ .Values.argo.postgresql.maxConnections | default 200 }}
      shared_buffers: {{.Values.argo.postgresql.sharedBuffers | default "24MB"}}
      {{- end }}
      {{- range .Values.argo.database.databases }}
      {{ .name }}_password: "${{ .name | upper }}_PASSWORD"
      {{ .name }}_db_name: "${{ .name | upper }}_DB_NAME"
      {{ .name }}_user_name: "${{ .name | upper }}_USER_NAME"
      {{- end }}
  bootstrap:
    initdb:
      database: postgres
      owner: postgres
      postInitApplicationSQL:
        - |
          {{- range .Values.argo.database.databases }}
          {{- $dbName := printf "%s-%s" .namespace .name }}
          {{- $userName := printf "%s-%s_user" .namespace .name }}
          -- Create database {{ $dbName }}
          DO $$
          BEGIN
              IF NOT EXISTS (SELECT FROM pg_database WHERE datname = '{{ $dbName }}') THEN
                  CREATE DATABASE "{{ $dbName }}";
              END IF;
          END
          $$;
          
          -- Create user with password from environment
          DO $$
          BEGIN
              IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '{{ $userName }}') THEN
                  -- Use environment variable for password
                  EXECUTE format('CREATE USER %I WITH PASSWORD %L', '{{ $userName }}', current_setting('{{ .name }}_password'));
              END IF;
          END
          $$;
          
          -- Setup permissions
          REVOKE CREATE ON SCHEMA public FROM PUBLIC;
          REVOKE ALL ON DATABASE "{{ $dbName }}" FROM PUBLIC;
          GRANT CONNECT ON DATABASE "{{ $dbName }}" TO "{{ $userName }}";
          GRANT ALL PRIVILEGES ON DATABASE "{{ $dbName }}" TO "{{ $userName }}";
          ALTER DATABASE "{{ $dbName }}" OWNER TO "{{ $userName }}";
          {{- end }}
