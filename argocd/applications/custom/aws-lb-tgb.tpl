# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

{{- if .Values.argo }}
{{- if .Values.argo.aws }}
{{- if .Values.argo.aws.targetGroup }}
targetGroup:
  traefik: {{.Values.argo.aws.targetGroup.traefik}}
  traefikGrpc: {{.Values.argo.aws.targetGroup.traefikGrpc}}
  nginx: {{.Values.argo.aws.targetGroup.nginx}}
  argocd: {{.Values.argo.aws.targetGroup.argocd}}
{{- end }}
{{- end }}
{{- end }}
