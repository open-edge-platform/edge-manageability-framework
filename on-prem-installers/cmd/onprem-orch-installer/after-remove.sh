#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit

cat << "EOF"

   ___           _         ___           _        _ _             ____
  / _ \ _ __ ___| |__     |_ _|_ __  ___| |_ __ _| | | ___ _ __  |  _ \ ___ _ __ ___   _____   _____
 | | | | '__/ __| '_ \     | || '_ \/ __| __/ _` | | |/ _ \ '__| | |_) / _ \ '_ ` _ \ / _ \ \ / / _ \
 | |_| | | | (__| | | |_   | || | | \__ \ || (_| | | |  __/ |    |  _ <  __/ | | | | | (_) \ V /  __/
  \___/|_|  \___|_| |_(_) |___|_| |_|___/\__\__,_|_|_|\___|_|    |_| \_\___|_| |_| |_|\___/ \_/ \___|


EOF

export KUBECONFIG=/home/$USER/.kube/config
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/games:/usr/local/games:/snap/bin

kubectl delete job -n gitea -l managed-by=edge-manageability-framework || true
kubectl delete sts -n orch-database postgresql || true
kubectl delete job -n orch-infra credentials || true
kubectl delete job -n orch-infra loca-credentials || true

if [ "${1}" = "upgrade" ]; then
    exit 0
fi

# Secrets for postgresql are generated on each installation, so we have to clean them up to avoid issues during reinstallation
kubectl delete secret -l managed-by=edge-manageability-framework -A || true
