# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Common name and DNS SAN of the self-signed TLS certificate
commonName: orch-10-139-220-230.pid.infra-host.com
nginxIngressRateLimit:
{{- if .Values.argo.nginxIngressRate }}
  rps: {{ .Values.argo.nginxIngressRate.rps | default 500 }}
  connections: {{ .Values.argo.nginxIngressRate.connections | default 70}}
{{- else }}
  rps: 500
  connections: 70
{{- end }}
