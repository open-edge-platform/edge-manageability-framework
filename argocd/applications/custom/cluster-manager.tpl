# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

{{- $keycloakUrl := "" }}
{{- if or (contains "kind.internal" .Values.argo.clusterDomain) (contains "localhost" .Values.argo.clusterDomain) (eq .Values.argo.clusterDomain "") }}
{{- $keycloakUrl = "http://platform-keycloak.orch-platform.svc.cluster.local:8080/realms/master" }}
{{- else }}
{{- $keycloakUrl = printf "https://keycloak.%s/realms/master" .Values.argo.clusterDomain }}
{{- end }}

openidc:
  issuer: {{ $keycloakUrl }}

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
    terminationGracePeriodSeconds: 90
    backoffLimit: 15
    activeDeadlineSeconds: 1200  # timeout 20 minutes
    ttlSecondsAfterFinished: 14400  # auto-deletion 4 hours
    retryAttempts: 10
    retryDelay: 30

    resources:
      limits:
        cpu: "2"
        memory: 2Gi
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
