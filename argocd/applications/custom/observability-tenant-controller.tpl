# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

image:
  tag: "nexus-replacement-20260427"
  registry: {{ .Values.argo.containerRegistryURL }}

imagePullSecrets:
{{- with .Values.argo.imagePullSecrets }}
{{- toYaml . | nindent 2 }}
{{- end }}

sre:
  enabled: {{ index .Values.argo.enabled "sre-exporter" | default false }}
