# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

clusterManager:
  args:
    clusterdomain: {{ .Values.argo.clusterDomain }}
    # JWT TTL configuration - this triggers the M2M credential usage
    # If kubeconfig-ttl-hours=0 access token lifespan inherits from
    # keycloak realm settings and upbounded by the SSO sessions max: 12h
    kubeconfig-ttl-hours: 8
  image:
    repository: cluster/cluster-manager
    registry:
      name: {{ .Values.argo.containerRegistryURL }}
      imagePullSecrets:
      {{- with .Values.argo.imagePullSecrets }}
        {{- toYaml . | nindent 6 }}
      {{- end }}
  {{- with .Values.argo.resources.clusterManager.clusterManager }}
  resources:
    {{- toYaml . | nindent 4 }}
  {{- end }}
templateController:
  image:
    repository: cluster/template-controller
    registry:
      name: {{ .Values.argo.containerRegistryURL }}
      imagePullSecrets:
      {{- with .Values.argo.imagePullSecrets }}
        {{- toYaml . | nindent 6 }}
      {{- end }}
  {{- with .Values.argo.resources.clusterManager.templateController }}
  resources:
    {{- toYaml . | nindent 4 }}
  {{- end }}

# Co-manager M2M Client Automation Configuration
credentialsAutomation:
  enabled: true

  # Job configuration for credential automation
  job:
    image:
      repository: curlimages/curl
      tag: "8.4.0"
      pullPolicy: IfNotPresent

    # Job execution settings
    backoffLimit: 3
    activeDeadlineSeconds: 300
    ttlSecondsAfterFinished: 86400  # 24 hours

    # Retry configuration
    retryAttempts: 5
    retryDelay: 10

    resources:
      limits:
        cpu: 100m
        memory: 128Mi
      requests:
        cpu: 50m
        memory: 64Mi

  # Platform services configuration
  vault:
    service: "vault.orch-platform.svc"
    port: 8200
    secretPath: "secret/data/co-manager-m2m-client-secret"
    authPath: "auth/kubernetes"

  keycloak:
    service: "platform-keycloak.orch-platform.svc"
    port: 80
    realm: "master"
    adminSecretName: "platform-keycloak"
    adminSecretNamespace: "orch-platform"
    clientId: "co-manager-m2m-client"
