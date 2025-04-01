# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

{{- if gt (len .Values.argo.proxy.noProxy) 4000}}
# yamllint disable-line rule:line-length
{{- end}}
extraEnv:
    - name: http_proxy
      value: {{.Values.argo.proxy.httpProxy}}
    - name: https_proxy
      value: {{.Values.argo.proxy.httpsProxy}}
    - name: no_proxy
      value: {{.Values.argo.proxy.noProxy}}
{{- with .Values.argo.resources.externalSecrets.root }}
resources:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- with .Values.argo.resources.externalSecrets.certController }}
certController:
  resources:
    {{- toYaml . | nindent 4 }}
{{- end }}
{{- with .Values.argo.resources.externalSecrets.webhook }}
webhook:
  resources:
    {{- toYaml . | nindent 4 }}
{{- end }}
