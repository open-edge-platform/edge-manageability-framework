# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

{{- if .Values.argo.istio.resources}}
global:
  proxy:
    # Resources for the sidecar.
    resources:
      {{.Values.argo.istio.resources | toYaml | nindent 6}}
{{- end}}