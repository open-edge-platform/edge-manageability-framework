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

image:
  registry:
    name: {{ .Values.argo.containerRegistryURL }}
    imagePullSecrets:
    {{- with .Values.argo.imagePullSecrets }}
      {{- toYaml . | nindent 6 }}
    {{- end }}

adm:
  image:
    tag: "nexus-replacement-20260427"
  extraEnv:
    - name: TENANT_MANAGER_URL
      value: "http://tenancy-manager.orch-iam.svc:8080"
  gitProxy: {{ .Values.argo.git.gitProxy }}
{{- if .Values.argo.git.gitServer }}
  gitServer: {{ .Values.argo.git.gitServer }}
{{- else }}
  gitServer: https://gitea.{{ .Values.argo.clusterDomain }}
{{- end }}
{{- if .Values.argo.git.fleetGitClientSecret }}
  fleetGitClientSecretName: {{ .Values.argo.git.fleetGitClientSecret }}
{{- end }}
  apiExtension:
    apiProxy:
      url: wss://ws-app-service-proxy.{{ .Values.argo.clusterDomain }}
    apiAgent:
      helmSecretName: {{ .Values.argo.adm.helmSecretName }}
  fullnameOverride: app-deployment-manager
{{- with .Values.argo.resources.appDeploymentManager.adm }}
  resources:
    {{- toYaml . | nindent 4 }}
{{- end }}

gateway:
  image:
    tag: "nexus-replacement-20260427"
  serviceArgs:
    nexus-api-url: "http://tenancy-manager.orch-iam.svc:8080"
  deployment:
    namespace: {{ .Values.argo.adm.deploymentNamespace }}
{{- with .Values.argo.resources.appDeploymentManager.gateway }}
  resources:
    {{- toYaml . | nindent 4 }}
{{- end }}

traefikReverseProxy:
  matchRoute: Host(`app-orch.{{ .Values.argo.clusterDomain }}`)
{{- if .Values.argo.traefik }}
  tlsOption: {{ .Values.argo.traefik.tlsOption | default "" | quote }}
{{- end }}
ingressRoute:
  apiHostname: api.{{ .Values.argo.clusterDomain }}

{{- with .Values.argo.resources.appDeploymentManager.openpolicyagent }}
openpolicyagent:
  resources:
    {{- toYaml . | nindent 4 }}
{{- end }}
