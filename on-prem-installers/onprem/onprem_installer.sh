#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Script Name: onprem_installer.sh
# Description: Orchestrates the complete Edge Orchestrator installation by running:
#               1. onprem_pre_install.sh - System preparation and RKE2 setup
#               2. onprem_orch_install.sh - Main orchestrator components installation
#
# Usage: ./onprem_installer.sh [PRE_OPTIONS] [-- MAIN_OPTIONS]
#   PRE_OPTIONS: Options for onprem_pre_install.sh (--skip-download, -t/--trace)
#   MAIN_OPTIONS: Options for onprem_orch_install.sh (after --) (-s/--sre, -d/--notls, --disable-*, -t/--trace)
#
# Prerequisites: onprem.env file must exist with proper configuration

set -e
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PRE_INSTALL="$SCRIPT_DIR/onprem_pre_install.sh"
MAIN_INSTALL="$SCRIPT_DIR/onprem_orch_install.sh"

# Arrays to hold options for each script
PRE_OPTIONS=()
MAIN_OPTIONS=()

# Flag to track which script we're collecting options for
COLLECTING_MAIN=false

usage() {
  cat >&2 <<EOF
Purpose:
Complete Edge Orchestrator installation orchestrator that runs both pre-install 
and main install scripts sequentially with their respective options.

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
    # Skip package downloads in pre-install

./$(basename "$0") -- -s /path/to/ca.crt -d
    # Use default pre-install, enable SRE TLS with CA cert and disable SMTP TLS in main install

./$(basename "$0") --skip-download -t -- -s /path/to/ca.crt --disable-o11y -t
    # Skip downloads with trace in pre-install, enable SRE with CA, disable O11y with trace in main install

./$(basename "$0") -t -- --disable-co --disable-ao
    # Enable trace in pre-install, disable CO and AO profiles in main install

./$(basename "$0") -- -y -s /path/to/ca.crt
    # Run with non-interactive mode, enable SRE with CA cert in main install

./$(basename "$0") -- -st --disable-o11y
    # Enable single tenancy mode and disable observability in main install

Pre-Install Options (before --):
    -h, --help                 Show this help message and exit
    --skip-download            Skip downloading install packages from registry
    -t, --trace                Enable bash debug tracing

Main Install Options (after --):
    -h, --help                 Show help message (will only show main install help)
    -s, --sre [CA_CERT_PATH]   Enable TLS for SRE with optional CA certificate
    -d, --notls                Disable TLS verification for SMTP endpoint
    -y, --yes                  Assume 'yes' to all prompts and run non-interactively
    --disable-co               Disable Cluster Orchestrator profile
    --disable-ao               Disable Application Orchestrator profile
    --disable-o11y             Disable Observability profile
    -st, --single_tenancy      Enable single tenancy mode
    -t, --trace                Enable bash debug tracing

Separator:
    --                         Separates pre-install options from main install options
                               All options before -- go to pre-install
                               All options after -- go to main install

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
      # Switch to collecting main install options
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
if [[ ! -f "$PRE_INSTALL" ]]; then
  echo "❌ Error: Pre-install script not found at: $PRE_INSTALL"
  exit 1
fi

if [[ ! -f "$MAIN_INSTALL" ]]; then
  echo "❌ Error: Main install script not found at: $MAIN_INSTALL"
  exit 1
fi

# Verify scripts are executable
if [[ ! -x "$PRE_INSTALL" ]]; then
  echo "Making pre-install executable..."
  chmod +x "$PRE_INSTALL"
fi

if [[ ! -x "$MAIN_INSTALL" ]]; then
  echo "Making main install executable..."
  chmod +x "$MAIN_INSTALL"
fi

echo "=========================================="
echo "  Edge Orchestrator Full Installation"
echo "=========================================="
echo

# Run pre-install
echo "📦 Step 1/2: Running Pre-Install"
echo "Command: $PRE_INSTALL ${PRE_OPTIONS[*]}"
echo "------------------------------------------"
"$PRE_INSTALL" "${PRE_OPTIONS[@]}"

echo
echo "✅ Pre-install completed successfully!"
echo
echo "=========================================="
echo

# Run main install
echo "📦 Step 2/2: Running Main Install"
echo "Command: $MAIN_INSTALL ${MAIN_OPTIONS[*]}"
echo "------------------------------------------"
"$MAIN_INSTALL" "${MAIN_OPTIONS[@]}"

echo
echo "=========================================="
echo "✅ Full Installation Completed!"
echo "=========================================="
echo