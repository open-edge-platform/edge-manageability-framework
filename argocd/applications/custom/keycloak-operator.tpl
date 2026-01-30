# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Custom overrides for keycloak-operator deployment
# This template applies cluster-specific customizations on top of base configuration

# Override operator deployment resources if specified
{{- if and .Values.argo .Values.argo.resources .Values.argo.resources.keycloakOperator }}
operator:
  container:
    resources:
      {{- toYaml .Values.argo.resources.keycloakOperator | nindent 6 }}
{{- end }}

# Override operator replicas if specified
{{- if and .Values.argo .Values.argo.keycloakOperator .Values.argo.keycloakOperator.replicas }}
operator:
  replicas: {{ .Values.argo.keycloakOperator.replicas }}
{{- end }}

# Override operator image if specified
{{- if and .Values.argo .Values.argo.keycloakOperator .Values.argo.keycloakOperator.image }}
operator:
  image: {{ .Values.argo.keycloakOperator.image | quote }}
{{- end }}

# Override operator imagePullSecrets if specified
{{- if and .Values.argo .Values.argo.keycloakOperator .Values.argo.keycloakOperator.imagePullSecrets }}
imagePullSecrets:
  {{- toYaml .Values.argo.keycloakOperator.imagePullSecrets | nindent 2 }}
{{- end }}
