# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

{{- if and .Values.argo.o11y .Values.argo.o11y.sre }}
metricsExporter:
  customerLabelValue: {{ .Values.argo.o11y.sre.customerLabel | default (.Values.argo.clusterName | default "local") }}
  {{- if .Values.argo.o11y.sre.vaultInstances }}
  vaultInstances:
  {{- with .Values.argo.o11y.sre.vaultInstances }}
    {{- toYaml . | nindent 2 }}
  {{- end }}
  {{- else }}
  vaultInstances:
    {{- range until (.Values.argo.vault.replicas | int | default 3) }}
    - {{ print "-vaultURI=vault-" . }}
    {{- end }}
  {{- end }}
  {{- with .Values.argo.resources.sreExporter.metricsExporter }}
  resources:
    {{- toYaml . | nindent 4}}
  {{- end }}
{{- end }}

otelCollector:
  {{- if and .Values.argo.o11y .Values.argo.o11y.sre .Values.argo.o11y.sre.pushInterval }}
  pushInterval: {{ .Values.argo.o11y.sre.pushInterval }}
  {{- end }}
  {{- if and .Values.argo.o11y .Values.argo.o11y.sre .Values.argo.o11y.sre.tls .Values.argo.o11y.sre.tls.enabled }}
  {{- with .Values.argo.o11y.sre.tls }}
  tls:
    enabled: true
    {{- if ne .insecureSkipVerify nil }}
    insecureSkipVerify: {{ .insecureSkipVerify }}
    {{- end }}
    {{- if ne .useSystemCaCertsPool nil }}
    useSystemCaCertsPool: {{ .useSystemCaCertsPool }}
    {{- end }}
    {{- if ne .caSecretEnabled nil }}
    caSecret:
      enabled: {{ .caSecretEnabled }}
    {{- end }}
  {{- end }}
  {{- end }}
  {{- if and .Values.argo.o11y .Values.argo.o11y.sre .Values.argo.o11y.sre.externalSecretsEnabled }}
  externalSecret:
    enabled: true
    providerSecretName: {{ .Values.argo.o11y.sre.providerSecretName | default (print "sre-secret-" (.Values.argo.clusterName | default "default")) }}
  {{- end }}
  proxy:
    {{- if and .Values.argo.proxy .Values.argo.proxy.httpProxy .Values.argo.proxy.httpsProxy }}
    enabled: true
    no_proxy: {{ .Values.argo.proxy.noProxy }}
    http_proxy: {{ .Values.argo.proxy.httpProxy }}
    https_proxy: {{ .Values.argo.proxy.httpsProxy }}
    {{- else }}
    enabled: false
    {{- end }}
  {{- with .Values.argo.resources.sreExporter.otelCollector }}
  resources:
    {{- toYaml . | nindent 4}}
  {{- end }}
imageRegistry: {{ .Values.argo.containerRegistryURL }}
imagePullSecrets:
{{- with .Values.argo.imagePullSecrets }}
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- with .Values.argo.resources.sreExporter.configReloader }}
configReloader:
  resources:
    {{- toYaml . | nindent 4}}
{{- end }}
