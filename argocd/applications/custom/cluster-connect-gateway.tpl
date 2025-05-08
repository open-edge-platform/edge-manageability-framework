# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

fullnameOverride: cluster-connect-gateway
gateway:
  {{- with .Values.argo.resources.clusterConnectGateway.gateway }}
  resources:
    {{- toYaml . | nindent 4 }}
  {{- end }}
  externalUrl: "wss://connect-gateway.{{ .Values.argo.clusterDomain }}:443"
  ingress:
    enabled: true
    hostname: "connect-gateway.{{ .Values.argo.clusterDomain }}"
    namespace: orch-gateway
  oidc:
    enabled: true
agent:
  tlsMode: system-store

controller:
  {{- with .Values.argo.resources.clusterConnectGateway.controller }}
  resources:
    {{- toYaml . | nindent 4 }}
  {{- end }}
  privateCA:
    enabled: true

security:
  agent:
    authMode: "jwt"

openpolicyagent:
  enabled: true
