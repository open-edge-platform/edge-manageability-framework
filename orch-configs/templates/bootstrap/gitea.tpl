# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

image:
{{- if .Values.orchestratorDeployment.dockerCache }}
  registry: {{ .Values.orchestratorDeployment.dockerCache }}
{{- end }}
  pullPolicy: IfNotPresent
  rootless: true
containerSecurityContext:
  allowPrivilegeEscalation: false
  capabilities:
    drop:
      - ALL
  seccompProfile:
    type: RuntimeDefault
  runAsNonRoot: true
postgresql-ha:
  enabled: false
postgresql:
  enabled: true
  image:
    registry: docker.io
    repository: library/postgres
    tag: 16.10-bookworm
  postgresqlDataDir: /var/postgres/data
  primary:
    extraEnvVars:
    - name: HOME
      value: /var/postgres
    resourcesPreset: none
    resource: {}
    containerSecurityContext:
      allowPrivilegeEscalation: false
      capabilities:
        drop:
          - ALL
      seccompProfile:
        type: RuntimeDefault
    extraVolumeMounts:
    - name: postgresql-run
      mountPath: /var/run
    - name: postgresql-config
      mountPath: /var/postgres/data/postgresql.conf
      subPath: postgresql.conf
    - name: postgresql-hba
      mountPath: /var/postgres/data/pg_hba.conf
      subPath: pg_hba.conf
    extraVolumes:
    - name: postgresql-run
      emptyDir: {}
    - name: postgresql-config
      configMap:
        name: postgresql-config
    - name: postgresql-hba
      configMap:
        name: postgresql-hba
  extraDeploy:
  - apiVersion: v1
    kind: ConfigMap
    metadata:
      name: postgresql-config
    data:
      postgresql.conf: |-
        huge_pages = 'off'
        listen_addresses = '*'
        port = 5432
        max_connections = 100
        shared_buffers = 128MB
        dynamic_shared_memory_type = posix
        max_wal_size = 1GB
        min_wal_size = 80MB
        log_timezone = UTC
        datestyle = 'iso, mdy'
        timezone = UTC
        lc_messages = 'en_US.utf8'
        lc_monetary = 'en_US.utf8'
        lc_time = 'en_US.utf8'
        default_text_search_config = 'pg_catalog.english'
  - apiVersion: v1
    kind: ConfigMap
    metadata:
      name: postgresql-hba
    data:
      pg_hba.conf: |-
        # TYPE  DATABASE        USER            ADDRESS                 METHOD
        local   all             all                                     trust
        host    all             all             127.0.0.1/32            trust
        host    all             all             ::1/128                 trust
        local   replication     all                                     trust
        host    replication     all             127.0.0.1/32            trust
        host    replication     all             ::1/128                 trust
  persistence:
    size: 1Gi
    mountPath: /var/postgres
  containerSecurityContext:
    runAsUser: 1000
  podSecurityContext:
    enabled: true
    fsGroup: 1000
persistence:
  enabled: true
  size: 1Gi
ingress:
  enabled: false
redis:
  enabled: true
  image:
    registry: docker.io
    repository: library/redis
    tag: 7.2.11
  master:
    resourcesPreset: none
    resources: {}
redis-cluster:
  enabled: false
extraContainerVolumeMounts:
  - name: secret-volume
    readOnly: true
    mountPath: /tmp/secret-volume
extraVolumes:
  - name: secret-volume
    secret:
      secretName: gitea-tls-certs
service:
  http:
    type: LoadBalancer
    port: 443
gitea:
  config:
    server:
      APP_DATA_PATH: /data
      DOMAIN: gitea.kind.internal
      HTTP_PORT: 3000
      PROTOCOL: https
      ROOT_URL: "https://gitea.kind.internal:3000"
      CERT_FILE: /tmp/secret-volume/tls.crt
      KEY_FILE: /tmp/secret-volume/tls.key
    repository:
      DEFAULT_PUSH_CREATE_PRIVATE: true
      ENABLE_PUSH_CREATE_USER: true
    service:
      DISABLE_REGISTRATION: true
resources: {}
