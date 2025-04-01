# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

remoteNamespace: orch-gateway
refreshInterval: "10m"
targetSecretName: tls-ca
sourceSecretName: tls-orch
keyName:
  {{- if index .Values.argo "self-signed-cert" }}
  {{- if eq (index .Values.argo "self-signed-cert" "generateOrchCert") true }}
  - source: ca.crt
  {{- else }}
  - source: tls.crt
  {{- end }}
  {{- else }}
  - source: tls.crt
  {{- end }}
    target: cacerts.pem
