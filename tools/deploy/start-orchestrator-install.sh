#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Default values
#embedded_images=0
embedded_images=1
release_service_url=${RELEASE_SERVICE:-registry-rs.edgeorchestration.intel.com}
AWS_REGION="${AWS_REGION:-us-west-2}"
BUCKET_REGION="${BUCKET_REGION:-$AWS_REGION}"

# echo "Embedded Images: $embedded_images"
# echo "Release Service URL: $release_service_url"

# Prompt for the installation type
while true; do
    echo "Select an installation option:"
    echo "  1. Provision and deploy a Cloud Edge Orchestrator"
    echo "  2. Manage an existing Cloud Edge Orchestrator"
    echo "  3. Uninstall and deprovision an existing Cloud Edge Orchestrator"
    read -p "Your selection (default [1]): " install_type
    if [[ -z "$install_type" ]]; then
        install_type="1"
    fi

    case "$install_type" in
    "1")
        deploy_type="cloudfull"
        # Start on installer script
        deploy_op="install"
        break
        ;;
    "2")
        deploy_type="cloudfull"
        # Start on bash prompt
        deploy_op="update"
        break
        ;;
    "3")
        deploy_type="cloudfull"
        # Start on bash prompt
        deploy_op="uninstall"
        break
        ;;
    *)
        echo "error: Unknown option: $install_type"
        echo ""
        ;;
    esac
done

# echo "Deploy Type: $deploy_type"
# echo "Operation: $deploy_op"

# Prompt for the target cluster name. Default to CLUSTER_NAME environment variable.
while true; do
    read -p "Enter the name of the cluster [$CLUSTER_NAME]: " cluster
    if [[ -z ${cluster} ]]; then
        cluster=$CLUSTER_NAME
    fi
    if [[ -n ${cluster} ]]; then
        break
    else
        echo "Error: A cluster name is required."
    fi
done

# Prompt for the install target AWS region. Default to AWS_REGION environment variable.
while true; do
    read -p "Specify the AWS region for the cluster (default [$AWS_REGION]): " region
    if [[ -z ${region} ]]; then
        region=$AWS_REGION
    fi

    if [[ "$region" =~ ^(us|ca|eu|ap)\-[a-z]+\-[0-9]+$ ]]; then
        break
    else
        echo "Error: Invalid AWS region. Please enter the region where your cluster is or will be deployed."
    fi
done

# Prompt for the AWS region for installer state bucket. Default to BUCKET_REGION environment variable.
while true; do
    read -p "Specify the AWS region for the bucket to store the installer state (default [$BUCKET_REGION]): " bucket_region
    if [[ -z ${region} ]]; then
        bucket_region=$BUCKET_REGION
    fi

    if [[ "$bucket_region" =~ ^(us|ca|eu|ap)\-[a-z]+\-[0-9]+$ ]]; then
        break
    else
        echo "Error: Invalid bucket region. Please enter the region where your bucket is or will be deployed."
    fi
done

# Prompt the location for CUSTOMER_STATE_PREFIX and local SAVEME content
echo
echo "Orchestrator deployment details will be archived in an S3 store in your AWS account. The S3 bucket name is"
echo "derived from application and account details combined with a unique state data identifier supplied at installation"
echo "time."
echo
echo "The same unique state data identifier must be used to retrieve the state data for install, update, and uninstall" 
echo "operations for a given set of Orchestrator instances."
echo
echo "State data for multiple Orchestrator instances may be saved using the same identifier. It is recommended to use a"
echo "state data identifier that is meaningful for a customer organization."
echo

while true; do
    read -p "Specify the state data identifier for the cluster (default [$CUSTOMER_STATE_PREFIX]): " state_prefix
    if [[ -z ${state_prefix} ]]; then
        state_prefix=$CUSTOMER_STATE_PREFIX
    fi

    if [[ -n ${state_prefix} ]]; then
        break
    else
        echo "Error: A state data identifier is required."
    fi
done

mount_save_param=""
while true; do
    echo "If you wish to keep a local copy of the cluster provisioning state data, specify a local path here."
    echo
    read -p "local state path (default []): " saveme_path
    if [[ -z "$saveme_path" ]]; then
        saveme_path=""
    fi

    if [[ -z "$saveme_path" ]]; then
        break
    else
        mkdir -p "$saveme_path"
        if [[ -d "$saveme_path" ]]; then
            mount_save_param="-v $(realpath $saveme_path):/root/pod-configs/SAVEME"
            break
        else
            echo "error: $saveme_path does not exist and can't be created"
        fi
    fi
done

mount_src_param=""
# installer_path=""
# if [ -n "$installer_path" ]; then
#     mount_src_param="-v $(realpath $installer_path):/root/isrc"
# fi

mount_gitops_param=""
# install_gitops_path=""
# if [ -n "$install_gitops_path" ]; then
#     mkdir -p $install_gitops_path
#     mount_gitops_param="-v $(realpath $install_gitops_path):/root/src"
# fi

image_name=$release_service_url/edge-orch/common/orchestrator-installer-${deploy_type}
image_tag=$(cat VERSION)

if [ $embedded_images -eq 1 ]; then
    echo Loading embedded image: ${image_name}:${image_tag}...
    docker load <orchestrator-installer-${deploy_type}.tar
fi

echo Launching Orchestrator Installer image: ${image_name}:${image_tag}...
docker run -ti --rm --name orchestrator-admin \
    -v ~/.aws:/tmp/.aws:ro \
    $mount_save_param $mount_src_param $mount_gitops_param \
    --cap-add NET_ADMIN --cap-add NET_RAW \
    -e http_proxy -e https_proxy -e socks_proxy -e no_proxy=${no_proxy},.eks.amazonaws.com \
    -e CLUSTER_NAME=${cluster} -e TARGET_ENV=${cluster} -e AWS_REGION=${region} -e CUSTOMER_STATE_PREFIX=${state_prefix} \
    -e AWS_PROFILE -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_SESSION_TOKEN \
    -e USER="root" -e ORCH_DEFAULT_PASSWORD -e DEPLOY_OP=${deploy_op} \
    ${image_name}:${image_tag} \
    bash
