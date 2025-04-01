# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

kube-prometheus-stack:
  grafana:
    grafana.ini:
      server:
        root_url: https://observability-ui.{{.Values.argo.clusterDomain}}
      auth:
        signout_redirect_url: https://keycloak.{{.Values.argo.clusterDomain}}/realms/master/protocol/openid-connect/logout?redirect_uri=https%3A%2F%2Fweb-ui.{{.Values.argo.clusterDomain}}
      auth.generic_oauth:
        auth_url: https://keycloak.{{.Values.argo.clusterDomain}}/realms/master/protocol/openid-connect/auth
grafana-admin:
  grafana.ini:
    server:
      root_url: https://observability-admin.{{.Values.argo.clusterDomain}}
    auth:
      signout_redirect_url: https://keycloak.{{.Values.argo.clusterDomain}}/realms/master/protocol/openid-connect/logout?redirect_uri=https%3A%2F%2Fweb-ui.{{.Values.argo.clusterDomain}}
    auth.generic_oauth:
      auth_url: https://keycloak.{{.Values.argo.clusterDomain}}/realms/master/protocol/openid-connect/auth
