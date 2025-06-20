#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit

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
    exit 0
fi

export KUBECONFIG=/home/$USER/.kube/config

# Add /usr/local/bin to the PATH as some utilities, like kubectl, could be installed there
export PATH=$PATH:/usr/local/bin

helm delete argocd -n argocd || true

# Remove artifacts
rm -rf /tmp/argo-cd || true
