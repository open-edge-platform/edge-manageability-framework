# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

{{- if .Values.argo}}
{{- if .Values.argo.releaseService}}
{{- if .Values.argo.releaseService.ociRegistry}}
ociUrl: {{ .Values.argo.releaseService.ociRegistry}}
{{- end}}
{{- end}}
{{- end}}
