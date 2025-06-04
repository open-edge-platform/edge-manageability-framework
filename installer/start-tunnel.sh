#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Get the directory of the calling script
SCRIPT_DIR=$(dirname "$(realpath "$0")")

if [ -f "${SCRIPT_DIR}/utils.sh" ]; then
    # shellcheck source=installer/utils.sh
    . "${SCRIPT_DIR}"/utils.sh
else
    echo "Error: Unable to load utils.sh"
    exit 1
fi

usage() {
        echo "Usage:"
        echo "start-tunnel.sh [--jumphost-ip {JUMPHOST IP ADDRESS}] [--cidr-block {CIDR BLOCK}]"
        echo ""
        echo "Example:"
        echo "    To show the usage:"
        echo "        start-tunnel.sh --help  # Show the usage"
        echo "    To start the tunnel using a standard deployment VPC:"
        echo "        start-tunnel.sh"
        echo "    To start the tunnel using a custom VPC jumphost:"
        echo "        start-tunnel.sh --jumphost-ip 10.139.222.218  --cidr-block 10.250.0.0/16"
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
        export VPC_CIDR="${VPC_CIDR}"
    fi
    if [ -n "$JUMPHOST_IP" ]; then
        export JUMPHOST_IP="${JUMPHOST_IP}"
    fi
}

# Consts
export BUCKET_REGION="us-west-2"
SAVE_DIR="${SAVE_DIR:-${HOME}/pod-configs/SAVEME}"
export SOCKS_PROXY="${socks_proxy:-}"

JUMPHOST_IP="${JUMPHOST_IP:-}"
VPC_CIDR="${VPC_CIDR:-}"

# Debug
load_provision_env
load_cluster_state_env

load_provision_values

parse_params "$@"

if [ -z "$VPC_CIDR" ]; then
    VPC_CIDR=$(aws ec2 describe-vpcs --region "${AWS_REGION}" --filter Name=tag:Name,Values="${CLUSTER_NAME}" --query Vpcs[].CidrBlock --output text)
fi

# Use common tunnel connection implementation
refresh_sshuttle
