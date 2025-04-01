# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

{{- if index .Values.argo "certManager"}}
dns01RecursiveNameserversOnly: {{ .Values.argo.certManager.dns01RecursiveNameserversOnly }}
dns01RecursiveNameservers: "{{ .Values.argo.certManager.dns01RecursiveNameservers }}"
{{- end }}

{{- if .Values.argo.proxy }}
{{- if .Values.argo.proxy.httpProxy }}
http_proxy: "{{ .Values.argo.proxy.httpProxy }}"
{{- end }}
{{- if .Values.argo.proxy.httpsProxy }}
https_proxy: "{{ .Values.argo.proxy.httpsProxy }}"
{{- end }}
{{- if .Values.argo.proxy.noProxy }}
no_proxy: "{{ .Values.argo.proxy.noProxy }}"
{{- end }}
{{- end }}

{{- with .Values.argo.resources.certManager.root }}
resources:
  {{- toYaml . | nindent 2 }}
{{- end }}

{{- with .Values.argo.resources.certManager.cainjector }}
cainjector:
  resources:
    {{- toYaml . | nindent 4 }}
{{- end }}

{{- with .Values.argo.resources.certManager.webhook }}
webhook:
  resources:
    {{- toYaml . | nindent 4 }}
{{- end }}
