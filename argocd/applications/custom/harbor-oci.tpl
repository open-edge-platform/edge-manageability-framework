# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

externalURL: "https://registry-oci.{{ .Values.argo.clusterDomain }}"

persistence:
  persistentVolumeClaim:
    {{- if .Values.argo.harbor.storageClass }}
    database:
      storageClass: {{ .Values.argo.harbor.storageClass }}
    {{ end }}
    registry:
      size: {{ .Values.argo.harbor.registrySize }}
    jobservice:
      jobLog:
        size: {{ .Values.argo.harbor.jobLogSize }}

proxy:
  httpProxy: "{{ .Values.argo.proxy.httpProxy }}"
  httpsProxy: "{{ .Values.argo.proxy.httpsProxy }}"
  noProxy: "{{ .Values.argo.proxy.noProxy }}"

{{- with .Values.argo.resources.harborOci.core }}
core:
  resources:
    {{- toYaml . | nindent 4 }}
{{- end }}
database:
  internal:
{{- with .Values.argo.resources.harborOci.database.internal.root }}
    resources:
      {{- toYaml . | nindent 6 }}
{{- end }}
    initContainer:
      migrator:
{{- with .Values.argo.resources.harborOci.database.internal.initContainer.migrator }}
        resources:
          {{- toYaml . | nindent 10 }}
{{- end }}
{{- with .Values.argo.resources.harborOci.database.internal.initContainer.permissions }}
        resources:
          {{- toYaml . | nindent 10 }}
{{- end }}
{{- with .Values.argo.resources.harborOci.exporter }}
exporter:
  resources:
    {{- toYaml . | nindent 4 }}
{{- end }}
{{- with .Values.argo.resources.harborOci.jobservice }}
jobservice:
  resources:
    {{- toYaml . | nindent 4 }}
{{- end }}
{{- with .Values.argo.resources.harborOci.nginx }}
nginx:
  resources:
    {{- toYaml . | nindent 4 }}
{{- end }}
{{- with .Values.argo.resources.harborOci.portal }}
portal:
  resources:
    {{- toYaml . | nindent 4 }}
{{- end }}
{{- with .Values.argo.resources.harborOci.redis.internal }}
redis:
  internal:
    resources:
      {{- toYaml . | nindent 6 }}
{{- end }}
{{- if .Values.argo.resources.harborOci.registry }}
registry:
  registry:
{{- if .Values.argo.resources.harborOci.registry.registry }}
    resources:
      {{- toYaml .Values.argo.resources.harborOci.registry.registry | nindent 6 }}
{{- else }}
    resources: null
{{- end }}
  controller:
{{- if .Values.argo.resources.harborOci.registry.controller }}
    resources:
      {{- toYaml .Values.argo.resources.harborOci.registry.controller | nindent 6 }}
{{- else }}
    resources: null
{{- end }}
{{- end }}
{{- with .Values.argo.resources.harborOci.trivy }}
trivy:
  resources:
    {{- toYaml . | nindent 4 }}
{{- end }}
