#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -xe

DOCKER_USERNAME=""
DOCKER_PASSWORD=""

while getopts 'hu:p:' flag; do
  case "${flag}" in
    u) DOCKER_USERNAME="${OPTARG}" ;;
    p) DOCKER_PASSWORD="${OPTARG}" ;;
    h) HELP='true' ;;
    *) HELP='true' ;;
  esac
done

function usage {
    cat >&2 <<EOF
Purpose:
Customize rke2 server on localhost

Usage:
$(basename "$0") [-h] [-u username] [-p password]

ex:
customize-rke2.sh -u myusername -p mypassword

Options:
    -u:             Docker username (optional)
    -p:             Docker password (optional)
    -h:             help
EOF
}

if [[ $HELP ]]; then
    usage
    exit 1
fi

# We create some customizations for the RKE2 cluster after it is installed

# Copy kubeconfig to home folder and chown to USER from root
mkdir -p /home/"$USER"/.kube
sudo cp  /etc/rancher/rke2/rke2.yaml /home/"$USER"/.kube/config
sudo chown -R "$USER":"$USER"  /home/"$USER"/.kube
sudo chmod 600 /home/"$USER"/.kube/config
export KUBECONFIG=/home/"$USER"/.kube/config

# Configure Docker credentials if provided
if [[ -n "${DOCKER_USERNAME}" && -n "${DOCKER_PASSWORD}" ]]; then
    echo "Configuring Docker credentials"
    sudo -E bash -c 'cat << EOF > /etc/rancher/rke2/registries.yaml
configs:
  "registry-1.docker.io":
    auth:
      username: "${DOCKER_USERNAME}"
      password: "${DOCKER_PASSWORD}"
EOF'
fi

# Copy binaries for post RKE2 install operations
sudo cp /var/lib/rancher/rke2/bin/ctr /usr/local/bin/
sudo cp /var/lib/rancher/rke2/bin/kubectl /usr/local/bin/

# Restart rancher server to apply the changes
sudo systemctl restart rke2-server.service
