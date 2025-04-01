# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

proxy:
  httpProxy: {{.Values.argo.proxy.httpProxy}}
  httpsProxy: {{.Values.argo.proxy.httpsProxy}}
  noProxy: {{.Values.argo.proxy.noProxy}}
image:
  registry: {{.Values.argo.containerRegistryURL }}
  repository: common/cert-synchronizer
imagePullSecrets:
  {{- with .Values.argo.imagePullSecrets }}
    {{- toYaml . | nindent 2 }}
  {{- end }}
{{- if .Values.argo.aws }}
aws:
  region: {{ .Values.argo.aws.region }}
{{- end }}
certDomain: {{ required "A valid clusterName entry required!" .Values.argo.clusterName }}
awsR35Domain: {{ .Values.argo.clusterDomain}}
{{- with .Values.argo.resources.certSynchronizer }}
resources:
  {{- toYaml . | nindent 2 }}
{{- end }}
