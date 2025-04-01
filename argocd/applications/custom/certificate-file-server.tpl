# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

rootMatchHost: Host(`{{ .Values.argo.clusterDomain }}`)
orchSecretName: {{ .Values.argo.tlsSecret }}

{{- if .Values.argo.traefik }}
tlsOption: {{ .Values.argo.traefik.tlsOption | default "" | quote }}
{{- end }}

{{- with .Values.argo.resources.certificateFileServer }}
resources:
  {{- toYaml . | nindent 2 }}
{{- end }}
