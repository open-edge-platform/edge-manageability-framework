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
