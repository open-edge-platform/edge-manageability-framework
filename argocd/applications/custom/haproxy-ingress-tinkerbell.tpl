# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0
commonName: tinkerbell-nginx.{{ .Values.argo.clusterDomain }}
# HAProxy Ingress for Tinkerbell - created after tinkerbell service exists
haproxyIngress:
  enabled: true
