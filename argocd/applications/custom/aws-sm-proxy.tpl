# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

image:
  registry: {{.Values.argo.containerRegistryURL }}
  repository: common/aws-sm-proxy
{{- if .Values.argo}}
{{- if .Values.argo.aws}}
aws:
  region: {{ .Values.argo.aws.smProxyRegion | default .Values.argo.aws.region }}
{{- end}}

{{- if .Values.argo.proxy}}

{{- if .Values.argo.proxy.httpsProxy}}
httpsProxy: {{.Values.argo.proxy.httpsProxy}}
{{- end}}

{{- if .Values.argo.proxy.noProxy}}
noProxy: {{.Values.argo.proxy.noProxy}}
{{- end}}

{{- end}}
{{- end}}

{{- with .Values.argo.resources.awsSmProxy }}
resources:
  {{- toYaml . | nindent 2 }}
{{- end }}
