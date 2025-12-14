#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

cat << "EOF"

   ___           _         ___           _        _ _
  / _ \ _ __ ___| |__     |_ _|_ __  ___| |_ __ _| | | ___ _ __
 | | | | '__/ __| '_ \     | || '_ \/ __| __/ _` | | |/ _ \ '__|
 | |_| | | | (__| | | |_   | || | | \__ \ || (_| | | |  __/ |
  \___/|_|  \___|_| |_(_) |___|_| |_|___/\__\__,_|_|_|\___|_|


EOF

set -o errexit

export KUBECONFIG=/home/$USER/.kube/config

# Add /usr/local/bin to the PATH as some utilities, like kubectl, could be installed there
export PATH=$PATH:/usr/local/bin

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Execute the installer script (pure shell, no Go required)
bash "$SCRIPT_DIR/install.sh"
