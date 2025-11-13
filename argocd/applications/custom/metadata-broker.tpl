# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

global:
  registry:
    name: {{ .Values.argo.containerRegistryURL }}
    imagePullSecrets:
    {{- with .Values.argo.imagePullSecrets }}
      {{- toYaml . | nindent 6 }}
    {{- end }}
{{- if and (index .Values.argo "metadata-broker") (index .Values.argo "metadata-broker" "storageClass")}}
persistence:
  enabled: true
  storageClassName: {{index .Values.argo "metadata-broker" "storageClass"}}
{{- end}}
service:
  traefik:
    hostname: "Host(`metadata.{{ .Values.argo.clusterDomain }}`)"
{{- with .Values.argo.resources.metadataBroker.root }}
resources:
  {{- toYaml . | nindent 2}}
{{- end }}
{{- with .Values.argo.resources.metadataBroker.opaResources }}
opaResources:
  {{- toYaml . | nindent 2}}
{{- end }}

# Keycloak issuer based on clusterDomain
{{- if or (contains "kind.internal" .Values.argo.clusterDomain) (contains "localhost" .Values.argo.clusterDomain) (eq .Values.argo.clusterDomain "") }}
openidc:
  issuer: "http://platform-keycloak.keycloak-system.svc.cluster.local/realms/master"
{{- else }}
openidc:
  issuer: "https://keycloak.{{ .Values.argo.clusterDomain }}/realms/master"
{{- end }}
