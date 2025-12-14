#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Script Name: onprem_pre_install.sh
# Description: This script prepares the system for Edge Orchestrator installation by:
#               - Loading configuration from onprem.env file
#               - Downloading installer and repository artifacts
#               - Setting up OS level dependencies
#               - Installing RKE2 Kubernetes cluster
#               - Configuring basic cluster components
#
# Usage: ./onprem_pre_install.sh [OPTIONS]
#   Options:
#     -h, --help         Show help message
#     --skip-download    Skip downloading packages (use existing ones)
#     -y, --yes          Skip Docker credentials prompt and run non-interactively
#     -t, --trace        Enable debug tracing
#
# Prerequisites: onprem.env file must exist with proper configuration

set -e
set -o pipefail

# Import shared functions
# shellcheck disable=SC1091
source "$(dirname "$0")/functions.sh"

SKIP_DOWNLOAD=false
ENABLE_TRACE=false
ASSUME_YES=false

# Source main environment configuration if it exists
MAIN_ENV_CONFIG="$(dirname "$0")/onprem.env"
if [[ -f "$MAIN_ENV_CONFIG" ]]; then
  echo "Loading environment configuration from: $MAIN_ENV_CONFIG"
  # shellcheck disable=SC1090
  source "$MAIN_ENV_CONFIG"
else
  echo "Warning: onprem.env file not found. Please ensure it exists with proper configuration."
  exit 1
fi

### Variables
cwd=$(pwd)
archives_rs_path="edge-orch/common/files/orchestrator"
si_config_repo="edge-manageability-framework"
installer_rs_path="edge-orch/common/files"
export GIT_REPOS=$cwd/$git_arch_name

# Variables that depend on the above and might require updating later, are placed in here
set_artifacts_version() {
  # Note: Installer list kept for version reference only - not used for downloads
  # Installation is done directly from source code
  
  git_archive_list=(
    "onpremfull:${DEPLOY_VERSION}"
  )
}


allow_config_in_runtime() {
  if [ "$ENABLE_TRACE" = true ]; then
    echo "Tracing is enabled. Temporarily disabling tracing"
    set +x
  fi

  tmp_dir="$cwd/$git_arch_name/tmp"

  ## Untar edge-manageability-framework repo
  repo_file=$(find "$cwd/$git_arch_name" -name "*$si_config_repo*.tgz" -type f -printf "%f\n")

  rm -rf "$tmp_dir"
  mkdir -p "$tmp_dir"
  tar -xf "$cwd/$git_arch_name/$repo_file" -C "$tmp_dir"

  if [ "$ASSUME_YES" = true ]; then
    echo "Assuming yes to use existing configuration."
    return
  fi

  # Prompt for Docker.io credentials
  ## Docker Hub usage and limits: https://docs.docker.com/docker-hub/usage/
  while true; do
    if [[ -z $DOCKER_USERNAME && -z $DOCKER_PASSWORD ]]; then
      read -rp "Would you like to provide Docker credentials? (Y/n): " yn
      case $yn in
        [Yy]* ) echo "Enter Docker Username:"; read -r DOCKER_USERNAME; export DOCKER_USERNAME; echo "Enter Docker Password:"; read -r -s DOCKER_PASSWORD; export DOCKER_PASSWORD; break;;
        [Nn]* ) echo "The installation will proceed without using Docker credentials."; unset DOCKER_USERNAME; unset DOCKER_PASSWORD; break;;
        * ) echo "Please answer yes or no.";;
      esac
    else
      echo "Setting Docker credentials."
      export DOCKER_USERNAME
      export DOCKER_PASSWORD
      break
    fi
  done

  if [[ -n $DOCKER_USERNAME && -n $DOCKER_PASSWORD ]]; then
    echo "Docker credentials are set."
  else
    echo "Docker credentials are not valid. The installation will proceed without using Docker credentials."
    unset DOCKER_USERNAME
    unset DOCKER_PASSWORD
  fi

}

usage() {
  cat >&2 <<EOF
Purpose:
Install OnPrem Edge Orchestrator pre-installation components including RKE2, dependencies, 
and package downloads. This script prepares the system for the main orchestrator installation.

Prerequisites:
- onprem.env file must exist in the same directory with proper configuration
- Root/sudo access for package installation
- Internet connectivity for downloading packages

Usage:
$(basename "$0") [OPTIONS]

Examples:
./$(basename "$0")                    # Basic installation with onprem.env config
./$(basename "$0") --skip-download    # Skip package downloads (use existing packages)
./$(basename "$0") -y                 # Skip Docker credentials prompt, run non-interactively
./$(basename "$0") -t                 # Enable debug tracing

Options:
    -h, --help                 Show this help message and exit
    
    --skip-download            Skip downloading installer packages from registry
                               Useful for development/testing when packages already exist
    
    -y, --yes                  Skip Docker credentials prompt and run non-interactively
                               Useful for automated deployments or CI/CD pipelines
    
    -t, --trace                Enable bash debug tracing (set -x)
                               Shows detailed command execution for troubleshooting

Configuration:
    All configuration is read from onprem.env file. Key variables include:
    - DEPLOY_VERSION: Version of Edge Orchestrator to deploy
    - ORCH_INSTALLER_PROFILE: Deployment profile (onprem/onprem-dev)
    - DOCKER_USERNAME/DOCKER_PASSWORD: Docker Hub credentials
    
    Note: Installation runs from local source code - no downloads from release service

EOF
}

print_env_variables() {
  echo; echo "========================================"
  echo "         Environment Variables"
  echo "========================================"
  printf "%-25s: %s\n" "ORCH_INSTALLER_PROFILE" "$ORCH_INSTALLER_PROFILE"
  printf "%-25s: %s\n" "DEPLOY_VERSION" "$DEPLOY_VERSION"
  echo "========================================"; echo
}

# Function to write shared variables to a configuration file for use by onprem_orch_install.sh
write_shared_variables() {
  local config_file="$cwd/onprem.env"
  
  # Update Docker credentials using common function
  update_config_variable "$config_file" "DOCKER_USERNAME" "${DOCKER_USERNAME}"
  update_config_variable "$config_file" "DOCKER_PASSWORD" "${DOCKER_PASSWORD}"
  
  echo "Runtime configuration updated in: $config_file"
  echo "To use in onprem_orch_install.sh, source this file: source $config_file"
}

################################
##### INSTALL SCRIPT START #####
################################

if [ -n "${1-}" ]; then
  while :; do
    case "$1" in
      -h|--help)
        usage
        exit 0
      ;;
      --skip-download)
        SKIP_DOWNLOAD=true
      ;;
      -t|--trace)
        set -x
        ENABLE_TRACE=true
      ;;
      -y|--yes)
        ASSUME_YES=true
      ;;
      -?*)
        echo "Unknown argument $1"
        exit 1
      ;;
      *) break
    esac
    shift
  done
fi

### Installer
echo "Running On Premise Edge Orchestrator pre-install"

# Print environment variables
print_env_variables

# Set the version of the artifacts to be deployed
set_artifacts_version

# Check & install script dependencies
check_oras
install_yq

# Note: All installation is done from local source code
# No downloads from release service needed
echo "Using local source code from repository for installation"

# Config - interactive
allow_config_in_runtime

# Run OS Configuration installer from source
echo "Running the OS level configuration installer..."
installer_script="$cwd/../cmd/onprem-config-installer/after-install.sh"
if [[ -f "$installer_script" ]]; then
    sudo bash "$installer_script"
    echo "OS level configuration completed"
else
    echo "❌ Installer script not found: $installer_script"
    echo "Please ensure you are running from the on-prem-installers/onprem directory."
    exit 1
fi

# Run K8s Installer from source
echo "Installing RKE2..."
installer_script="$cwd/../cmd/onprem-ke-installer/after-install.sh"
if [[ -f "$installer_script" ]]; then
    if [[ -n "${DOCKER_USERNAME}" && -n "${DOCKER_PASSWORD}" ]]; then
        echo "Docker credentials provided. Installing RKE2 with Docker credentials"
        sudo DOCKER_USERNAME="${DOCKER_USERNAME}" DOCKER_PASSWORD="${DOCKER_PASSWORD}" bash "$installer_script"
    else
        sudo bash "$installer_script"
    fi
    echo "RKE2 Installed"
else
    echo "❌ Installer script not found: $installer_script"
    echo "Please ensure you are running from the on-prem-installers/onprem directory."
    exit 1
fi

mkdir -p /home/"$USER"/.kube
sudo cp  /etc/rancher/rke2/rke2.yaml /home/"$USER"/.kube/config
sudo chown -R "$USER":"$USER"  /home/"$USER"/.kube
sudo chmod 600 /home/"$USER"/.kube/config

# Write shared variables to configuration file for use by onprem_orch_install.sh
write_shared_variables

echo "End On Premise Edge Orchestrator pre-install"