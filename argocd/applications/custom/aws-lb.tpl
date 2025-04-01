# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

clusterName: {{.Values.argo.clusterName}}
env:
  http_proxy: {{.Values.argo.proxy.httpProxy}}
  https_proxy: {{.Values.argo.proxy.httpsProxy}}
  {{- if gt (len .Values.argo.proxy.noProxy) 4000}}
  # yamllint disable-line rule:line-length
  {{- end}}
  no_proxy: {{.Values.argo.proxy.noProxy}}