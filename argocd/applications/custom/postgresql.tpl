# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

primary:
  {{- if and .Values.argo.postgresql .Values.argo.postgresql.resourcesPreset }}
  resourcesPreset: {{ .Values.argo.postgresql.resourcesPreset | quote }}
  {{- else }}
  resourcesPreset: "micro"
  {{- end }}
  # Following resource setting will override resourcesPreset
  {{- with .Values.argo.resources.postgresql.primary }}
  resources:
    {{- toYaml . | nindent 4 }}
  {{- end }}
  persistentVolumeClaimRetentionPolicy:
    enabled: true
    whenDeleted: Delete
  extendedConfiguration: |-
    huge_pages = off
    {{- if .Values.argo.postgresql }}
    max_connections = {{ .Values.argo.postgresql.maxConnections | default 200 }}
    shared_buffers = {{.Values.argo.postgresql.sharedBuffers | default "24MB"}}
    {{- end }}
  extraVolumes:
  - name: passwords
    secret:
      secretName: passwords
  extraVolumeMounts:
  - name: passwords
    readOnly: true
    mountPath: "/var/lib/postgres/pgpass"
  initdb:
    scripts:
    {{- range .Values.argo.database.databases }}
    {{ $db := printf "\\\"%s-%s\\\"" .namespace .name}}
    {{ $user := printf "\\\"%s-%s_user\\\"" .namespace .name }}
      create_{{ .name }}.sh: |
        #!/bin/sh
        export PGPASSWORD="$POSTGRES_PASSWORD"
        password=$(cat /var/lib/postgres/pgpass/{{ .name }})
        psql -c "CREATE DATABASE {{ $db }}"
        psql -c "BEGIN; REVOKE CREATE ON SCHEMA public FROM PUBLIC; REVOKE ALL ON DATABASE {{ $db }} FROM PUBLIC; CREATE USER {{ $user }} WITH PASSWORD '$password'; \
          GRANT CONNECT ON DATABASE {{ $db }} TO {{ $user }}; GRANT ALL PRIVILEGES ON DATABASE {{ $db }} TO {{ $user }}; \
          ALTER DATABASE {{ $db }} OWNER TO {{ $user }};COMMIT;"
    {{- end }}
