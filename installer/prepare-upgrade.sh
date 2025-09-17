#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

. ${HOME}/utils.sh

# Consts
BUCKET_REGION="${BUCKET_REGION:-us-west-2}"
SAVE_DIR="${SAVE_DIR:-${HOME}/pod-configs/SAVEME}"

usage() {
        echo "Usage:"
        echo "prepare-upgrade.sh [--jumphost-ip {JUMPHOST IP ADDRESS}] [--cidr-block {CIDR BLOCK}]"
        echo ""
        echo "Example:"
        echo "    To show the usage:"
        echo "        prepare-upgrade.sh --help  # Show the usage"
        echo "    To start the tunnel using a standard deployment VPC:"
        echo "        prepare-upgrade.sh"
        echo "    To start the tunnel using a custom VPC jumphost:"
        echo "        prepare-upgrade.sh --jumphost-ip 10.139.222.218  --cidr-block 10.250.0.0/16"
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

parse_params "$@"

load_cluster_state_env
if ! load_scm_auth; then
    exit 1
fi
save_scm_auth

update_kube_config

# Clone and init main branch for Code Commit Repos
ORCH_DEPLOY="https://gitea.${CLUSTER_FQDN}/argocd/edge-manageability-framework"

mkdir -p ${HOME}/src

# Clone GitOps Repos
clone_repo $EDGE_MANAGEABILITY_FRAMEWORK edge-manageability-framework

# Load ADMIN_EMAIL from cluster template
export ADMIN_EMAIL=$(yq .argo.adminEmail ${HOME}/src/edge-manageability-framework/orch-configs/clusters/${CLUSTER_NAME}.yaml)
echo ADMIN_EMAIL=${ADMIN_EMAIL} >> ${HOME}/.env

# Update configs and deploy contents required for pre-upgrade
pushd ${HOME}/src/edge-manageability-framework/config
cp ${HOME}/edge-manageability-framework/configs/profiles/enable-aws.yaml profiles/enable-aws.yaml
popd

# Push changes to GitOps Repos
commit_repo edge-manageability-framework "Prepare for upgrade"

./start-tunnel.sh

# Update the ArgoCD deployment
make update

# TBD: Await ArgoCD deployment status update? Add a separate script for this?
