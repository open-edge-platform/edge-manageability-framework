# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

cluster:
  postgresql:
    parameters:
      huge_pages: 'off'
      {{- if .Values.argo.postgresql }}
      max_connections: {{ .Values.argo.postgresql.maxConnections | default 200 }}
      shared_buffers: {{.Values.argo.postgresql.sharedBuffers | default "24MB"}}
      {{- end }}
  bootstrap:
    initdb:
      database: postgres
      owner: postgres
      secret:
        name: my-postgres-credentials
      postInitSQL:
      {{- range .Values.argo.database.databases }}
      - CREATE DATABASE {{ printf "%s-%s" .namespace .name }};
      - BEGIN; REVOKE CREATE ON SCHEMA public FROM PUBLIC; REVOKE ALL ON DATABASE {{ printf "%s-%s" .namespace .name }} FROM PUBLIC;
      - CREATE USER {{ printf "%s-%s_user" .namespace .name }} WITH PASSWORD '{{ .password }}';
      - GRANT CONNECT ON DATABASE {{ printf "%s-%s" .namespace .name }} TO {{ printf "%s-%s_user" .namespace .name }};      
      - GRANT ALL PRIVILEGES ON DATABASE {{ printf "%s-%s" .namespace .name }} TO {{ printf "%s-%s_user" .namespace .name }};
      - ALTER DATABASE {{ printf "%s-%s" .namespace .name }} OWNER TO {{ printf "%s-%s_user" .namespace .name }};
      - COMMIT;
      {{- end }}