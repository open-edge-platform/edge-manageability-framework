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

# Keycloak OIDC configuration
# Use internal service URL (like Bitnami mainline) - no CoreDNS rewrites needed
# Internal cluster communication uses cluster-internal DNS
auth:
  oidc:
    idPAddr: "http://platform-keycloak.keycloak-system.svc.cluster.local"
    idPDiscoveryURL: "http://platform-keycloak.keycloak-system.svc.cluster.local/realms/master"
