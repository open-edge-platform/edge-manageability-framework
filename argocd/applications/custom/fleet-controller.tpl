# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# proxy settings
{{- if .Values.argo.proxy.httpProxy}}
proxy: "{{ .Values.argo.proxy.httpProxy }}"
{{- end}}

{{- if .Values.argo.proxy.noProxy}}
noProxy: "{{ .Values.argo.proxy.noProxy }}"
{{- end}}

apiServerURL: "https://fleet.{{ .Values.argo.clusterDomain }}"
