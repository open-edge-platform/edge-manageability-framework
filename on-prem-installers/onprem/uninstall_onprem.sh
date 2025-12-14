#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -x
set -e
set -o pipefail

# Script to uninstall on-premise Edge Orchestrator
# This script removes all installed components in reverse order

echo "Starting Edge Orchestrator uninstallation..."

# Stop and remove RKE2 cluster if running
if systemctl is-active --quiet rke2-server; then
    echo "Stopping RKE2 server..."
    sudo systemctl stop rke2-server
    sudo systemctl disable rke2-server
fi

# Clean up RKE2 installation
if [ -f "/usr/local/bin/rke2-uninstall.sh" ]; then
    echo "Removing RKE2..."
    sudo /usr/local/bin/rke2-uninstall.sh
fi

# Remove kubectl configuration
echo "Removing kubectl configuration..."
rm -rf "$HOME/.kube"

# Remove installation artifacts and downloaded archives
echo "Removing installation artifacts..."
sudo rm -rf repo_archives/

# Remove all PVCs created by Orchestrator
sudo rm -rf "/var/openebs"

# Remove leftover rke2 configuration
sudo rm -f /etc/default/rke2-server
