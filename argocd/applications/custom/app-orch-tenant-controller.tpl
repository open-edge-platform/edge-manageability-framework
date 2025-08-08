# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

clusterDomain: {{ .Values.argo.clusterDomain }}

global:
  registry:
    name: {{ .Values.argo.containerRegistryURL }}
image:
  registry:
    name: {{ .Values.argo.containerRegistryURL }}

{{- with .Values.argo.imagePullSecrets }}
imagePullSecrets:
  {{- toYaml . | nindent 2 }}
{{- end }}

configProvisioner:
  useM2MToken: true
  # harborServerExternal: The URL to be used in the Catalog's harbor-helm and harbor-docker Registry objects
  harborServerExternal: "https://registry-oci.{{ .Values.argo.clusterDomain }}"
  keycloakServer: "https://keycloak.{{ .Values.argo.clusterDomain }}"


  # releaseServiceRootUrl: The URL to be used in the Catalog's release-helm and release-docker Registry objects
  {{- if .Values.argo.releaseService.ociRegistry}}
  releaseServiceRootUrl: oci://{{ .Values.argo.releaseService.ociRegistry }}
  {{- end}}

  manifestTag: "v1.3.6"

  # http proxy settings
  {{- if .Values.argo.proxy.httpProxy}}
  httpProxy: "{{ .Values.argo.proxy.httpProxy }}"
  httpsProxy: "{{ .Values.argo.proxy.httpsProxy }}"
  noProxy: "{{ .Values.argo.proxy.noProxy }}"
  {{- end}}

  {{- with .Values.argo.resources.appOrchTenantController.configProvisioner }}
  resources:
    {{- toYaml . | nindent 4 }}
  {{- end }}
