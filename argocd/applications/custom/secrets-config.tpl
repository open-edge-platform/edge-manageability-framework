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
# Disable Istio sidecar injection for the secrets-config job pods so
# they can talk to Keycloak instances that intentionally have injection
# disabled (prevents mTLS/sidecar mismatch issues).
podAnnotations:
  sidecar.istio.io/inject: "false"
podLabels:
  sidecar.istio.io/inject: "false"