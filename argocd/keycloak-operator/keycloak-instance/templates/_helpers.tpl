{{/*
SPDX-FileCopyrightText: 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0

Expand the name of the chart.
*/}}
{{- define "keycloak-instance.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "keycloak-instance.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "keycloak-instance.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.AppVersion | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "keycloak-instance.labels" -}}
helm.sh/chart: {{ include "keycloak-instance.chart" . }}
{{ include "keycloak-instance.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "keycloak-instance.selectorLabels" -}}
app.kubernetes.io/name: {{ include "keycloak-instance.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Get the cluster domain from values or default
*/}}
{{- define "keycloak-instance.clusterDomain" -}}
{{- if and .Values.argo .Values.argo.clusterDomain }}
{{- .Values.argo.clusterDomain }}
{{- else }}
{{- "kind.internal" }}
{{- end }}
{{- end }}
