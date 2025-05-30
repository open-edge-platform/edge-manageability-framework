#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Function to check for specific CRDs
check_crds() {
    local crds=("$@")

    # Function to check if a CRD exists
    check_crd() {
        kubectl get crd "$1" &> /dev/null
    }

    # Check all CRDs
    for crd in "${crds[@]}"; do
        if ! check_crd "$crd"; then
            echo "$crd not found"
            return 1
        fi
    done

    return 0
}

# Function to retry checking CRDs with timeout
await_crds_check() {
    local delay=$1
    local timeout=$2
    local start_time
    start_time=$(date +%s)    
    local end_time=$((start_time + timeout))

    while true; do
        if check_crds "${CRDS[@]}"; then
            return 0
        fi

        current_time=$(date +%s)
        if [ "$current_time" -ge $end_time ]; then
            return 1
        fi

        echo "Waiting for ${delay} seconds for CRD deployment..."
        sleep "$delay"
    done
}

# Function to print usage
usage() {
    echo "Usage: $0 [-t|--timeout TIMEOUT] [-d|--delay DELAY]"
    echo "  -t, --timeout TIMEOUT    Set the maximum timeout in seconds (default: 120)"
    echo "  -d, --delay DELAY        Set the delay between attempts in seconds (default: 5)"
    exit 1
}

# Parse command-line options
TIMEOUT=1800
DELAY=5

while [[ $# -gt 0 ]]; do
    case $1 in
        -t|--timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        -d|--delay)
            DELAY="$2"
            shift 2
            ;;
        *)
            usage
            ;;
    esac
done

# Validate inputs
if ! [[ "$TIMEOUT" =~ ^[0-9]+$ ]] || ! [[ "$DELAY" =~ ^[0-9]+$ ]]; then
    echo "Error: TIMEOUT and DELAY must be positive integers."
    usage
fi

# List of CRDs to check
CRDS=(
    "applications.argoproj.io"
    "applicationsets.argoproj.io"
    "appprojects.argoproj.io"
    # Add more CRDs as needed
)

# Call the retry function with specified delay and timeout
await_crds_check "$DELAY" "$TIMEOUT"

# Capture the exit status
exit_status=$?

if [[ "$exit_status" -eq 0 ]]; then
    echo "Argo CD ready to process application deployment"
else
    echo "Timeout waiting for Argo CD"
fi

# Exit with the appropriate status
exit $exit_status
