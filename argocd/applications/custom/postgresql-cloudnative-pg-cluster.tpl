# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

cluster:
  env:
    {{- range .Values.argo.database.databases }}
    - name: {{ printf "%s-%s_user_password" .namespace .name | replace "-" "_" | upper }}
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
      huge_pages: "off"
      {{- if .Values.argo.postgresql }}
      max_connections: {{ .Values.argo.postgresql.maxConnections | default "200" }}
      shared_buffers: {{.Values.argo.postgresql.sharedBuffers | default "24MB"}}
      {{- end }}
  initdb:
    database: postgres
    owner: postgres
    postInitSQL:
    - |
      CREATE OR REPLACE FUNCTION setup_database_and_user(
        db_name TEXT,
        user_name TEXT,
        env_var_name TEXT
      ) RETURNS VOID AS $setup$ DECLARE
          password_val TEXT;
          temp_table TEXT;
      BEGIN
        temp_table := 'temp_' || replace(replace(env_var_name, '-', '_'), '.', '_');
        EXECUTE format('CREATE TEMP TABLE %I (value TEXT)', temp_table);
        EXECUTE format('COPY %I FROM PROGRAM ''printenv %s || echo NOTFOUND''', temp_table, env_var_name);
        EXECUTE format('SELECT trim(value) FROM %I WHERE value != ''NOTFOUND'' AND value != '''' LIMIT 1', temp_table) INTO password_val;
        EXECUTE format('DROP TABLE %I', temp_table);
        IF password_val IS NULL THEN
          RAISE EXCEPTION 'Environment variable % not found or empty', env_var_name;
        END IF;
        EXECUTE format('CREATE USER %I WITH PASSWORD %L', user_name, password_val);
      END; $setup$ LANGUAGE plpgsql;
    {{- range .Values.argo.database.databases }}
    {{- $dbName := printf "%s-%s" .namespace .name }}
    {{- $userName := printf "%s-%s_user" .namespace .name }}
    {{- $password := printf "%s_password" $userName | replace "-" "_" | upper }}
    - CREATE DATABASE "{{ $dbName }}";
    - SELECT setup_database_and_user('{{ $dbName }}', '{{ $userName }}', '{{ $password }}');
    - |-
      BEGIN;
      REVOKE CREATE ON SCHEMA public FROM PUBLIC;
      REVOKE ALL ON DATABASE "{{ $dbName }}" FROM PUBLIC;
      GRANT CONNECT ON DATABASE "{{ $dbName }}" TO "{{ $userName }}";
      GRANT ALL PRIVILEGES ON DATABASE "{{ $dbName }}" TO "{{ $userName }}";
      ALTER DATABASE "{{ $dbName }}" OWNER TO "{{ $userName }}";
      COMMIT;
    {{- end }}
