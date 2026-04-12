# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

image:
  registry: {{.Values.argo.containerRegistryURL }}
  repository: common/tenancy-manager
  tag: "nexus-replacement-20260411"
imagePullSecrets:
  {{- with .Values.argo.imagePullSecrets }}
    {{- toYaml . | nindent 2 }}
  {{- end }}

{{- with .Values.argo.resources.tenancyManager }}
resources:
  {{- toYaml . | nindent 4 }}
{{- end }}
postgres:
  secrets: iam-tenancy-local-postgresql
