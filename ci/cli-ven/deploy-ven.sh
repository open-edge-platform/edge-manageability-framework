#!/bin/bash
# filepath: /home/sunil/OnPrem/edge-manageability-framework/vm-host-wrapper.sh

set -e

# Timer functions
start_timer() {
    date +%s
}

end_timer() {
    local start_time=$1
    local end_time
    end_time=$(date +%s)
    local duration=$((end_time - start_time))
    echo "${duration}s"
}

# Function to parse VM arguments (single, range, or list)
parse_vm_args() {
    local input="$1"
    local vm_list=""
    
    if [[ "$input" == *"-"* ]]; then
        # Range format: 100-120
        local start
        local end
        start=$(echo "$input" | cut -d'-' -f1)
        end=$(echo "$input" | cut -d'-' -f2)
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

# Function to initialize logging
init_logging() {
    local log_file="./ven-logs.log"
    local timestamp
    timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo "========================================" | tee -a "$log_file"
    echo "VEN Script Execution Started: $timestamp" | tee -a "$log_file"
    echo "Command: $0 $*" | tee -a "$log_file"
    echo "========================================" | tee -a "$log_file"
}

host_onboarding() {
    local vm_args="$1"
    local log_file="./ven-logs.log"
    local total_start
    total_start=$(start_timer)
    
    # Arrays to track stage names and times
    local stage_names=()
    local stage_times=()
    
    echo "Starting host onboarding for: $vm_args" | tee -a "$log_file"
    echo "========================================" | tee -a "$log_file"
    
    echo "Step 1-5: Initial Setup (Login, Create Region Site, List Sites, Get Profile, Host NIO Registration)..." | tee -a "$log_file"
    local step_start
    step_start=$(start_timer)
    
    echo "  - Logging in..." | tee -a "$log_file"
    ./host-config-cli.sh login 2>&1 | tee -a "$log_file"
    
    echo "  - Creating region site..." | tee -a "$log_file"
    ./host-config-cli.sh create-region-site 2>&1 | tee -a "$log_file"
    
    echo "  - Listing region sites..." | tee -a "$log_file"
    ./host-config-cli.sh list-region-site 2>&1 | tee -a "$log_file"
    
    echo "  - Getting profile..." | tee -a "$log_file"
    ./host-config-cli.sh get-profile 2>&1 | tee -a "$log_file"
    
    echo "  - Host NIO registration for $vm_args..." | tee -a "$log_file"
    ./host-config-cli.sh host-nio-registration "$vm_args" 2>&1 | tee -a "$log_file"
    
    local step_time
    step_time=$(end_timer "$step_start")
    stage_names+=("Initial Setup")
    stage_times+=("$step_time")
    echo "Initial Setup completed in $step_time" | tee -a "$log_file"
    echo "" | tee -a "$log_file"
    
    echo "Step 6: Creating pico-ven for $vm_args..." | tee -a "$log_file"
    step_start=$(start_timer)
    ./pico-ven.sh create "$vm_args" 2>&1 | tee -a "$log_file"
    step_time=$(end_timer "$step_start")
    stage_names+=("Pico-VEN Creation")
    stage_times+=("$step_time")
    echo "Pico-ven creation completed in $step_time" | tee -a "$log_file"
    echo "" | tee -a "$log_file"
    
    echo "Step 7: Checking host status for $vm_args..." | tee -a "$log_file"
    step_start=$(start_timer)
    ./host-config-cli.sh host-status 2>&1 | tee -a "$log_file"
    step_time=$(end_timer "$step_start")
    stage_names+=("Host Status Check")
    stage_times+=("$step_time")
    echo "Host status check completed in $step_time" | tee -a "$log_file"
    echo "" | tee -a "$log_file"
    
    local total_time
    total_time=$(end_timer "$total_start")
    
    # Print summary
    echo "========================================" | tee -a "$log_file"
    echo "STAGE COMPLETION SUMMARY" | tee -a "$log_file"
    echo "========================================" | tee -a "$log_file"
    for i in "${!stage_names[@]}"; do
        printf "%-25s: %s\n" "${stage_names[$i]}" "${stage_times[$i]}" | tee -a "$log_file"
    done
    echo "----------------------------------------" | tee -a "$log_file"
    printf "%-25s: %s\n" "Total Time" "$total_time" | tee -a "$log_file"
    echo "========================================" | tee -a "$log_file"
    echo "Host onboarding completed for: $vm_args" | tee -a "$log_file"
    echo "All stages finished successfully!" | tee -a "$log_file"
    
    # Save to file
    local timestamp
    timestamp=$(date '+%Y%m%d_%H%M%S')
    local timing_log_file="./host_onboarding_times_${vm_args}_${timestamp}.log"
    {
        echo "Host Onboarding Stage Times - VM: $vm_args"
        echo "Date: $(date)"
        echo "========================================"
        for i in "${!stage_names[@]}"; do
            printf "%-25s: %s\n" "${stage_names[$i]}" "${stage_times[$i]}"
        done
        echo "----------------------------------------"
        printf "%-25s: %s\n" "Total Time" "$total_time"
    } > "$timing_log_file"
    echo "Stage times saved to: $timing_log_file" | tee -a "$log_file"
    echo "Complete logs saved to: $log_file" | tee -a "$log_file"
}

delete_host() {
    local vm_args="$1"
    local log_file="./ven-logs.log"
    local total_start
    total_start=$(start_timer)
    
    # Arrays to track stage names and times
    local stage_names=()
    local stage_times=()
    
    echo "Starting host deletion for: $vm_args" | tee -a "$log_file"
    echo "========================================" | tee -a "$log_file"
    
    echo "Step 1: Logging in..." | tee -a "$log_file"
    local step_start
    step_start=$(start_timer)
    ./host-config-cli.sh login 2>&1 | tee -a "$log_file"
    local step_time
    step_time=$(end_timer "$step_start")
    stage_names+=("Login")
    stage_times+=("$step_time")
    echo "Login completed in $step_time" | tee -a "$log_file"
    echo "" | tee -a "$log_file"
    
    echo "Step 2: Deleting hosts $vm_args..." | tee -a "$log_file"
    step_start=$(start_timer)
    ./host-config-cli.sh delete-hosts "$vm_args" 2>&1 | tee -a "$log_file"
    step_time=$(end_timer "$step_start")
    stage_names+=("Delete Hosts")
    stage_times+=("$step_time")
    echo "Host deletion completed in $step_time" | tee -a "$log_file"
    echo "" | tee -a "$log_file"
    
    echo "Step 3: Destroying pico-ven for $vm_args..." | tee -a "$log_file"
    step_start=$(start_timer)
    yes | ./pico-ven.sh destroy "$vm_args" 2>&1 | tee -a "$log_file"
    step_time=$(end_timer "$step_start")
    stage_names+=("Pico-VEN Destruction")
    stage_times+=("$step_time")
    echo "Pico-ven destruction completed in $step_time" | tee -a "$log_file"
    echo "" | tee -a "$log_file"
    
    local total_time
    total_time=$(end_timer "$total_start")
    
    # Print summary
    echo "========================================" | tee -a "$log_file"
    echo "STAGE COMPLETION SUMMARY" | tee -a "$log_file"
    echo "========================================" | tee -a "$log_file"
    for i in "${!stage_names[@]}"; do
        printf "%-25s: %s\n" "${stage_names[$i]}" "${stage_times[$i]}" | tee -a "$log_file"
    done
    echo "----------------------------------------" | tee -a "$log_file"
    printf "%-25s: %s\n" "Total Time" "$total_time" | tee -a "$log_file"
    echo "========================================" | tee -a "$log_file"
    echo "Host deletion completed for: $vm_args" | tee -a "$log_file"
    
    # Save to file
    local timestamp
    timestamp=$(date '+%Y%m%d_%H%M%S')
    local timing_log_file="./host_deletion_times_${vm_args}_${timestamp}.log"
    {
        echo "Host Deletion Stage Times - VM: $vm_args"
        echo "Date: $(date)"
        echo "========================================"
        for i in "${!stage_names[@]}"; do
            printf "%-25s: %s\n" "${stage_names[$i]}" "${stage_times[$i]}"
        done
        echo "----------------------------------------"
        printf "%-25s: %s\n" "Total Time" "$total_time"
    } > "$timing_log_file"
    echo "Stage times saved to: $timing_log_file" | tee -a "$log_file"
    echo "Complete logs saved to: $log_file" | tee -a "$log_file"
}

case "$1" in
    host-onboarding)
        init_logging "$@"
        host_onboarding "$2"
        ;;
    delete-host)
        init_logging "$@"
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

