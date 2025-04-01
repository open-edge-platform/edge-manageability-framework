# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

service:
  {{- if .Values.argo.traefikSvcType }}
  type: {{ .Values.argo.traefikSvcType }}
  {{- else}}
  type: NodePort
  {{- end}}
ports:
  web:
    expose: false
  websecure:
    nodePort: 31443
