# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Keycloak URLs - always use external domain to match Keycloak's configured hostname
{{- $keycloakUrl := printf "https://keycloak.%s/realms/master" .Values.argo.clusterDomain }}
{{- $keycloakHost := printf "keycloak.%s" .Values.argo.clusterDomain }}

openidc:
  issuer: {{ $keycloakUrl }}

clusterManager:
  args:
    clusterdomain: {{ .Values.argo.clusterDomain }}
    # JWT TTL configuration for kubeconfig
    # If kubeconfig-ttl-hours=0 token expires at creation
    # keycloak realm settings and upbounded by the SSO sessions max: 12h
    kubeconfig-ttl-hours: 3
  # Add init container to wait for Keycloak to be ready
  # This prevents cluster-manager from crashing when Keycloak is not ready
  initContainers:
    - name: wait-for-keycloak
      image: curlimages/curl:8.5.0
      command:
        - sh
        - -c
        - |
          echo "Waiting for Keycloak at {{ $keycloakHost }} to be ready..."
          until curl --fail --connect-timeout 5 --max-time 10 -s {{ $keycloakUrl }} > /dev/null 2>&1; do
            echo "Keycloak not ready yet, retrying in 5 seconds..."
            sleep 5
          done
          echo "Keycloak is ready!"
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
        allowPrivilegeEscalation: false
        seccompProfile:
          type: RuntimeDefault
        capabilities:
          drop:
            - ALL
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
    service: "platform-keycloak.keycloak-system.svc.cluster.local" # internal k8s DNS always uses cluster.local
    port: 8080
    realm: "master"
    adminSecretName: "platform-keycloak"
    adminSecretNamespace: "orch-platform"
    clientId: "co-manager-m2m-client"
