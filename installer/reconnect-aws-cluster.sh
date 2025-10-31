#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Run this script to re-establish connection to a provisioned AWS EKS cluster
# from a new shell session. This updates your kubeconfig with the cluster
# credentials before running configure-cluster.sh.

. ${HOME}/utils.sh

load_provision_env
load_cluster_state_env
update_kube_config
