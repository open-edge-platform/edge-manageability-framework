# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

server:
  service:
    type: ClusterIP
configs:
  params:
    application.namespaces: "*"
    server.tls.minversion: "1.2"
    # // Note that for TLS v1.3, cipher suites are not configurable and will be chosen automatically.
    server.tls.ciphers: "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384:TLS_AES_256_GCM_SHA384"

  # https://argo-cd.readthedocs.io/en/stable/operator-manual/health/#argocd-app
  cm:
    resource.customizations: |
      argoproj.io/Application:
        health.lua: |
          hs = {}
          hs.status = "Progressing"
          hs.message = ""
          if obj.status ~= nil then
            if obj.status.health ~= nil then
              hs.status = obj.status.health.status
              if obj.status.health.message ~= nil then
                hs.message = obj.status.health.message
              end
            end
          end
          return hs
global:
  env:
    # Proxy setting for Intel internal clusters
    - name: http_proxy
      value: "{{ .Values.proxy.httpProxy }}"
    - name: https_proxy
      value: "{{ .Values.proxy.httpsProxy }}"
    - name: no_proxy
      value: "{{ .Values.proxy.noProxy }}"

# FIXME Workaround for ArgoCD not applying CA file when pulling from OCI registry. Remove this once the issue is fixed
# Ref: https://github.com/argoproj/argo-cd/issues/13726, https://github.com/argoproj/argo-cd/issues/14877
repoServer:
  # -- Additional volumeMounts to the Repo Server main container
  volumeMounts:
    - mountPath: /etc/ssl/certs/registry-certs.pem
      name: registry-certs
      subPath: registry-certs.crt
  # -- Additional volumes to the Repo Server pod
  volumes:
    - name: registry-certs
      configMap:
        name: registry-certs