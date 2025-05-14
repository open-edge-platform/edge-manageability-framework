# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

clusterManager:
  args:
    clusterdomain: {{ .Values.argo.clusterDomain }}
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