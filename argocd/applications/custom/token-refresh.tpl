# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

{{- if .Values.argo.proxy.httpsProxy }}
proxy:
  deploy: true
env:
  - name: https_proxy
    value: {{ .Values.argo.proxy.httpsProxy }}
  - name: no_proxy
    value: {{ .Values.argo.proxy.noProxy }}
{{- end }}

{{- if .Values.argo.releaseService }}
{{- if .Values.argo.releaseService.caCert }}
proxyTargetCA: "{{ .Values.argo.releaseService.caCert  }}"
{{- end }}
{{- if .Values.argo.releaseService.tokenRefresh }}
authEndpoint: {{ .Values.argo.releaseService.tokenRefresh.endpoint }}
authPath: {{ .Values.argo.releaseService.tokenRefresh.path }}
useRefreshToken: {{ .Values.argo.releaseService.tokenRefresh.useRefreshToken }}
{{- end }}
{{- end }}
{{- with .Values.argo.resources.tokenRefresh }}
resources:
  {{- toYaml . | nindent 2 }}
{{- end }}
