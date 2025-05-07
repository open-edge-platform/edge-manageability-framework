# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

{{- with .Values.argo.resources.orchestratorPrometheusAgent.prometheusOperator }}
prometheusOperator:
  resources:
    {{- toYaml . | nindent 4 }}
  prometheusConfigReloader:
    resources:
      {{- toYaml . | nindent 6 }}
{{- end }}
{{- with .Values.argo.resources.orchestratorPrometheusAgent.kubeStateMetrics }}
kube-state-metrics:
  resources:
    {{- toYaml . | nindent 4 }}
{{- end }}
{{- with .Values.argo.resources.orchestratorPrometheusAgent.prometheus.prometheusSpec }}
prometheus:
  prometheusSpec:
    resources:
      {{- toYaml . | nindent 6 }}
{{- end }}

{{- if (index .Values.argo.enabled kube-apiserver-metrics) }}
kubeApiServer:
  enabled: true
{{- end }}