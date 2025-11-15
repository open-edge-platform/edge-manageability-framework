# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

traefikReverseProxy:
  enabled: true
  host:
    grpc:
      name: "cluster-orch-node.{{ .Values.argo.clusterDomain }}"
      secretName: tls-orch
{{- if .Values.argo.traefik }}
      tlsOption: {{ .Values.argo.traefik.tlsOption | default "" | quote }}
{{- end }}

manager:
  image:
    registry:
      name: {{ .Values.argo.containerRegistryURL }}
    repository: cluster/capi-provider-intel-manager
{{- with .Values.argo.resources.intelInfraProvider.manager }}
  resources:
    {{- toYaml . | nindent 4 }}
{{- end }}

southboundApi:
  image:
    registry:
      name: {{ .Values.argo.containerRegistryURL }}
    repository: cluster/capi-provider-intel-southbound
{{- with .Values.argo.resources.intelInfraProvider.southboundApi }}
  resources:
    {{- toYaml . | nindent 4 }}
{{- end }}

# Keycloak OIDC server URL - use internal service URL to avoid unnecessary Traefik load
oidc:
  oidc_server_url: "http://platform-keycloak.keycloak-system.svc.cluster.local/realms/master"
