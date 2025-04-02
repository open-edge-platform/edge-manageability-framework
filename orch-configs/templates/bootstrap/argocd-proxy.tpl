# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

global:
  env:
    # Proxy setting for Intel internal clusters
    - name: http_proxy
      value: "{{ .Values.argo.proxy.httpProxy }}"
    - name: https_proxy
      value: "{{ .Values.argo.proxy.httpsProxy }}"
    - name: no_proxy
      value: "{{ .Values.argo.proxy.noProxy }}"
