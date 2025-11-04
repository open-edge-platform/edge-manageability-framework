#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Run this script to re-establish connection to a provisioned AWS EKS cluster
# from a new shell session. This updates your kubeconfig with the cluster
# credentials before running configure-cluster.sh.

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    echo "Error: This script must be sourced, not executed."
    echo "Usage:"
    echo "  source ./reconnect-aws-cluster.sh"
    echo "  OR"
    echo "  . ./reconnect-aws-cluster.sh"
    exit 1
fi

. ${HOME}/utils.sh

load_provision_env
load_cluster_state_env
update_kube_config

# Get FILE_SYSTEM_ID from cluster terraform state
export FILE_SYSTEM_ID=$(aws s3 cp s3://${BUCKET_NAME}/${AWS_REGION}/cluster/${CLUSTER_NAME} - 2>/dev/null | jq -r '.outputs.efs_file_system_id.value // empty')
export TRAEFIK_TG_ARN=$(aws s3 cp s3://${BUCKET_NAME}/${AWS_REGION}/orch-load-balancer/${CLUSTER_NAME} - 2>/dev/null | jq -r '.outputs.traefik_target_groups.value.default.arn // empty')
export TRAEFIKGRPC_TG_ARN=$(aws s3 cp s3://${BUCKET_NAME}/${AWS_REGION}/orch-load-balancer/${CLUSTER_NAME} - 2>/dev/null | jq -r '.outputs.traefik_target_groups.value.grpc.arn // empty')
export NGINX_TG_ARN=$(aws s3 cp s3://${BUCKET_NAME}/${AWS_REGION}/orch-load-balancer/${CLUSTER_NAME} - 2>/dev/null | jq -r '.outputs.traefik2_target_groups.value.https.arn // empty')
export ARGOCD_TG_ARN=$(aws s3 cp s3://${BUCKET_NAME}/${AWS_REGION}/orch-load-balancer/${CLUSTER_NAME} - 2>/dev/null | jq -r '.outputs.argocd_target_groups.value.argocd.arn // empty')
export S3_PREFIX=$(get_s3_prefix)

echo "Environment variables loaded. You can now run: ./configure-cluster.sh"