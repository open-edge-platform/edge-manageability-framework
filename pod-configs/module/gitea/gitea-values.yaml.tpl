# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

global:
  storageClass: "efs-1000"
image:
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
    initContainers:
    - name: init-config-check
      image: busybox:1.36
      command:
        - "sh"
        - "-c"
        - |
          if [ -s "/var/postgres/data/PG_VERSION" ]; then
            echo "Previous database detected. Installing postgresql.conf and pg_hba.conf."
            cp /var/postgres/postgresql.conf /var/postgres/data/postgresql.conf
            cp /var/postgres/pg_hba.conf /var/postgres/data/pg_hba.conf
          else
            echo "Fresh install. The official entrypoint will generate the default configuration."
          fi
      volumeMounts:
      - name: postgres-config
        mountPath: /var/postgres/postgresql.conf
        subPath: postgresql.conf
      - name: postgres-hba
        mountPath: /var/postgres/pg_hba.conf
        subPath: pg_hba.conf
      - name: data
        mountPath: /var/postgres
      containerSecurityContext:
        allowPrivilegeEscalation: false
        capabilities:
          drop:
          - ALL
        seccompProfile:
          type: RuntimeDefault
        runAsNonRoot: true
    extraEnvVars:
    - name: HOME
      value: /var/postgres
    containerSecurityContext:
      allowPrivilegeEscalation: false
      capabilities:
        drop:
          - ALL
      seccompProfile:
        type: RuntimeDefault
      # Storage class efs-1000 uses user 1000
      runAsUser: 1000
      runAsGroup: 1000
    extraVolumeMounts:
    - name: postgres-run
      mountPath: /var/run
    extraVolumes:
    - name: postgres-run
      emptyDir: {}
    - name: postgres-config
      configMap:
        name: postgres-config
    - name: postgres-hba
      configMap:
        name: postgres-hba
  extraDeploy:
  - apiVersion: v1
    kind: ConfigMap
    metadata:
      name: postgres-config
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
      name: postgres-hba
    data:
      pg_hba.conf: |-
        # TYPE  DATABASE        USER            ADDRESS                 METHOD
        local   all             all                                     trust
        host    all             all             127.0.0.1/32            trust
        host    all             all             ::1/128                 trust
        local   replication     all                                     trust
        host    replication     all             127.0.0.1/32            trust
        host    replication     all             ::1/128                 trust
        host all all all scram-sha-256
    persistence:
      storageClass: "efs-1000"
    resourcesPreset: none
    resource: {}
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
  annotations:
    helm.sh/resource-policy: ""
  storageClass: "efs-1000"
ingress:
  enabled: false
redis:
  enabled: true
  image:
    registry: docker.io
    repository: library/redis
    tag: 7.2.11
  master:
    persistence:
      storageClass: "efs-1000"
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
    type: ClusterIP
    port: 443
    clusterIP: ""
gitea:
  admin:
    password: ${gitea_password}
    passwordMode: initialOnlyNoReset
  config:
    server:
      APP_DATA_PATH: /data
      DOMAIN: "${gitea_domain}"
      HTTP_PORT: 3000
      PROTOCOL: https
      ROOT_URL: "https://${gitea_domain}"
      CERT_FILE: /tmp/secret-volume/tls.crt
      KEY_FILE: /tmp/secret-volume/tls.key
    repository:
      DEFAULT_PUSH_CREATE_PRIVATE: true
      ENABLE_PUSH_CREATE_USER: true
    service:
      DISABLE_REGISTRATION: true
podDisruptionBudget:
  maxUnavailable: 1
# Uncomment following settings when switching to RDS
# Also need to update the value for database endpoint info and credential
#     database:
#       DB_TYPE: "postgres"
#       HOST: "gitea_database_endpoint"
#       NAME: "gitea_database"
#       USER: "gitea_database_username"
#       PASSWD: "gitea_database_password"
#       SSL_MODE: "require"
resources: {}
