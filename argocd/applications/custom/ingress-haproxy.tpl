# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

controller:
{{- if eq .Values.argo.traefikSvcType "NodePort" }}
  service:
    type: NodePort
    nodePorts:
      https: 31443
{{- end}}
{{- if .Values.argo.resources.ingressHaproxy.controller.root }}
  resources:
    {{- toYaml .Values.argo.resources.ingressHaproxy.controller.root | nindent 4 }}
{{- else }}
  resources: null
{{- end }}
  admissionWebhooks:
    createSecretJob:
{{- if .Values.argo.resources.ingressHaproxy.controller.admissionWebhooks.createSecretJob }}
      resources:
        {{- toYaml .Values.argo.resources.ingressHaproxy.controller.admissionWebhooks.createSecretJob | nindent 8 }}
{{- else }}
      resources: null
{{- end }}
