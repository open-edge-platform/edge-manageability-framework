#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

. ${HOME}/utils.sh

# Consts
BUCKET_REGION="us-west-2"
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

    set -- $options

    while [ $# -gt 0 ]
    do
        case $1 in
            --cidr-block) VPC_CIDR=$(eval echo $2); shift;;
            --jumphost-ip) JUMPHOST_IP=$(eval echo $2); shift;;
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
save_cluster_env

load_cluster_state_env
if ! load_scm_auth; then
    exit 1
fi
save_scm_auth

#
# Create Cluster Configuration
#
if [ -z "$FILE_SYSTEM_ID" ] || [ -z "$TRAEFIK_TG_ARN" ] || [ -z "$ARGOCD_TG_ARN" ]; then
    echo "  Missing one or more of: FILE_SYSTEM_ID, TRAEFIK_TG_ARN, ARGOCD_TG_ARN"
    echo "  Please run provision.sh first."
    exit 1
fi

export FILE_SYSTEM_ID
export TRAEFIK_TG_ARN
export TRAEFIKGRPC_TG_ARN
export NGINX_TG_ARN
export ARGOCD_TG_ARN
export S3_PREFIX

source ./generate_cluster_yaml.sh aws

cp -rf ${CLUSTER_NAME}.yaml edge-manageability-framework/orch-configs/clusters/

echo
echo =============================================================================
echo "Please review the cluster settings in the generated configuration and make"
echo "any necessary updates."
echo
echo "Press any key to open your editor..."
echo =============================================================================
echo
read -n 1
"${EDITOR:-vi}" "edge-manageability-framework/orch-configs/clusters/${CLUSTER_NAME}.yaml"

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
echo The ${CLUSTER_NAME} should now be ready for deployment.
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
