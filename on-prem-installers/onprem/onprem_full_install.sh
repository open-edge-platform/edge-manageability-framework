#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Script Name: onprem_full_install.sh
# Description: Orchestrates the complete Edge Orchestrator installation by running:
#               1. onprem_pre_installer.sh - System preparation and RKE2 setup
#               2. onprem_installer.sh - Main orchestrator components installation
#
# Usage: ./onprem_full_install.sh [PRE_OPTIONS] [-- MAIN_OPTIONS]
#   PRE_OPTIONS: Options for onprem_pre_installer.sh (--skip-download, -t/--trace)
#   MAIN_OPTIONS: Options for onprem_installer.sh (after --) (-s/--sre, -d/--notls, --disable-*, -t/--trace)
#
# Prerequisites: onprem.env file must exist with proper configuration

set -e
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PRE_INSTALLER="$SCRIPT_DIR/onprem_pre_installer.sh"
MAIN_INSTALLER="$SCRIPT_DIR/onprem_installer.sh"

# Arrays to hold options for each script
PRE_OPTIONS=()
MAIN_OPTIONS=()

# Flag to track which script we're collecting options for
COLLECTING_MAIN=false

usage() {
  cat >&2 <<EOF
Purpose:
Complete Edge Orchestrator installation orchestrator that runs both pre-installer 
and main installer scripts sequentially with their respective options.

Prerequisites:
- onprem.env file must exist with proper configuration
- Root/sudo access for package installation
- Internet connectivity for downloading packages

Usage:
$(basename "$0") [PRE_OPTIONS] [-- MAIN_OPTIONS]

Examples:
./$(basename "$0")
    # Run both installers with default settings

./$(basename "$0") --skip-download
    # Skip package downloads in pre-installer

./$(basename "$0") -- -s /path/to/ca.crt -d
    # Use default pre-installer, enable SRE TLS with CA cert and disable SMTP TLS in main installer

./$(basename "$0") --skip-download -t -- -s /path/to/ca.crt --disable-o11y -t
    # Skip downloads with trace in pre-installer, enable SRE with CA, disable O11y with trace in main installer

./$(basename "$0") -t -- --disable-co --disable-ao
    # Enable trace in pre-installer, disable CO and AO profiles in main installer

Pre-Installer Options (before --):
    -h, --help                 Show this help message and exit
    --skip-download            Skip downloading installer packages from registry
    -t, --trace                Enable bash debug tracing

Main Installer Options (after --):
    -h, --help                 Show help message (will only show main installer help)
    -s, --sre [CA_CERT_PATH]   Enable TLS for SRE with optional CA certificate
    -d, --notls                Disable TLS verification for SMTP endpoint
    --disable-co               Disable Cluster Orchestrator profile
    --disable-ao               Disable Application Orchestrator profile
    --disable-o11y             Disable Observability profile
    -t, --trace                Enable bash debug tracing

Separator:
    --                         Separates pre-installer options from main installer options
                               All options before -- go to pre-installer
                               All options after -- go to main installer

Configuration:
    All configuration is read from onprem.env file in the script directory.

EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --)
      # Switch to collecting main installer options
      COLLECTING_MAIN=true
      shift
      continue
      ;;
    *)
      if [[ "$COLLECTING_MAIN" == true ]]; then
        MAIN_OPTIONS+=("$1")
      else
        PRE_OPTIONS+=("$1")
      fi
      shift
      ;;
  esac
done

# Verify scripts exist
if [[ ! -f "$PRE_INSTALLER" ]]; then
  echo "❌ Error: Pre-installer script not found at: $PRE_INSTALLER"
  exit 1
fi

if [[ ! -f "$MAIN_INSTALLER" ]]; then
  echo "❌ Error: Main installer script not found at: $MAIN_INSTALLER"
  exit 1
fi

# Verify scripts are executable
if [[ ! -x "$PRE_INSTALLER" ]]; then
  echo "Making pre-installer executable..."
  chmod +x "$PRE_INSTALLER"
fi

if [[ ! -x "$MAIN_INSTALLER" ]]; then
  echo "Making main installer executable..."
  chmod +x "$MAIN_INSTALLER"
fi

echo "=========================================="
echo "  Edge Orchestrator Full Installation"
echo "=========================================="
echo

# Run pre-installer
echo "📦 Step 1/2: Running Pre-Installer"
echo "Command: $PRE_INSTALLER ${PRE_OPTIONS[*]}"
echo "------------------------------------------"
"$PRE_INSTALLER" "${PRE_OPTIONS[@]}"

echo
echo "✅ Pre-installer completed successfully!"
echo
echo "=========================================="
echo

# Run main installer
echo "📦 Step 2/2: Running Main Installer"
echo "Command: $MAIN_INSTALLER ${MAIN_OPTIONS[*]}"
echo "------------------------------------------"
"$MAIN_INSTALLER" "${MAIN_OPTIONS[@]}"

echo
echo "=========================================="
echo "✅ Full Installation Completed!"
echo "=========================================="
echo