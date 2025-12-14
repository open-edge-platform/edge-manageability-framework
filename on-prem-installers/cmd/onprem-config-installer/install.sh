#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# OnPrem OS Configuration Installer
# This script configures the operating system for Edge Orchestrator installation

set -o errexit
set -o pipefail

cat << "EOF"
 ____            _                    ____             __ _       
/ ___| _   _ ___| |_ ___ _ __ ___    / ___|___  _ __  / _(_) __ _ 
\___ \| | | / __| __/ _ \ '_ ' _ \  | |   / _ \| '_ \| |_| |/ _' |
 ___) | |_| \__ \ ||  __/ | | | | | | |__| (_) | | | |  _| | (_| |
|____/ \__, |___/\__\___|_| |_| |_|  \____\___/|_| |_|_| |_|\__, |
       |___/                                                |___/ 
EOF

echo "Configuring OS for Edge Orchestrator..."

# Add /usr/local/bin to the PATH
export PATH=$PATH:/usr/local/bin

#######################################
# Update sysctl configuration for inotify
#######################################
update_sysctl_config() {
    local sysctl_file="/etc/sysctl.conf"
    local found_max_queued_events=false
    local found_max_user_instances=false
    local found_max_user_watches=false

    echo "Updating sysctl configuration..."

    # Check if values already exist in sysctl.conf
    if grep -q "^fs.inotify.max_queued_events = 1048576" "$sysctl_file" 2>/dev/null; then
        found_max_queued_events=true
    fi
    if grep -q "^fs.inotify.max_user_instances = 1048576" "$sysctl_file" 2>/dev/null; then
        found_max_user_instances=true
    fi
    if grep -q "^fs.inotify.max_user_watches = 1048576" "$sysctl_file" 2>/dev/null; then
        found_max_user_watches=true
    fi

    # Append missing values
    if [ "$found_max_queued_events" = false ]; then
        echo "fs.inotify.max_queued_events = 1048576" >> "$sysctl_file"
    fi
    if [ "$found_max_user_instances" = false ]; then
        echo "fs.inotify.max_user_instances = 1048576" >> "$sysctl_file"
    fi
    if [ "$found_max_user_watches" = false ]; then
        echo "fs.inotify.max_user_watches = 1048576" >> "$sysctl_file"
    fi

    # Apply sysctl settings
    sysctl -p
}

#######################################
# Install yq tool
#######################################
install_yq() {
    local yq_file="yq_linux_amd64.tar.gz"
    
    echo "Installing yq tool..."
    
    # Get latest version
    local version
    version=$(curl -s -L -I -o /dev/null -w '%{url_effective}' https://github.com/mikefarah/yq/releases/latest | sed 's|.*/||')
    
    local yq_url="https://github.com/mikefarah/yq/releases/download/${version}/${yq_file}"
    
    # Download and install yq
    cd /tmp
    rm -f "$yq_file"
    curl -fsSL -o "/tmp/${yq_file}" "$yq_url"
    tar xvf "/tmp/${yq_file}" -C /usr/local/bin
    mv /usr/local/bin/yq_linux_amd64 /usr/local/bin/yq
    chmod +x /usr/local/bin/yq
    
    echo "yq installed successfully"
}

#######################################
# Install Helm tool
#######################################
install_helm() {
    local helm_script="get_helm.sh"
    local helm_version="v3.12.3"
    
    echo "Installing Helm ${helm_version}..."
    
    local helm_url="https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3"
    
    # Download and install Helm
    cd /tmp
    rm -f "$helm_script"
    curl -fsSL -o "/tmp/${helm_script}" "$helm_url"
    chmod 700 "/tmp/${helm_script}"
    "/tmp/${helm_script}" --version "$helm_version"
    
    echo "Helm installed successfully"
}

#######################################
# Configure kernel modules for LV snapshots
#######################################
configure_kernel_modules() {
    echo "Configuring kernel modules for LV snapshots..."
    
    local modules_file="/etc/modules-load.d/lv-snapshots.conf"
    
    # Create modules configuration
    cat > "$modules_file" << 'MODEOF'
dm-snapshot
dm-mirror
MODEOF
    
    # Load modules
    modprobe dm-snapshot
    modprobe dm-mirror
    
    echo "Kernel modules configured"
}

#######################################
# Ensure hostpath directories exist
#######################################
ensure_hostpath_directories() {
    echo "Ensuring hostpath directories exist..."
    
    local dirs=("/var/openebs/local")
    
    for dir in "${dirs[@]}"; do
        if [ ! -d "$dir" ]; then
            mkdir -p "$dir"
            chmod 755 "$dir"
            echo "Created directory: $dir"
        fi
    done
}

#######################################
# Main installation flow
#######################################
main() {
    # Update sysctl configuration
    update_sysctl_config
    
    # Install required tools
    install_yq
    install_helm
    
    # Ensure hostpath directories
    ensure_hostpath_directories
    
    # Configure kernel modules
    configure_kernel_modules
    
    echo ""
    echo "✓ OnPrem OS configuration completed successfully!"
    echo ""
}

# Run main function
main
