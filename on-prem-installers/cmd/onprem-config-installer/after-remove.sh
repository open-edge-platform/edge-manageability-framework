#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit

# We don't want to remove dependencies on upgrade.
if [ "${1}" = "upgrade" ]; then
    exit 0
fi

# disable loading of kernel modules at boot
rm -fr /etc/modules-load.d/lv-snapshots.conf

# uninstall yq
rm -rf /usr/local/bin/yq

# uninstall helm
rm -rf /usr/local/bin/helm
