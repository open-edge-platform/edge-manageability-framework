# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

{{- with .Values.argo.resources.istiod.global.proxy }}
global:
  proxy:
    # Resources for the sidecar.
    resources:
      {{- toYaml . | nindent 6}}
{{- end}}
{{- with .Values.argo.resources.istiod.pilot }}
pilot:
  resources:
    {{- toYaml . | nindent 4 }}
{{- end }}
