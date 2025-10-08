# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

cluster:
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
  services:
    additional:
      - selectorType: rw
        serviceTemplate:
          metadata:
            name: postgresql
          spec:
            type: ClusterIP
    disabledDefaultServices: ["ro", "r"]
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
    secret:
      name: "orch-database-postgresql"
    postInitSQL:
    {{- range .Values.argo.database.databases }}
    {{- $dbName := printf "%s-%s" .namespace .name }}
    {{- $userName := printf "%s-%s_user" .namespace .name }}
    - CREATE ROLE "{{ $userName }}";
    - CREATE DATABASE "{{ $dbName }}";
    - BEGIN;
    - REVOKE CREATE ON SCHEMA public FROM PUBLIC;
    - REVOKE ALL ON DATABASE "{{ $dbName }}" FROM PUBLIC;
    - GRANT CONNECT ON DATABASE "{{ $dbName }}" TO "{{ $userName }}";
    - GRANT ALL PRIVILEGES ON DATABASE "{{ $dbName }}" TO "{{ $userName }}";
    - ALTER DATABASE "{{ $dbName }}" OWNER TO "{{ $userName }}";
    - COMMIT;
    {{- end }}
