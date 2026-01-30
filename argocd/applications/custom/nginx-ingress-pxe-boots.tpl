# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Common name and DNS SAN of the self-signed TLS certificate
commonName: tinkerbell-nginx.{{ .Values.argo.clusterDomain }}
# Enable HAProxy ingress instead of nginx
# Disabled at wave 1100 - cert/issuer only. Ingress created separately at wave 1300+
haproxyIngress:
  enabled: false
