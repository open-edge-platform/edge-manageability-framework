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
# Use external domain with HTTPS so that:
# 1. CoreDNS rewrites keycloak.orch-X-X.pid.infra-host.com → traefik service
# 2. Traefik routes HTTPS requests to Keycloak pod
# 3. OIDC discovery validation succeeds (issuer URL is reachable)
# 4. Browsers can also access via same external domain
auth:
  oidc:
    idPAddr: "https://keycloak.{{ .Values.argo.clusterDomain }}"
    idPDiscoveryURL: "https://keycloak.{{ .Values.argo.clusterDomain }}/realms/master"
