# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Common name and DNS SAN of the self-signed TLS certificate
commonName: tinkerbell-nginx.{{ .Values.argo.clusterDomain }}
# Enable HAProxy ingress instead of nginx
haproxyIngress:
  enabled: true
