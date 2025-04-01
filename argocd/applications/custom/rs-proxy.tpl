# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

{{- if .Values.argo }}
{{- if .Values.argo.releaseService }}

{{- if .Values.argo.releaseService.tokenRefresh }}
withReleaseServiceToken: true
{{- else }}
withReleaseServiceToken: false
{{- end }}

{{- if .Values.argo.releaseService.ociRegistry }}
proxyTargetRegistry: {{ .Values.argo.releaseService.ociRegistry }}
{{- end }}

{{- if .Values.argo.releaseService.fileServer }}
proxyTargetFiles: {{ .Values.argo.releaseService.fileServer }}
{{- end }}

{{- if .Values.argo.releaseService.caCert }}
proxyTargetCA: "{{ .Values.argo.releaseService.caCert  }}"
{{- end }}

{{- end }}
{{- end }}

{{- if .Values.argo }}
{{- if .Values.argo.proxy }}
env:
{{- if .Values.argo.proxy.httpsProxy }}
  - name: https_proxy
    value: {{.Values.argo.proxy.httpsProxy }}
{{- end }}
{{- if .Values.argo.proxy.httpProxy }}
  - name: http_proxy
    value: {{.Values.argo.proxy.httpProxy }}
{{- end }}
{{- if .Values.argo.proxy.noProxy }}
  - name: no_proxy
    value: {{.Values.argo.proxy.noProxy }}
{{- end }}
{{- end }}
{{- end }}
{{- with .Values.argo.resources.rsProxy }}
resources:
  {{- toYaml . | nindent 2 }}
{{- end }}
