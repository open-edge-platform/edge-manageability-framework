# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Proxy setting for Intel internal networks
# Use only one proxy-* profile

argo:
  proxy:
    httpProxy: {{ .Values.proxy.httpProxy }}
    httpsProxy: {{ .Values.proxy.httpsProxy }}
    noProxy: "{{ .Values.proxy.noProxy }}"
    # Proxy Config for the Edge Node
    enHttpProxy: {{ .Values.proxy.enHttpProxy }}
    enHttpsProxy: {{ .Values.proxy.enHttpsProxy }}
    enFtpProxy: {{ .Values.proxy.enFtpProxy }}
    enSocksProxy: {{ .Values.proxy.enSocksProxy }}
    enNoProxy: {{ .Values.proxy.enNoProxy }}
{{- if .Values.proxy.noPeerProxyDomains }}
    noPeerProxyDomains: "{{ .Values.proxy.noPeerProxyDomains }}"
{{- end }}