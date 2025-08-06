#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Script Name: rke2installerlocal.sh
# Description: This script installs RKE2 on the machine where the script is executed.
#              This script takes arguments as described below.
# Usage: ./rke2/rke2installerlocal.sh [ -i <host ip>] [ -v <rke2_version> ] [ -a ]
#    -i:             Host IP (optional)
#    -v:             rke2 version (optional)
#    -h:             help (optional)

set +xe

HELP=""
RKE2VERSION="v1.30.10+rke2r1"
HOSTIP=""
ROOT_DIR=$(pwd)

while getopts 'i:v:h' flag; do
  case "${flag}" in
   # i) HOSTIP="${HOSTIP}" ;; is a noop shellcheck flags?
    v) RKE2VERSION="${OPTARG}" ;;
    h) HELP='true' ;;
    *) HELP='true' ;;
  esac
done

function usage {
    cat >&2 <<EOF
Purpose:
install rke2 server on localhost

Usage:
$(basename "$0") [ -i <host ip >  -v <rke2_version> -a ]

ex:
./rke2/rkeinstallerlocal.sh -i 192.168.0.100 -v v1.25.4+rke2r1 -a

Options:
    -i:             Host IP (optional)
    -v:             rke2 version (optional)
    -h:             help (optional)
EOF
}


if [[ $HELP ]]; then
    usage
    exit 1
fi

# Escape '+' character if found in the URL request
# shellcheck disable=SC2001
RKE2VERSION=$(echo "$RKE2VERSION" | sed 's/+/%2b/g')
curl -sfL --create-dirs "https://github.com/rancher/rke2/releases/download/${RKE2VERSION}/rke2-images.linux-amd64.tar.zst" --output "${ROOT_DIR}/assets/rke2/rke2-images.linux-amd64.tar.zst"
curl -sfL --create-dirs "https://github.com/rancher/rke2/releases/download/${RKE2VERSION}/rke2-images-calico.linux-amd64.tar.zst" --output "${ROOT_DIR}/assets/rke2/rke2-images-calico.linux-amd64.tar.zst"
curl -sfL --create-dirs "https://github.com/rancher/rke2/releases/download/${RKE2VERSION}/rke2.linux-amd64.tar.gz" --output "${ROOT_DIR}/assets/rke2/rke2.linux-amd64.tar.gz"
curl -sfL --create-dirs "https://github.com/rancher/rke2/releases/download/${RKE2VERSION}/sha256sum-amd64.txt" --output "${ROOT_DIR}/assets/rke2/sha256sum-amd64.txt"
if [[ ${INSTALL_RKE2_MIRROR} && ${INSTALL_RKE2_MIRROR} == "cn" ]]; then
curl -sfL --create-dirs https://rancher-mirror.rancher.cn/rke2/install.sh --output "${ROOT_DIR}/assets/rke2/install.sh"
else
curl -sfL --create-dirs https://get.rke2.io --output "${ROOT_DIR}/assets/rke2/install.sh"
fi
chmod +x "${ROOT_DIR}/assets/rke2/install.sh"

#pass shellcheck
declare https_proxy
# Configure proxy settings
if [[ ${http_proxy} ]]; then
  if [[ ${HOSTIP} ]]; then
    HOSTIP=",${HOSTIP}"
    no_proxy=${no_proxy},::1${HOSTIP}
  fi
  sudo tee /etc/default/rke2-server &> /dev/null <<EOF


HTTP_PROXY=${http_proxy}
HTTPS_PROXY=${https_proxy}
NO_PROXY=${no_proxy}
EOF
fi

# Install RKE2
if [[ ${INSTALL_RKE2_MIRROR} && ${INSTALL_RKE2_MIRROR} == "cn" ]]; then
  sudo INSTALL_RKE2_ARTIFACT_PATH="${ROOT_DIR}/assets/rke2/" INSTALL_RKE2_MIRROR=cn INSTALL_RKE2_CHANNEL="$RKE2VERSION"  sh "${ROOT_DIR}/assets/rke2/install.sh"
else
  sudo INSTALL_RKE2_ARTIFACT_PATH="${ROOT_DIR}/assets/rke2/" sh "${ROOT_DIR}/assets/rke2/install.sh"
fi
# Check return status of RKE2 install process
# shellcheck disable=SC2181
if [ $? -eq 0 ]
then
  echo "Successfully installed RKE2"
else
  echo "error installing RKE2"
  exit 1
fi

# Create rke2 directory and copy audit-policy file
sudo mkdir -p /etc/rancher/rke2
sudo cp rke2/audit-policy.yaml /etc/rancher/rke2/audit-policy.yaml

# Enable Calico CNI. Disable Canal CNI and Nginx Ingress.
sudo mkdir -p /etc/rancher/rke2
export rancher_ip=`hostname -i`
sudo -E bash -c 'cat << EOF >  /etc/rancher/rke2/config.yaml
write-kubeconfig-mode: "0644"
audit-policy-file: "/etc/rancher/rke2/audit-policy.yaml"
bind-address: $rancher_ip
kube-apiserver-arg:
  - "bind-address=$rancher_ip"
kubelet-arg:
  - address=$rancher_ip
etcd-arg:
  - listen-client-urls=https://$rancher_ip:2379
  - listen-peer-urls=https://$rancher_ip:2380
advertise-address: $rancher_ip
cni:
  - calico
disable:
  - rke2-canal
  - rke2-ingress-nginx
  - rke2-snapshot-controller
  - rke2-snapshot-validation-webhook
kubelet-arg:
  - "max-pods=200"
etcd-arg:
  - --debug=false
  - --log-package-levels=INFO
  - --config-file=/var/lib/rancher/rke2/server/db/etcd/config
  - --quota-backend-bytes=8589934592
  - --auto-compaction-mode=periodic
  - --auto-compaction-retention=1h
services:
  kubelet:
    extra_binds:
      - /var/openebs/local:/var/openebs/local
EOF

# Configure Custom Values to CoreDNS
mkdir -p /var/lib/rancher/rke2/server/manifests/
cat << EOF >  /var/lib/rancher/rke2/server/manifests/rke2-coredns-config.yaml
apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: rke2-coredns
  namespace: kube-system
spec:
  valuesContent: |-
    global:
      clusterCIDR: 10.42.0.0/16
      clusterCIDRv4: 10.42.0.0/16
      clusterDNS: 10.43.0.10
      rke2DataDir: /var/lib/rancher/rke2
      serviceCIDR: 10.43.0.0/16
    service:
      name: "kube-dns"
EOF'

# Copy Calico Images tarball
sudo mkdir -p /var/lib/rancher/rke2/agent/images/
sudo cp "${ROOT_DIR}/assets/rke2/rke2-images-calico.linux-amd64.tar.zst" /var/lib/rancher/rke2/agent/images/

# Enable RKE2
sudo systemctl enable --now rke2-server.service
# Provide some buffer time before checking running status
sleep 5

running=$(systemctl status rke2-server.service | grep 'active (running)')
if [ "$running" == "" ]; then
  echo "RKE2 server is not in active (running) state"
  exit 1
fi
