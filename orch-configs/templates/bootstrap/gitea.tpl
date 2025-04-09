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
  primary:
    resourcesPreset: none
    resource: {}
    containerSecurityContext:
      allowPrivilegeEscalation: false
      capabilities:
        drop:
          - ALL
      seccompProfile:
        type: RuntimeDefault
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
ingress:
  enabled: false
redis:
  enabled: true
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
