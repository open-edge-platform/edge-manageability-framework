# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

releaseMatchHost: Host(`release.{{ .Values.argo.clusterDomain }}`)
orchSecretName: tls-orch
image:
  registry: {{.Values.argo.containerRegistryURL }}
  repository: common/token-fs
imagePullSecrets:
  {{- with .Values.argo.imagePullSecrets }}
    {{- toYaml . | nindent 2 }}
  {{- end }}
{{- if .Values.argo.traefik }}
tlsOption: {{ .Values.argo.traefik.tlsOption | default "" | quote }}
{{- end }}
{{- with .Values.argo.resources.tokenFileServer }}
resources:
  {{- toYaml . | nindent 2 }}
{{- end }}

{{- if .Values.argo.tokenRefresh }}
emptyReleaseServiceToken: "false"
{{- else }}
emptyReleaseServiceToken: "true"
{{- end }}
