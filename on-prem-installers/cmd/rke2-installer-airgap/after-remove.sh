#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit

# Stop the cluster
/usr/local/bin/rke2-killall.sh || true

# Remove RKE2
/usr/local/bin/rke2-uninstall.sh || true

# Remove artifacts
rm -rf /tmp/rke2-installer-airgap || true
