#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Exit if any of the intermediate steps fail
set -o errexit -o nounset -o pipefail -o xtrace

# Check if sshpass is installed
if ! command -v sshpass &> /dev/null
then
    echo "sshpass could not be found, please install it."
    exit 1
fi

# Read the input from stdin. jq will ensure that the values are properly quoted and escaped for consumption by the
# shell.
eval "$(jq -r '@sh "SSHPASS_PATH=\(.sshpass_path) PM_SSH_HOST=\(.pm_ssh_host) PM_SSH_PORT=\(.pm_ssh_port) PM_SSH_USER=\(.pm_ssh_user) PM_SSH_PASSWORD=\(.pm_ssh_password) VM_ID=\(.vm_id) VM_SSH_USER=\(.vm_ssh_user) VM_SSH_PASSWORD=\(.vm_ssh_password) VM_SSH_PORT=\(.vm_ssh_port)"')"

# Inject the password into the ssh command
export SSH_PASSWORD="${PM_SSH_PASSWORD}"

VM_SSH_HOST=$("${SSHPASS_PATH}" ssh -o LogLevel=ERROR -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no "${PM_SSH_USER}@${PM_SSH_HOST}" -p "${PM_SSH_PORT}" qm agent "${VM_ID}" network-get-interfaces | grep -v -e password | jq -r '.[] | select(.name == "ens18") | ."ip-addresses" | .[] | select(."ip-address-type" == "ipv4") | ."ip-address"')
if [ -z "${VM_SSH_HOST}" ]; then
    echo "Failed to get the IPv4 address of the VM with ID ${VM_ID}"
    exit 1
fi

jq -n "{\"vm_ssh_host\":\"${VM_SSH_HOST}\"}"