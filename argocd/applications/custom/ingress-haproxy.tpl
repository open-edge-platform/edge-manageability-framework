# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

controller:
{{- if eq .Values.argo.traefikSvcType "NodePort" }}
  service:
    type: NodePort
    httpsPort: 443
    targetPorts:
      https: 8443
    nodePorts:
      https: 31443
{{- end}}
{{- if .Values.argo.resources.ingressHaproxy.controller.root }}
  resources:
    {{- toYaml .Values.argo.resources.ingressHaproxy.controller.root | nindent 4 }}
{{- else }}
  resources: null
{{- end }}
