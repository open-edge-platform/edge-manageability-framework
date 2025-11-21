# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

certDomain: {{ required "A valid clusterDomain entry required!" .Values.argo.clusterDomain }}

# DNS names for the TLS certificate (including wildcards for all subdomains)
# This allows Traefik to serve HTTPS for all services under the cluster domain
dnsNames:
  - "keycloak.{{ .Values.argo.clusterDomain }}"
  - "*.{{ .Values.argo.clusterDomain }}"

{{- if index .Values.argo "self-signed-cert"}}
generateOrchCert: {{index .Values.argo "self-signed-cert" "generateOrchCert"}}
{{- end}}
