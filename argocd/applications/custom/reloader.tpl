# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

{{- with .Values.argo.resources.reloader }}
reloader:
  deployment:
    resources:
      {{- toYaml . | nindent 6 }}
{{- end }}
