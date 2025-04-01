# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

remoteNamespace: orch-gateway
refreshInterval: "10m"
targetSecretName: gateway-ca-cert
targetKeyName: ca.crt
sourceSecretName: tls-orch
keyName:
  - source: tls.crt
    target: ca.crt
