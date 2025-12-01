# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

param:
  orch_fqdn: {{ .Values.argo.clusterDomain }}
  orch_ip: {{ .Values.argo.enic.orchestratorIp }}
  orchUser: {{ .Values.argo.enic.orchestratorUser }}
  orchPass: {{ .Values.argo.enic.orchestratorPass }}
  orchOrg: {{ .Values.argo.enic.orchestratorOrg }}
  orchProject: {{ .Values.argo.enic.orchestratorProject }}
replicaCount: {{ .Values.argo.enic.replicas }}

global:
  registry:
    name: "{{ .Values.argo.containerRegistryURL }}/"
    {{- $imagePullSecretsLength := len .Values.argo.imagePullSecrets }}
    {{- if eq $imagePullSecretsLength 0 }}
    imagePullSecrets: []
    {{- else }}
    imagePullSecrets:
    {{- with .Values.argo.imagePullSecrets }}
      {{- toYaml . | nindent 6 }}
    {{- end }}
    {{- end }}

proxy:
{{- if .Values.argo.proxy }}
  {{- if .Values.argo.proxy.httpProxy }}
  enabled: true
  {{- else }}
  enabled: false
  {{- end }}
  {{- if .Values.argo.proxy.httpProxy }}
  http_proxy: "{{ .Values.argo.proxy.httpProxy }}"
  {{- end }}
  {{- if .Values.argo.proxy.httpsProxy }}
  https_proxy: "{{ .Values.argo.proxy.httpsProxy }}"
  {{- end }}
  {{- if .Values.argo.proxy.noProxy }}
  no_proxy: "{{ .Values.argo.proxy.noProxy }}"
  {{- end }}
{{- end }}
