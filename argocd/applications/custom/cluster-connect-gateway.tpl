# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

fullnameOverride: cluster-connect-gateway
gateway:
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
  privateCA:
    enabled: true

security:
  agent:
    authMode: "jwt"

openpolicyagent:
  enabled: true
