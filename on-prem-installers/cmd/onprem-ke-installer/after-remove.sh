#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit

# If rke2 is upgraded it shouldn't be removed from the system at all, upgrade is done by
# installing upgrade-operator on existing cluster and executing upgrade Plan.
# More info can be found in Upgrade:rke2Cluster mage target.
if [ "${1}" = "upgrade" ]; then
    exit 0
fi

# Stop the cluster
/usr/local/bin/rke2-killall.sh || true

# Remove RKE2
/usr/local/bin/rke2-uninstall.sh || true

# Remove artifacts
rm -rf /tmp/onprem-ke-installer || true
