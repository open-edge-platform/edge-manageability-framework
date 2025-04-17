# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

env:
  manager:
  {{- if .Values.argo.proxy }}
  {{- if .Values.argo.proxy.httpProxy }}
  - name: http_proxy
    value: "{{ .Values.argo.proxy.httpProxy }}"
  {{- end }}
  {{- if .Values.argo.proxy.httpsProxy }}
  - name: https_proxy
    value: "{{ .Values.argo.proxy.httpsProxy }}"
  {{- end }}
  {{- if .Values.argo.proxy.noProxy }}
  - name: no_proxy
    value: "{{ .Values.argo.proxy.noProxy }}"
  {{- end }}
  {{- end }}

containerSecurityContext:
  allowPrivilegeEscalation: false
  capabilities:
    drop:
      - ALL
securityContext:
  seccompProfile:
    type: RuntimeDefault
  runAsNonRoot: true
