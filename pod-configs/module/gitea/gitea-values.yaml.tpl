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
  primary:
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
    persistence:
      storageClass: "efs-1000"
    resourcesPreset: none
    resource: {}
  persistence:
    size: 1Gi
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
