#!/bin/bash
# filepath: /home/sunil/OnPrem/edge-manageability-framework/vm-host-wrapper.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Timer functions
start_timer() {
    echo $(date +%s)
}

end_timer() {
    local start_time=$1
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    echo "${duration}s"
}

# Function to parse VM arguments (single, range, or list)
parse_vm_args() {
    local input="$1"
    local vm_list=""
    
    if [[ "$input" == *"-"* ]]; then
        # Range format: 100-120
        local start=$(echo "$input" | cut -d'-' -f1)
        local end=$(echo "$input" | cut -d'-' -f2)
        vm_list=$(seq "$start" "$end" | tr '\n' ' ')
    elif [[ "$input" == *","* ]]; then
        # List format: 100,105,110,115
        vm_list=$(echo "$input" | tr ',' ' ')
    else
        # Single VM: 100
        vm_list="$input"
    fi
    
    echo "$vm_list"
}

host_onboarding() {
    local vm_args="$1"
    local total_start=$(start_timer)
    echo "Starting host onboarding for: $vm_args"
    echo "========================================"
    
    echo "Step 1: Logging in..."
    local step_start=$(start_timer)
    ./host-config-cli.sh login
    echo "Login completed in $(end_timer $step_start)"
    echo ""
    
    echo "Step 2: Creating region site..."
    step_start=$(start_timer)
    ./host-config-cli.sh create-region-site
    echo "Region site creation completed in $(end_timer $step_start)"
    echo ""
    
    echo "Step 3: Listing region sites..."
    step_start=$(start_timer)
    ./host-config-cli.sh list-region-site
    echo "Region site listing completed in $(end_timer $step_start)"
    echo ""
    
    echo "Step 4: Getting profile..."
    step_start=$(start_timer)
    ./host-config-cli.sh get-profile
    echo "Profile retrieval completed in $(end_timer $step_start)"
    echo ""
    
    echo "Step 5: Host NIO registration for $vm_args..."
    step_start=$(start_timer)
    ./host-config-cli.sh host-nio-registration "$vm_args"
    echo "Host NIO registration completed in $(end_timer $step_start)"
    echo ""
    
    echo "Step 6: Creating pico-ven for $vm_args..."
    step_start=$(start_timer)
    ./pico-ven.sh create "$vm_args"
    echo "Pico-ven creation completed in $(end_timer $step_start)"
    echo ""
    
    echo "Step 7: Checking host status for $vm_args..."
    step_start=$(start_timer)
    ./host-config-cli.sh host-status
    echo "Host status check completed in $(end_timer $step_start)"
    echo ""
    
    echo "========================================"
    echo "Host onboarding completed for: $vm_args"
    echo "Total execution time: $(end_timer $total_start)"
}

delete_host() {
    local vm_args="$1"
    local total_start=$(start_timer)
    echo "Starting host deletion for: $vm_args"
    echo "========================================"
    
    echo "Step 1: Logging in..."
    local step_start=$(start_timer)
    ./host-config-cli.sh login
    echo "Login completed in $(end_timer $step_start)"
    echo ""
    
    echo "Step 2: Deleting hosts $vm_args..."
    step_start=$(start_timer)
    ./host-config-cli.sh delete-hosts "$vm_args"
    echo "Host deletion completed in $(end_timer $step_start)"
    echo ""
    
    echo "Step 3: Destroying pico-ven for $vm_args..."
    step_start=$(start_timer)
    yes | ./pico-ven.sh destroy "$vm_args"
    echo "Pico-ven destruction completed in $(end_timer $step_start)"
    echo ""
    
    echo "========================================"
    echo "Host deletion completed for: $vm_args"
    echo "Total execution time: $(end_timer $total_start)"
}

case "$1" in
    host-onboarding)
        host_onboarding "$2"
        ;;
    delete-host)
        delete_host "$2"
        ;;
    *)
        echo "Usage: $0 {host-onboarding|delete-host} <vm-spec>"
        echo ""
        echo "VM specification formats:"
        echo "  Single VM:    $0 host-onboarding 100"
        echo "  Range of VMs: $0 host-onboarding 100-120"
        echo "  List of VMs:  $0 host-onboarding 100,105,110,115"
        echo ""
        echo "Examples:"
        echo "  $0 host-onboarding 100-120"
        echo "  $0 delete-host 100-120"
        exit 1
        ;;
esac
