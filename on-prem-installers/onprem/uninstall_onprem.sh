#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -x
set -e
set -o pipefail

# Packages in order of removal - reverse order to installation
packages=(
    onprem-orch-installer
    onprem-gitea-installer
    onprem-argocd-installer
    onprem-ke-installer
    onprem-config-installer
)

remove_package() {
    package_name=$1

    echo "Uninstalling package $package_name..."
    sudo dpkg --purge --force-remove-reinstreq "$package_name"
}

for package in "${packages[@]}"; do
    echo "Removing $package"
    remove_package "$package"
done

sudo rm -rf repo_archives/ installers/

# Remove all PVCs created by Orchestrator
sudo rm -rf "/var/openebs"

# Remove leftover rke2 configuration
sudo rm -f /etc/default/rke2-server
