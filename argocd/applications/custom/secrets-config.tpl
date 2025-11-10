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
  {{- end }}  {{- end }}

# Keycloak URLs based on clusterDomain
{{- if or (contains "kind.internal" .Values.argo.clusterDomain) (contains "localhost" .Values.argo.clusterDomain) (eq .Values.argo.clusterDomain "") }}
auth:
  oidc:
    idPAddr: "http://platform-keycloak.orch-platform.svc.cluster.local:8080"
    idPDiscoveryURL: "http://platform-keycloak.orch-platform.svc.cluster.local:8080/realms/master"
{{- else }}
auth:
  oidc:
    idPAddr: "https://keycloak.{{ .Values.argo.clusterDomain }}"
    idPDiscoveryURL: "https://keycloak.{{ .Values.argo.clusterDomain }}/realms/master"
{{- end }}
