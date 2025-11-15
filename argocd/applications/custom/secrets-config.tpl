# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Vault should be manually initialized in production and production-like environments.
{{- if .Values.argo.vault.autoInit }}
autoInit: {{.Values.argo.vault.autoInit }}
{{- end }}
{{- if .Values.argo.vault.autoUnseal }}
autoUnseal: {{.Values.argo.vault.autoUnseal }}
{{- end }}
image:
  registry: {{.Values.argo.containerRegistryURL }}
  repository: common/secrets-config
imagePullSecrets:
  {{- with .Values.argo.imagePullSecrets }}
    {{- toYaml . | nindent 2 }}
  {{- end }}

# Keycloak URLs - use internal service URL to avoid unnecessary Traefik load
# Backend services access Keycloak directly via cluster DNS, bypassing Traefik
auth:
  oidc:
    idPAddr: "http://platform-keycloak.keycloak-system.svc.cluster.local"
    idPDiscoveryURL: "http://platform-keycloak.keycloak-system.svc.cluster.local/realms/master"
