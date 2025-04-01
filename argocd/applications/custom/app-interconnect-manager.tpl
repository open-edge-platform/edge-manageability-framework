# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

clusterDomain: {{ .Values.argo.clusterDomain }}

global:
  registry:
    name: {{ .Values.argo.containerRegistryURL }}
image:
  registry:
    name: {{ .Values.argo.containerRegistryURL }}
imagePullSecrets:
{{- with .Values.argo.imagePullSecrets }}
{{- toYaml . | nindent 2 }}
{{- end }}
{{- with .Values.argo.resources.interconnectManager }}
resources:
  {{- toYaml . | nindent 2 }}
{{- end }}
