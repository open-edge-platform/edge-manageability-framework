# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

{{- if .Values.argo.aws }}
enableDatabaseNamespace: false
{{- end }}

# Enable EnvoyFilter for Grafana header size limits
enableGrafanaHeaderSizeLimits: true
