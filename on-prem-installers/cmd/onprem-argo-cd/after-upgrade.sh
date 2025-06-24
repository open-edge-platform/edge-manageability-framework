#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit

cat << "EOF"

     _                     ____ ____    _   _                           _
    / \   _ __ __ _  ___  / ___|  _ \  | | | |_ __   __ _ _ __ __ _  __| | ___
   / _ \ | '__/ _` |/ _ \| |   | | | | | | | | '_ \ / _` | '__/ _` |/ _` |/ _ \
  / ___ \| | | (_| | (_) | |___| |_| | | |_| | |_) | (_| | | | (_| | (_| |  __/
 /_/   \_\_|  \__, |\___/ \____|____/   \___/| .__/ \__, |_|  \__,_|\__,_|\___|
              |___/                          |_|    |___/

EOF

export KUBECONFIG=/home/$USER/.kube/config

# Add /usr/local/bin to the PATH as some utilities, like kubectl, could be installed there
export PATH=$PATH:/usr/local/bin

cp /tmp/argo-cd/values.tmpl /tmp/argo-cd/argo-cd/templates/values.tmpl
# helm treats comma as separator in '--set' command, so multiple no_proxy values are treated as different env values, so we have to write them to file first
cat <<EOF >/tmp/argo-cd/proxy-values.yaml
http_proxy: ${http_proxy}
https_proxy: ${https_proxy}
no_proxy: ${no_proxy}
EOF

helm template -s templates/values.tmpl /tmp/argo-cd/argo-cd/ --values /tmp/argo-cd/proxy-values.yaml >/tmp/argo-cd/values.yaml
rm /tmp/argo-cd/argo-cd/templates/values.tmpl

echo "
notifications:
  extraVolumeMounts:
  - mountPath: /etc/ssl/certs/ca-certificates.crt
    name: tls-from-node
  - mountPath: /etc/ssl/certs/gitea_cert.crt
    name: gitea-tls
  extraVolumes:
  - name: tls-from-node
    hostPath:
      path: /etc/ssl/certs/ca-certificates.crt
  - name: gitea-tls
    hostPath:
      path: /usr/local/share/ca-certificates/gitea_cert.crt
server:
  volumeMounts:
  - mountPath: /etc/ssl/certs/ca-certificates.crt
    name: tls-from-node
  - mountPath: /etc/ssl/certs/gitea_cert.crt
    name: gitea-tls
  volumes:
  - name: tls-from-node
    hostPath:
      path: /etc/ssl/certs/ca-certificates.crt
  - name: gitea-tls
    hostPath:
      path: /usr/local/share/ca-certificates/gitea_cert.crt
repoServer:
  volumeMounts:
  - mountPath: /etc/ssl/certs/ca-certificates.crt
    name: tls-from-node
  - mountPath: /etc/ssl/certs/gitea_cert.crt
    name: gitea-tls
  volumes:
  - name: tls-from-node
    hostPath:
      path: /etc/ssl/certs/ca-certificates.crt
  - name: gitea-tls
    hostPath:
      path: /usr/local/share/ca-certificates/gitea_cert.crt
applicationSet:
  extraVolumeMounts:
  - mountPath: /etc/ssl/certs/ca-certificates.crt
    name: tls-from-node
  - mountPath: /etc/ssl/certs/gitea_cert.crt
    name: gitea-tls
  extraVolumes:
  - name: tls-from-node
    hostPath:
      path: /etc/ssl/certs/ca-certificates.crt
  - name: gitea-tls
    hostPath:
      path: /usr/local/share/ca-certificates/gitea_cert.crt
" >/tmp/argo-cd/mounts.yaml

echo "ArgoCD Helm Chart is being upgraded, please wait, timeout is set to 10m..."
helm upgrade argocd /tmp/argo-cd/argo-cd --values /tmp/argo-cd/values.yaml -f /tmp/argo-cd/mounts.yaml -n argocd \
  --wait --timeout 15m0s
