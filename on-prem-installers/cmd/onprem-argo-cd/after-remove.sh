#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit

export KUBECONFIG=/home/$USER/.kube/config

# Add /usr/local/bin to the PATH as some utilities, like kubectl, could be installed there
export PATH=$PATH:/usr/local/bin

remove_argocd() {   

cat << "EOF"

     _                     ____ ____    ____
    / \   _ __ __ _  ___  / ___|  _ \  |  _ \ ___ _ __ ___   _____   _____
   / _ \ | '__/ _` |/ _ \| |   | | | | | |_) / _ \ '_ ` _ \ / _ \ \ / / _ \
  / ___ \| | | (_| | (_) | |___| |_| | |  _ <  __/ | | | | | (_) \ V /  __/
 /_/   \_\_|  \__, |\___/ \____|____/  |_| \_\___|_| |_| |_|\___/ \_/ \___|
              |___/

EOF

    # If ArgoCD is upgraded its helm chart shouldn't be deleted, as helm upgrade
    # will be called in after-upgrade
    if [ "${1}" = "upgrade" ]; then
        return 0
    fi

    helm delete argocd -n argocd || true

    # Remove artifacts
    rm -rf /tmp/argo-cd || true
}


remove_gitea() {
cat << "EOF"

   ____ _ _               ____
  / ___(_) |_ ___  __ _  |  _ \ ___ _ __ ___   _____   _____
 | |  _| | __/ _ \/ _` | | |_) / _ \ '_ ` _ \ / _ \ \ / / _ \
 | |_| | | ||  __/ (_| | |  _ <  __/ | | | | | (_) \ V /  __/
  \____|_|\__\___|\__,_| |_| \_\___|_| |_| |_|\___/ \_/ \___|


EOF

    helm delete gitea -n gitea || true
    kubectl delete secret gitea-cred gitea-tls-certs gitea-token -n gitea || true
    # clean the certificate on the system
    rm -f /usr/local/share/ca-certificates/gitea_cert.crt || true
}

remove_gitea
remove_argocd