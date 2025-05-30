#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# shellcheck source=installer/utils.sh
. "${HOME}"/utils.sh

# Consts
export BUCKET_REGION="us-west-2"
SAVE_DIR="${SAVE_DIR:-${HOME}/pod-configs/SAVEME}"

usage() {
        echo "Usage:"
        echo "configure-cluster.sh [--jumphost-ip {JUMPHOST IP ADDRESS}] [--cidr-block {CIDR BLOCK}]"
        echo ""
        echo "Example:"
        echo "    To show the usage:"
        echo "        configure-cluster.sh --help  # Show the usage"
        echo "    To start the tunnel using a standard deployment VPC:"
        echo "        configure-cluster.sh"
        echo "    To start the tunnel using a custom VPC jumphost:"
        echo "        configure-cluster.sh --jumphost-ip 10.139.222.218  --cidr-block 10.250.0.0/16"
}

parse_params() {
    if ! options=$(getopt -o h -l cidr-block:,help,jumphost-ip: -- "$@")
    then
        usage
        exit 1
    fi

    set -- "$options"

    while [ $# -gt 0 ]
    do
        case $1 in
            --cidr-block) VPC_CIDR=$(eval echo "$2"); shift;;
            --jumphost-ip) JUMPHOST_IP=$(eval echo "$2"); shift;;
            -h|--help) usage; exit;;
            (--) shift; break;;
            (-*) echo "$0: error - unrecognized option $1" 1>&2; exit 1;;
            (*) break;;
        esac
        shift
    done

    if [ -n "$VPC_CIDR" ]; then
        export VPC_CIDR=${VPC_CIDR}
    fi
    if [ -n "$JUMPHOST_IP" ]; then
        export JUMPHOST_IP=${JUMPHOST_IP}
    fi
}

load_provision_env

parse_params "$@"

load_cluster_state_env
check_provision_env -p
load_provision_values
save_cluster_env

load_cluster_state_env
if ! load_scm_auth; then
    exit 1
fi
save_scm_auth

update_kube_config


# Create Cluster Configuration
file_system_id=$(aws efs --region "${AWS_REGION}" describe-file-systems --query "FileSystems[?Name == '${CLUSTER_NAME}'].FileSystemId" --output text)
export FILE_SYSTEM_ID="$file_system_id"
s3_prefix=$(get_s3_prefix)
export S3_PREFIX="$s3_prefix"
traefik_tg_hash=$(echo -n "${CLUSTER_NAME}-traefik-default" | sha256sum | cut -c-32)
export TRAEFIK_TG_HASH="${traefik_tg_hash}"
traefik_grpc_tg_hash=$(echo -n "${CLUSTER_NAME}-traefik-grpc" | sha256sum | cut -c-32)
export TRAEFIKGRPC_TG_HASH="${traefik_grpc_tg_hash}"
nginx_tg_hash=$(echo -n "${CLUSTER_NAME}-traefik2-https" | sha256sum | cut -c-32)
export NGINX_TG_HASH="${nginx_tg_hash}"
argocd_tg_hash=$(echo -n "${CLUSTER_NAME}-argocd-default" | sha256sum | cut -c-32)
export ARGOCD_TG_HASH="${argocd_tg_hash}"

traefik_tg_arn=$(aws elbv2 describe-target-groups --names "${TRAEFIK_TG_HASH}" | jq -r '.TargetGroups[].TargetGroupArn')
export TRAEFIK_TG_ARN="$traefik_tg_arn"
if [ -z "$TRAEFIK_TG_ARN" ]; then
    traefik_tg_arn=$(aws elbv2 describe-target-groups --names "${CLUSTER_NAME}"-traefik-default | jq -r '.TargetGroups[].TargetGroupArn')
    export TRAEFIK_TG_ARN="$traefik_tg_arn"
fi
if [ -z "$TRAEFIK_TG_ARN" ]; then
    echo "  error: Load balancer Target Group for ${CLUSTER_NAME} not found."
    exit 1
fi

traefik_grpc_tg_arn=$(aws elbv2 describe-target-groups --names "${TRAEFIKGRPC_TG_HASH}" | jq -r '.TargetGroups[].TargetGroupArn')
export TRAEFIKGRPC_TG_ARN="$traefik_grpc_tg_arn"
nginx_tg_hash=$(aws elbv2 describe-target-groups --names "${NGINX_TG_HASH}" | jq -r '.TargetGroups[].TargetGroupArn')
export NGINX_TG_ARN="$nginx_tg_hash"
argocd_tg_arn=$(aws elbv2 describe-target-groups --names "${ARGOCD_TG_HASH}" | jq -r '.TargetGroups[].TargetGroupArn')
export ARGOCD_TG_ARN="$argocd_tg_arn"
if [ -z "$ARGOCD_TG_ARN" ]; then
    argocd_tg_arn=$(aws elbv2 describe-target-groups --names "${CLUSTER_NAME}"-argocd-https | jq -r '.TargetGroups[].TargetGroupArn')
    export ARGOCD_TG_ARN="$argocd_tg_arn"
fi


if [ -n "$SRE_BASIC_AUTH_USERNAME" ] || [ -n "$SRE_BASIC_AUTH_PASSWORD" ] || [ -n "$SRE_DESTINATION_SECRET_URL" ] || [ -n "$SRE_DESTINATION_CA_SECRET" ]; then
    export SRE_PROFILE="- orch-configs/profiles/enable-sre.yaml"
else
    export SRE_PROFILE="#- orch-configs/profiles/enable-sre.yaml"
fi

if [ -z "$SMTP_URL" ]; then
    export EMAIL_PROFILE="#- orch-configs/profiles/alerting-emails.yaml"
else
    export EMAIL_PROFILE="- orch-configs/profiles/alerting-emails.yaml"
fi

if [ -z "$AUTO_CERT" ]; then
    export AUTOCERT_PROFILE="#- orch-configs/profiles/profile-autocert.yaml"
else
    export AUTOCERT_PROFILE="- orch-configs/profiles/profile-autocert.yaml"
fi

export AWS_PROD_PROFILE="- orch-configs/profiles/profile-aws-production.yaml"
if [[ "$DISABLE_AWS_PROD_PROFILE" == "true" ]]; then
    export AWS_PROD_PROFILE="#- orch-configs/profiles/profile-aws-production.yaml"
fi

export O11Y_PROFILE="- orch-configs/profiles/o11y-release.yaml"
cluster_scale_profile=$(grep -oP '^# Profile: "\K[^"]+' ~/pod-configs/SAVEME/"${AWS_ACCOUNT}"-"${CLUSTER_NAME}"-profile.tfvar)
export CLUSTER_SCALE_PROFILE="${cluster_scale_profile}"
if [[ "$CLUSTER_SCALE_PROFILE" == "500en" || "$CLUSTER_SCALE_PROFILE" == "1ken" || "$CLUSTER_SCALE_PROFILE" == "10ken" ]]; then
    export O11Y_PROFILE="- orch-configs/profiles/o11y-release-large.yaml"
fi

echo
echo "Creating cluster definition for ${CLUSTER_NAME}"
envsubst < cluster.tpl > edge-manageability-framework/orch-configs/clusters/"${CLUSTER_NAME}".yaml

echo
echo =============================================================================
echo "Please review the cluster settings in the generated configuration and make"
echo "any necessary updates."
echo
echo "Press any key to open your editor..."
echo =============================================================================
echo
read -r -n 1
"${EDITOR:-vi}" "edge-manageability-framework/orch-configs/clusters/\"${CLUSTER_NAME}\".yaml"

# Clone / Initialize the GitOps repositories
# Push Release Contents to the GitOps Repos
# Commit the Release Contents GitOps Repos
echo
echo Initializing GitOps Repos
./initialize-gitops-repos.sh

echo Starting VPC tunnel
./start-tunnel.sh

echo
echo =============================================================================
echo The "${CLUSTER_NAME}" should now be ready for deployment.
echo Please verify kubectl access to the cluster before proceeding
echo
echo   kubectl get nodes
echo
echo Then start the Orchestrator installation, for a new install run:
echo
echo   make install
echo
echo If this is an upgrade, run:
echo
echo   make upgrade
echo
echo =============================================================================
echo