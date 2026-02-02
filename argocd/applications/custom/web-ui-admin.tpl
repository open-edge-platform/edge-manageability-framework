# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

global:
  registry:
    name: {{ .Values.argo.containerRegistryURL }}
    imagePullSecrets:
    {{- with .Values.argo.imagePullSecrets }}
      {{- toYaml . | nindent 6 }}
    {{- end }}

# MFE configuration - controls which micro-frontends are available
# Alerts is treated as a separate orchestrator-level feature like app-orch, cluster-orch
mfe:
  app_orch: {{ and (index .Values.argo.enabled "web-ui-app-orch" | default false) (index .Values.argo.enabled "app-orch-catalog" | default false) }}
  cluster_orch: {{ and (index .Values.argo.enabled "web-ui-cluster-orch" | default false) (or (index .Values.argo.enabled "cluster-manager") (index .Values.argo.enabled "capi-operator") (index .Values.argo.enabled "intel-infra-provider") | default false) }}
  infra: {{ and (index .Values.argo.enabled "web-ui-infra" | default false) (or (index .Values.argo.enabled "infra-manager") (index .Values.argo.enabled "infra-operator") (index .Values.argo.enabled "tinkerbell") (index .Values.argo.enabled "infra-onboarding") (index .Values.argo.enabled "maintenance-manager") | default false) }}
  admin: {{ index .Values.argo.enabled "web-ui-admin" | default false }}
  alerts: {{ or (index .Values.argo.enabled "orchestrator-observability") (index .Values.argo.enabled "alerting-monitor") | default false }}
{{- with .Values.argo.resources.webUiAdmin }}
resources:
  {{- toYaml . | nindent 2 }}
{{- end }}
