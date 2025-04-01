# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

{{- with .Values.argo.resources.kiali.deployment }}
deployment:
  resources:
    {{- toYaml . | nindent 4 }}
{{- end }}
