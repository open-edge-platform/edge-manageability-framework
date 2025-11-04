#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# reconnect-cluster.sh - Re-establish connection to provisioned cluster
#
# Use this script when running the installer in a new shell session after
# provision.sh has completed. This updates the kubeconfig to connect to
# the EKS cluster.

. ${HOME}/utils.sh

load_provision_env
load_cluster_state_env

update_kube_config

echo "Successfully updated kubeconfig for cluster ${CLUSTER_NAME}"
echo "You can now run: ./configure-cluster.sh"
