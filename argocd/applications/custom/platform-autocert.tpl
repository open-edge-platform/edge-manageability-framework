# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

certDomain: {{ required "A valid clusterDomain entry required!" .Values.argo.clusterDomain }}

{{- if index .Values.argo "self-signed-cert"}}
generateOrchCert: {{index .Values.argo "self-signed-cert" "generateOrchCert"}}
{{- end}}

{{ if ((.Values.argo.aws).account) }}
{{- if index .Values.argo "autoCert"}}
autoCert:
  enabled: {{index .Values.argo "autoCert" "enabled"}}
  certDomain: {{index .Values.argo "autoCert" "domain"}}
  production: {{ .Values.argo.autoCert.production }}
  issuer: {{index .Values.argo "autoCert" "issuer"}}
  aws:
    region: {{ .Values.argo.aws.region }}
    role: "arn:aws:iam::{{ .Values.argo.aws.account }}:role/certmgr-{{ .Values.argo.clusterName }}"
  cert:
    adminEmail: {{index .Values.argo "adminEmail"}}
{{- end}}
{{- end}}