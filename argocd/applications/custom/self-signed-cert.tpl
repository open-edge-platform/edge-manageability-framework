# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

certDomain: {{ required "A valid clusterDomain entry required!" .Values.argo.clusterDomain }}

{{- if index .Values.argo "self-signed-cert"}}
generateOrchCert: {{index .Values.argo "self-signed-cert" "generateOrchCert"}}
{{- end}}
