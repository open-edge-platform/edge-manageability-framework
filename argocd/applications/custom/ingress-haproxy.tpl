# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

{{- if eq .Values.argo.traefikSvcType "NodePort" }}
service:
  type: NodePort
  nodePorts:
    http: 30080
    https: 30443
{{- end}}
{{- if .Values.argo.resources.ingressHaproxy.root }}
resources:
  {{- toYaml .Values.argo.resources.ingressHaproxy.root | nindent 0 }}
{{- end}}
