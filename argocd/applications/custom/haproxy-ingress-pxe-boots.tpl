# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Common name and DNS SAN of the self-signed TLS certificate
commonName: tinkerbell-haproxy.{{ .Values.argo.clusterDomain }}

# Ingress configuration
haproxyIngress:
  # Set to false to disable Ingress resource (use LoadBalancer instead)
  enabled: true

# Rate limiting for haproxy ingress (when ingress.enabled=true)
haproxyIngressRateLimit:
{{- if .Values.argo.haproxyIngressRate }}
  rps: {{ .Values.argo.haproxyIngressRate.rps | default 500 }}
  connections: {{ .Values.argo.haproxyIngressRate.connections | default 70}}
{{- else }}
  rps: 500
  connections: 70
{{- end }}
