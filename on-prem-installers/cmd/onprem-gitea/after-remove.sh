#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit

cat << "EOF"

   ____ _ _               ____
  / ___(_) |_ ___  __ _  |  _ \ ___ _ __ ___   _____   _____
 | |  _| | __/ _ \/ _` | | |_) / _ \ '_ ` _ \ / _ \ \ / / _ \
 | |_| | | ||  __/ (_| | |  _ <  __/ | | | | | (_) \ V /  __/
  \____|_|\__\___|\__,_| |_| \_\___|_| |_| |_|\___/ \_/ \___|


EOF

export KUBECONFIG=/home/$USER/.kube/config

# Add /usr/local/bin to the PATH as some utilities, like kubectl, could be installed there
export PATH=$PATH:/usr/local/bin

helm delete gitea -n gitea || true
kubectl delete secret gitea-cred gitea-tls-certs gitea-token -n gitea || true
# clean the certificate on the system
rm -f /usr/local/share/ca-certificates/gitea_cert.crt || true
