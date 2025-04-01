#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit

# Change directories to the path where artifacts are installed
cd /tmp/rke2-installer-airgap

# Add /usr/local/bin to the PATH as some utilities, like kubectl, could be installed there
export PATH=$PATH:/usr/local/bin

# Execute the installer with the current directory as context
/usr/bin/rke2-installer-airgap

# Clean up artifacts directory
rm -rf /tmp/rke2-installer-airgap
