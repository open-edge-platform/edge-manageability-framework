# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

databases:
  {{ .Values.argo.database.databases | toYaml | nindent 2 }}
