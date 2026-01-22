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
  catalogServer: app-orch-catalog-grpc-server.orch-app.svc.cluster.local:8080
  admServer: app-deployment-api-grpc-server.orch-app.svc.cluster.local:8080
  namespace: orch-app
  vaultServer: "http://vault.orch-platform.svc.cluster.local:8200"
  keycloakServiceBase: "http://platform-keycloak.orch-platform.svc.cluster.local:8080"
  keycloakServer: "https://keycloak.{{ .Values.argo.clusterDomain }}"
  keycloakNamespace: "orch-platform"
  keycloakSecret: "platform-keycloak"
  releaseServiceBase: "rs-proxy.orch-platform.svc.cluster.local:8081"
  # harborServerExternal: The URL to be used in the Catalog's harbor-helm and harbor-docker Registry objects
  harborServerExternal: "https://registry-oci.{{ .Values.argo.clusterDomain }}"
  # releaseServiceRootUrl: The URL to be used in the Catalog's release-helm and release-docker Registry objects
  {{- if .Values.argo.releaseService.ociRegistry}}
  releaseServiceRootUrl: oci://{{ .Values.argo.releaseService.ociRegistry }}
  {{- else}}
  releaseServiceRootUrl: "oci://registry-rs.edgeorchestration.intel.com"
  {{- end}}

  manifestTag: "v1.5.2"

  # http proxy settings
  {{- if .Values.argo.proxy.httpProxy}}
  httpProxy: "{{ .Values.argo.proxy.httpProxy }}"
  httpsProxy: "{{ .Values.argo.proxy.httpsProxy }}"
  noProxy: "{{ .Values.argo.proxy.noProxy }}"
  {{- else}}
  httpProxy: ""
  httpsProxy: ""
  noProxy: ""
  {{- end}}
