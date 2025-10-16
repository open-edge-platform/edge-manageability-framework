# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

clusterManager:
  args:
    clusterdomain: {{ .Values.argo.clusterDomain }}
    # JWT TTL configuration for kubeconfig
    # If kubeconfig-ttl-hours=0 token expires at creation
    # keycloak realm settings and upbounded by the SSO sessions max: 12h
    kubeconfig-ttl-hours: 3
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

# co-manager M2M client configuration
credentialsM2M:
  enabled: true

  job:
    # job execution settings
    backoffLimit: 6
    activeDeadlineSeconds: 600  # timeout 10 minutes
    ttlSecondsAfterFinished: 86400  # auto-deletion 24 hours
    retryAttempts: 5
    retryDelay: 30
    # enable proper Istio sidecar termination
    terminateIstioSidecar: true
    # istio annotations for graceful job completion
    annotations:
      sidecar.istio.io/inject: "true"
      proxy.istio.io/config: |
        holdApplicationUntilProxyStarts: true
        # terminate sidecar when main container exits
        terminationDrainDuration: 5s

    resources:
      limits:
        cpu: "64"
        memory: 64Gi
      requests:
        cpu: 10m
        memory: 16Mi

  vault:
    service: "vault.orch-platform.svc.cluster.local" # internal k8s DNS always uses cluster.local
    port: 8200
    secretPath: "secret/data/co-manager-m2m-client-secret"
    authPath: "auth/kubernetes"

  keycloak:
    service: "platform-keycloak.orch-platform.svc.cluster.local" # internal k8s DNS always uses cluster.local
    port: 8080
    realm: "master"
    adminSecretName: "platform-keycloak"
    adminSecretNamespace: "orch-platform"
    clientId: "co-manager-m2m-client"
