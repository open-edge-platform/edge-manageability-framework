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
      CREATE OR REPLACE FUNCTION get_env_var(env_name TEXT) 
      RETURNS TEXT AS $getenv$
      DECLARE
          temp_table_name TEXT;
          password_val TEXT;
      BEGIN
          temp_table_name := 'temp_env_' || replace(env_name, '-', '_');
          EXECUTE format('CREATE TEMP TABLE %I (value TEXT)', temp_table_name);
          EXECUTE format('COPY %I FROM PROGRAM ''printenv %s || echo NOTFOUND''', temp_table_name, env_name);
          EXECUTE format('SELECT trim(value) FROM %I LIMIT 1', temp_table_name) INTO password_val;
          EXECUTE format('DROP TABLE %I', temp_table_name);
          
          IF password_val = 'NOTFOUND' OR password_val = '' THEN
              RETURN NULL;
          END IF;
          
          RETURN password_val;
      END;
      $getenv$ LANGUAGE plpgsql;
    - |
      CREATE OR REPLACE FUNCTION setup_database_and_user(
          db_name TEXT,
          username TEXT,
          env_var_name TEXT
      ) RETURNS VOID AS $setup$
      DECLARE
          password_val TEXT;
      BEGIN
          password_val := get_env_var(env_var_name);
          
          IF password_val IS NULL THEN
              RAISE EXCEPTION 'Environment variable % not found', env_var_name;
          END IF;
          
          EXECUTE 'REVOKE CREATE ON SCHEMA public FROM PUBLIC';
          EXECUTE format('REVOKE ALL ON DATABASE %I FROM PUBLIC', db_name);
          EXECUTE format('CREATE USER %I WITH PASSWORD %L', username, password_val);
          EXECUTE format('GRANT CONNECT ON DATABASE %I TO %I', db_name, username);
          EXECUTE format('GRANT ALL PRIVILEGES ON DATABASE %I TO %I', db_name, username);
          EXECUTE format('ALTER DATABASE %I OWNER TO %I', db_name, username);
      END;
      $setup$ LANGUAGE plpgsql;
    {{- range .Values.argo.database.databases }}
    {{- $dbName := printf "%s-%s" .namespace .name }}
    {{- $userName := printf "%s-%s_user" .namespace .name }}
    {{- $password := printf "%s_password" $userName | replace "-" "_" | upper }}
    - CREATE DATABASE "{{ $dbName }}";
    - SELECT setup_database_and_user("{{ $dbName }}", "{{ $userName }}", "{{ $password }}");
    {{- end }}
