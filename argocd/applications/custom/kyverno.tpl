# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

{{- with .Values.argo.resources.kyverno.admissionController }}
{{- if or .initContainer .container }}
admissionController:
{{- with .initContainer }}
  initContainer:
    resources:
      {{- toYaml . | nindent 6 }}
{{- end }}
{{- with .container }}
  container:
    resources:
      {{- toYaml . | nindent 6 }}
{{- end }}
{{- end }}
{{- end }}
{{- with .Values.argo.resources.kyverno.backgroundController }}
backgroundController:
  resources:
    {{- toYaml . | nindent 4 }}
{{- end }}
{{- with .Values.argo.resources.kyverno.cleanupController }}
cleanupController:
  resources:
    {{- toYaml . | nindent 4 }}
{{- end }}
{{- with .Values.argo.resources.kyverno.reportsController }}
reportsController:
  resources:
    {{- toYaml . | nindent 4 }}
{{- end }}
