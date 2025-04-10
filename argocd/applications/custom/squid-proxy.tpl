# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

{{- if .Values.argo.proxy.httpsProxy }}
httpsProxy: {{.Values.argo.proxy.httpsProxy }}
{{- end }}
image:
  registry: {{.Values.argo.containerRegistryURL }}
  repository: common/squid-proxy
imagePullSecrets:
  {{- with .Values.argo.imagePullSecrets }}
    {{- toYaml . | nindent 2 }}
  {{- end }}
{{- if .Values.argo.proxy.noPeerProxyDomains }}
noPeerProxyDomains: {{.Values.argo.proxy.noPeerProxyDomains }}
{{- end }}
