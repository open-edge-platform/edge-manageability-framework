#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -x
set -e
set -o pipefail

# The following labels are required workaround since normal clusters have these labels
# Needed for istio
kubectl label nodes edge-node-control-plane node-role.kubernetes.io/worker=true --overwrite
# Needed for prometheus
kubectl label nodes edge-node-control-plane node-role.kubernetes.io/control-plane=true --overwrite

# The following namespaces are needed for network-policies
kubectl create namespace cattle-system
kubectl create namespace calico-system

# Needed for fluent-bit
if [ -d "/tmp/var/log" ] 
then
    sudo touch /tmp/var/log/syslog /tmp/var/log/auth.log
fi
