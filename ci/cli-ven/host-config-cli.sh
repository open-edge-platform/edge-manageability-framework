#!/bin/bash
set -euo pipefail

# Error handling function
handle_error() {
    local exit_code=$?
    local line_number=$1
    echo "❌ Error occurred in script at line $line_number. Exit code: $exit_code" >&2
    exit $exit_code
}

# Set up error trap
trap 'handle_error $LINENO' ERR

# Source VEN environment variables
ENV_FILE="$(dirname "$0")/config.env"
if [ -f "$ENV_FILE" ]; then
    if ! source "$ENV_FILE"; then
        echo "❌ Failed to source environment file: $ENV_FILE" >&2
        exit 1
    fi
else
    echo "⚠️ Environment file not found: $ENV_FILE. Some variables may not be set." >&2
fi

# VEN prefix for range operations (can be customized)
VEN_PREFIX_SN="${VEN_PREFIX_SN:-VIRTUALEN}"

# Function to check if orch-cli is available
check_orch_cli() {
    if ! command -v orch-cli &> /dev/null; then
        echo "❌ orch-cli is not installed or not in PATH" >&2
        return 1
    fi
    return 0
}

# Function to validate required variables for login
validate_login_vars() {
    local required_vars=("CLUSTER_FQDN" "PROJECT_NAME" "ORCH_DEFAULT_USER" "ORCH_DEFAULT_PASSWORD")
    for var in "${required_vars[@]}"; do
        if [ -z "${!var:-}" ]; then
            echo "❌ Required variable $var is not set for login" >&2
            return 1
        fi
    done
    return 0
}

# Function to login to orchestrator
login_to_orch() {
    echo "=== Logging into Orchestrator ==="
    
    if ! check_orch_cli; then
        exit 1
    fi
    
    if ! validate_login_vars; then
        exit 1
    fi
    
    # Logout first (ignore errors if not logged in)
    orch-cli logout 2>/dev/null || true
    
    if ! orch-cli login "$ORCH_DEFAULT_USER" "$ORCH_DEFAULT_PASSWORD" --keycloak "https://keycloak.$CLUSTER_FQDN/realms/master"; then
        echo "❌ Failed to login to orchestrator" >&2
        exit 1
    fi
    
    if ! orch-cli config set project "$PROJECT_NAME"; then
        echo "❌ Failed to set project: $PROJECT_NAME" >&2
        exit 1
    fi
    
    if ! orch-cli config set api-endpoint "https://api.$CLUSTER_FQDN"; then
        echo "❌ Failed to set api-endpoint" >&2
        exit 1
    fi
    
    echo "Login completed successfully"
}

# Function to find overall latest (version priority, then date)
find_overall_latest() {
    echo "=== Overall Latest (Version Priority, then Date) ==="
    
    if ! check_orch_cli; then
        exit 1
    fi
    
    if ! orch-cli list osprofile | grep "Edge Microvisor Toolkit" | grep -v "Real Time"; then
        echo "❌ Failed to find Edge Microvisor Toolkit profiles" >&2
        exit 1
    fi
}

# Function to save variables to env file
save_to_env() {
    local var_name="$1"
    local var_value="$2"
    
    if [ -z "$var_name" ] || [ -z "$var_value" ]; then
        echo "❌ Variable name and value cannot be empty" >&2
        return 1
    fi
    
    # Create env file if it doesn't exist
    if ! touch "$ENV_FILE"; then
        echo "❌ Failed to create/access environment file: $ENV_FILE" >&2
        return 1
    fi
    
    # Remove existing entry and add new one
    if ! sed -i "/^$var_name=/d" "$ENV_FILE"; then
        echo "❌ Failed to update environment file" >&2
        return 1
    fi
    
    if ! echo "$var_name='$var_value'" >> "$ENV_FILE"; then
        echo "❌ Failed to write to environment file" >&2
        return 1
    fi
    
    echo "Saved $var_name='$var_value' to $ENV_FILE"
}

# Function to get profile (find latest toolkit)
get_profile() {
    echo "=== Getting Latest Profile ==="
    
    if ! check_orch_cli; then
        exit 1
    fi
    
    # Store the result in a variable
    local LATEST_EMT
    LATEST_EMT=$(find_overall_latest | \
    while IFS= read -r line; do
        # Skip empty lines or lines that don't start with "Edge Microvisor Toolkit"
        if [[ -z "$line" || ! "$line" =~ ^Edge\ Microvisor\ Toolkit ]]; then
            continue
        fi
        
        # Extract version (4th field)
        version=$(echo "$line" | awk '{print $4}')
        
        # Validate version format (should be X.Y.YYYYMMDD)
        if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            continue
        fi
        
        major=$(echo "$version" | cut -d'.' -f1)
        minor=$(echo "$version" | cut -d'.' -f2)
        date_part=$(echo "$version" | cut -d'.' -f3)
        
        # Validate that major and minor are numbers
        if [[ "$major" =~ ^[0-9]+$ ]] && [[ "$minor" =~ ^[0-9]+$ ]] && [[ "$date_part" =~ ^[0-9]+$ ]]; then
            printf "%05d %05d %s|%s\n" "$major" "$minor" "$date_part" "$line"
        fi
    done | \
    sort -k1,1nr -k2,2nr -k3,3nr | \
    head -1 | \
    cut -d'|' -f2)

    if [ -z "$LATEST_EMT" ]; then
        echo "❌ No valid Edge Microvisor Toolkit found" >&2
        exit 1
    fi

    # Extract just the toolkit name and version for easier use
    local LATEST_EMT_VERSION
    LATEST_EMT_VERSION=$(echo "$LATEST_EMT" | awk '{print $1, $2, $3, $4}')
    
    if [ -z "$LATEST_EMT_VERSION" ]; then
        echo "❌ Failed to extract toolkit version" >&2
        exit 1
    fi
    
    echo "Latest EMT Non RT Version: $LATEST_EMT_VERSION"
    
    # Save LATEST_EMT_VERSION to env file
    if ! save_to_env "LATEST_EMT_VERSION" "$LATEST_EMT_VERSION"; then
        exit 1
    fi
}

# Function to validate region/site names
validate_name() {
    local name="$1"
    local type="$2"
    
    if [ -z "$name" ]; then
        echo "❌ $type name cannot be empty" >&2
        return 1
    fi
    
    # Check for invalid characters
    if [[ ! "$name" =~ ^[a-zA-Z0-9_-]+$ ]]; then
        echo "❌ Invalid $type name: $name. Only alphanumeric, underscore, and dash allowed" >&2
        return 1
    fi
    
    return 0
}

# Function to create region and site
create_region_site() {
    # Set default values or use passed arguments
    local REGION_NAME=${2:-"Bangalore"}
    local SITE_NAME=${3:-"SRR3"}
    
    if ! validate_name "$REGION_NAME" "region"; then
        exit 1
    fi
    
    if ! validate_name "$SITE_NAME" "site"; then
        exit 1
    fi
    
    if ! check_orch_cli; then
        exit 1
    fi
    
    echo "=== Creating Region and Site ==="
    echo "Region: $REGION_NAME, Site: $SITE_NAME"
    
    # Check if region already exists
    local EXISTING_REGION REGION_ID
    if ! EXISTING_REGION=$(orch-cli list region 2>/dev/null | grep "$REGION_NAME" || true); then
        echo "❌ Failed to list regions" >&2
        exit 1
    fi
    
    if [ -n "$EXISTING_REGION" ]; then
        echo "Region $REGION_NAME already exists, skipping creation..."
        REGION_ID=$(echo "$EXISTING_REGION" | grep -o "region-[a-f0-9]*" | head -1)
    else
        # Create region
        echo "Creating region $REGION_NAME..."
        if ! orch-cli create region "$REGION_NAME" --type city; then
            echo "❌ Failed to create region: $REGION_NAME" >&2
            exit 1
        fi
        
        # List regions to find the region ID
        echo "Listing regions to find region ID..."
        if ! orch-cli list region; then
            echo "❌ Failed to list regions after creation" >&2
            exit 1
        fi
        
        # Extract region ID for the specified region (use first match if multiple exist)
        if ! REGION_ID=$(orch-cli list region | grep "$REGION_NAME" | grep -o "region-[a-f0-9]*" | head -1); then
            echo "❌ Failed to extract region ID" >&2
            exit 1
        fi
    fi
    
    if [ -z "$REGION_ID" ]; then
        echo "❌ Could not find region ID for $REGION_NAME" >&2
        exit 1
    fi
    
    echo "Found region ID: $REGION_ID"
    
    # Check if site already exists
    local EXISTING_SITE SITE_ID
    if ! EXISTING_SITE=$(orch-cli list site 2>/dev/null | grep "$SITE_NAME" || true); then
        echo "❌ Failed to list sites" >&2
        exit 1
    fi
    
    if [ -n "$EXISTING_SITE" ]; then
        echo "Site $SITE_NAME already exists, skipping creation..."
        SITE_ID=$(echo "$EXISTING_SITE" | awk '{print $1}')
    else
        # Create site using the found region ID
        echo "Creating site $SITE_NAME in region $REGION_ID..."
        if ! orch-cli create site "$SITE_NAME" --region "$REGION_ID"; then
            echo "❌ Failed to create site: $SITE_NAME" >&2
            exit 1
        fi
        
        # Get the site ID after creation
        if ! SITE_ID=$(orch-cli list site | grep "$SITE_NAME" | awk '{print $1}'); then
            echo "❌ Failed to get site ID after creation" >&2
            exit 1
        fi
    fi
    
    if [ -z "$SITE_ID" ]; then
        echo "❌ Could not find site ID for $SITE_NAME" >&2
        exit 1
    fi
    
    # List sites to confirm creation
    echo "Listing sites..."
    if ! orch-cli list site; then
        echo "⚠️ Failed to list sites for confirmation, but continuing..." >&2
    fi
    
    # Save variables to env file
    if ! save_to_env "REGION_ID" "$REGION_ID"; then
        exit 1
    fi
    
    if ! save_to_env "SITE_ID" "$SITE_ID"; then
        exit 1
    fi
    
    echo "Region and site creation completed successfully"
}

# Function to list regions and sites
list_region_site() {
    echo "=== Listing Regions and Sites ==="
    
    echo "Listing all regions:"
    orch-cli list region
    
    echo ""
    echo "Listing all sites:"
    orch-cli list site
}

# Function to show host status
host_status() {
    echo "=== Host Status ==="
    
    if ! check_orch_cli; then
        exit 1
    fi
    
    # If no arguments provided, wait for all hosts to be running
    if [ $# -lt 2 ]; then
        echo "Monitoring all hosts until they reach running state..."
        local max_wait=1200  # 20 minutes
        local wait_interval=10
        local elapsed=0
        
        while [ $elapsed -lt $max_wait ]; do
            echo "=== Checking host status (elapsed: ${elapsed}s) ==="
            
            # Get all hosts and their status
            local all_hosts_output
            if ! all_hosts_output=$(orch-cli list host 2>/dev/null); then
                echo "❌ Failed to list hosts" >&2
                exit 1
            fi
            
            echo "$all_hosts_output"
            
            # Check if there are any hosts at all (exclude header)
            local host_count=$(echo "$all_hosts_output" | tail -n +2 | awk 'NF >= 8 && /^host-/ {print}' | wc -l)
            if [ "$host_count" -eq 0 ]; then
                echo "⚠️ No hosts found. Waiting for hosts to be created..."
                echo "Waiting ${wait_interval} seconds before next check..."
                sleep $wait_interval
                elapsed=$((elapsed + wait_interval))
                continue
            fi
            
            # Count non-running hosts - look for actual host entries that don't have "Running" in column 3
            local non_running_count
            non_running_count=$(echo "$all_hosts_output" | tail -n +2 | awk 'NF >= 8 && /^host-/ && $3 != "Running" {print}' | wc -l)
            
            # Debug: Show all host entries and their status
            echo "Debug: Host entries found:"
            #echo "$all_hosts_output" | tail -n +2 | awk 'NF >= 8 && /^host-/ {print "  Host: " $1 " Status: " $3}'
            
            if [ "$non_running_count" -eq 0 ]; then
                echo "✅ All hosts are in running state!"
                return 0
            fi
            
            echo "⏳ Waiting for hosts to reach running state. ${non_running_count} hosts are not yet running."
            echo "Waiting ${wait_interval} seconds before next check..."
            sleep $wait_interval
            elapsed=$((elapsed + wait_interval))
        done
        
        echo "⚠️ Timeout reached (${max_wait}s). Some hosts may still not be running."
        return 1
    fi
    
    # Arguments provided - show specific host(s) status and wait for them to be running
    local target_hosts=()
    
    if [[ "${2}" == *-* ]]; then
        # Range format: START-END (for VEN with configurable prefix)
        local RANGE="${2}"
        local START END
        START=$(echo "$RANGE" | cut -d'-' -f1)
        END=$(echo "$RANGE" | cut -d'-' -f2)
        
        # Validate range
        if ! [[ "$START" =~ ^[0-9]+$ ]] || ! [[ "$END" =~ ^[0-9]+$ ]]; then
            echo "❌ Invalid range format. START and END must be numeric" >&2
            exit 1
        fi
        
        if [ "$START" -gt "$END" ]; then
            echo "❌ Invalid range: START ($START) cannot be greater than END ($END)" >&2
            exit 1
        fi
        
        # Build array of target hosts
        for ((i=START; i<=END; i++)); do
            target_hosts+=("$VEN_PREFIX_SN$i")
        done
    else
        # Multiple individual serial numbers provided as arguments
        for i in $(seq 2 $#); do
            local EN_SN=${!i}
            if [ -z "$EN_SN" ]; then
                echo "❌ Serial number cannot be empty" >&2
                exit 1
            fi
            target_hosts+=("$EN_SN")
        done
    fi
    
    echo "Monitoring hosts until they reach running state: ${target_hosts[*]}"
    local max_wait=1800  # 30 minutes
    local wait_interval=30
    local elapsed=0
    
    while [ $elapsed -lt $max_wait ]; do
        echo "=== Checking host status for specified hosts (elapsed: ${elapsed}s) ==="
        
        local all_running=true
        local hosts_status=()
        
        for host_serial in "${target_hosts[@]}"; do
            local HOST_ID
            if HOST_ID=$(orch-cli list host | grep "$host_serial" | awk '{print $1}' | head -1); then
                if [ -n "$HOST_ID" ]; then
                    # Get the host status from list command instead of get command for better performance
                    local host_line
                    host_line=$(orch-cli list host | grep "$host_serial")
                    local host_status=$(echo "$host_line" | awk '{print $3}')
                    
                    echo "Host $host_serial (ID: $HOST_ID): Status=$host_status"
                    
                    # Check if host status is "Running"
                    if [ "$host_status" != "Running" ]; then
                        all_running=false
                        hosts_status+=("$host_serial: $host_status")
                    else
                        hosts_status+=("$host_serial: RUNNING")
                    fi
                else
                    echo "⚠️ Could not find host ID for serial $host_serial" >&2
                    all_running=false
                    hosts_status+=("$host_serial: NOT_FOUND")
                fi
            else
                echo "⚠️ Could not find host for serial $host_serial" >&2
                all_running=false
                hosts_status+=("$host_serial: NOT_FOUND")
            fi
        done
        
        if $all_running; then
            echo "✅ All specified hosts are in running state!"
            echo "Final status:"
            printf '%s\n' "${hosts_status[@]}"
            return 0
        fi
        
        echo "⏳ Waiting for hosts to reach running state..."
        printf '%s\n' "${hosts_status[@]}"
        echo "Waiting ${wait_interval} seconds before next check..."
        sleep $wait_interval
        elapsed=$((elapsed + wait_interval))
    done
    
    echo "⚠️ Timeout reached (${max_wait}s). Some hosts may still not be running."
    printf '%s\n' "${hosts_status[@]}"
    return 1
}

# Function for host NIO registration
host_nio_registration() {
    echo "=== Host NIO Registration ==="
    
    if ! check_orch_cli; then
        exit 1
    fi
    
    # Check if required environment variables are set
    if [ -z "${LATEST_EMT_VERSION:-}" ] || [ -z "${SITE_ID:-}" ]; then
        echo "❌ Error: LATEST_EMT_VERSION or SITE_ID not found in environment" >&2
        echo "Please run 'get-profile' and 'create-region-site' first" >&2
        exit 1
    fi
    
    # Create host-config.csv file
    local HOST_CONFIG_FILE="host-config.csv"
    
    echo "Creating $HOST_CONFIG_FILE..."
    # Write CSV header
    if ! cat > "$HOST_CONFIG_FILE" << 'EOF'; then
Serial,UUID,OSProfile,Site,Secure,RemoteUser,Metadata,LVMSize,CloudInitMeta,K8sEnable,K8sClusterTemplate,K8sConfig,Error - do not fill
EOF
        echo "❌ Failed to create host config file" >&2
        exit 1
    fi
    
    # Handle different input formats
    if [ $# -gt 1 ]; then
        # Check if it's a range (contains -)
        if [[ "${2}" == *-* ]]; then
            # Range format: START-END (for VEN only with configurable prefix)
            local RANGE="${2}"
            local START END
            START=$(echo "$RANGE" | cut -d'-' -f1)
            END=$(echo "$RANGE" | cut -d'-' -f2)
            
            # Validate range
            if ! [[ "$START" =~ ^[0-9]+$ ]] || ! [[ "$END" =~ ^[0-9]+$ ]]; then
                echo "❌ Invalid range format. START and END must be numeric" >&2
                exit 1
            fi
            
            if [ "$START" -gt "$END" ]; then
                echo "❌ Invalid range: START ($START) cannot be greater than END ($END)" >&2
                exit 1
            fi
            
            echo "Adding VEN serial numbers from range: $VEN_PREFIX_SN$START to $VEN_PREFIX_SN$END"
            for ((i=START; i<=END; i++)); do
                local VEN_SERIAL="$VEN_PREFIX_SN$i"
                if ! echo "$VEN_SERIAL,,$LATEST_EMT_VERSION,$SITE_ID,FALSE,,,,,,,," >> "$HOST_CONFIG_FILE"; then
                    echo "❌ Failed to write to host config file" >&2
                    exit 1
                fi
                echo "Added VEN: $VEN_SERIAL"
            done
        else
            # Multiple individual serial numbers (strings or numbers) provided as arguments
            echo "Adding multiple ENs to host config..."
            for i in $(seq 2 $#); do
                local EN_SN=${!i}
                if [ -z "$EN_SN" ]; then
                    echo "❌ Serial number cannot be empty" >&2
                    exit 1
                fi
                if ! echo "$EN_SN,,$LATEST_EMT_VERSION,$SITE_ID,FALSE,,,,,,,," >> "$HOST_CONFIG_FILE"; then
                    echo "❌ Failed to write to host config file" >&2
                    exit 1
                fi
                echo "Added EN: $EN_SN"
            done
        fi
    else
        # Single EN or default (can be string or number)
        local EN_SN1=${2:-"DEFAULT_SERIAL"}
        if ! echo "$EN_SN1,,$LATEST_EMT_VERSION,$SITE_ID,FALSE,,,,,,,," >> "$HOST_CONFIG_FILE"; then
            echo "❌ Failed to write to host config file" >&2
            exit 1
        fi
        echo "Added single EN: $EN_SN1"
    fi
    
    echo "Host config file created:"
    if ! cat "$HOST_CONFIG_FILE"; then
        echo "❌ Failed to display host config file" >&2
        exit 1
    fi
    
    # Create host using the CSV file
    echo "Creating host(s) using NIO registration..."
    if ! orch-cli create host -i "$HOST_CONFIG_FILE"; then
        echo "❌ Failed to create hosts" >&2
        exit 1
    fi
    
    # List hosts
    echo "Listing all hosts..."
    if ! orch-cli list host; then
        echo "⚠️ Failed to list hosts, but continuing..." >&2
    fi
    
    # Get details for created hosts and save the first host ID
    if [ $# -gt 1 ]; then
        # Multiple hosts - get all host IDs and show status for each
        if [[ "${2}" == *-* ]]; then
            # Range format - use configurable prefix
            RANGE="${2}"
            START=$(echo "$RANGE" | cut -d'-' -f1)
            END=$(echo "$RANGE" | cut -d'-' -f2)
            
            echo "Getting status for all created VEN hosts..."
            for ((i=START; i<=END; i++)); do
                VEN_SERIAL="$VEN_PREFIX_SN$i"
                HOST_ID=$(orch-cli list host | grep "$VEN_SERIAL" | awk '{print $1}' | head -1)
                if [ -n "$HOST_ID" ]; then
                    echo "Getting details for host $VEN_SERIAL: $HOST_ID"
                    orch-cli get host "$HOST_ID"
                    if [ $i -eq $START ]; then
                        save_to_env "HOST_ID" "$HOST_ID"
                    fi
                else
                    echo "Warning: Could not find host ID for serial $VEN_SERIAL"
                fi
            done
        else
            # List format - get status for each provided serial
            echo "Getting status for all created hosts..."
            for i in $(seq 2 $#); do
                EN_SN=${!i}
                HOST_ID=$(orch-cli list host | grep "$EN_SN" | awk '{print $1}' | head -1)
                if [ -n "$HOST_ID" ]; then
                    echo "Getting details for host $EN_SN: $HOST_ID"
                    orch-cli get host "$HOST_ID"
                    if [ $i -eq 2 ]; then
                        save_to_env "HOST_ID" "$HOST_ID"
                    fi
                else
                    echo "Warning: Could not find host ID for serial $EN_SN"
                fi
            done
        fi
    else
        # Single host
        EN_SN1=${2:-"DEFAULT_SERIAL"}
        HOST_ID=$(orch-cli list host | grep "$EN_SN1" | awk '{print $1}' | head -1)
        if [ -n "$HOST_ID" ]; then
            echo "Getting details for host: $HOST_ID"
            orch-cli get host "$HOST_ID"
            save_to_env "HOST_ID" "$HOST_ID"
        else
            echo "Warning: Could not find host ID for serial $EN_SN1"
        fi
    fi
    
    echo "Host NIO registration completed successfully"
}

# Function to delete hosts
delete_hosts() {
    echo "=== Deleting Hosts ==="
    
    if ! check_orch_cli; then
        exit 1
    fi
    
    if [ $# -lt 2 ]; then
        echo "❌ Error: No serial numbers provided for deletion" >&2
        echo "Usage: delete-hosts [serial_numbers]" >&2
        return 1
    fi
    
    # List current hosts first
    echo "Current hosts:"
    if ! orch-cli list host; then
        echo "❌ Failed to list current hosts" >&2
        exit 1
    fi
    echo ""
    
    local failed_deletions=()
    
    # Handle different input formats
    if [[ "${2}" == *-* ]]; then
        # Range format: START-END (for VEN with configurable prefix)
        local RANGE="${2}"
        local START END
        START=$(echo "$RANGE" | cut -d'-' -f1)
        END=$(echo "$RANGE" | cut -d'-' -f2)
        
        # Validate range
        if ! [[ "$START" =~ ^[0-9]+$ ]] || ! [[ "$END" =~ ^[0-9]+$ ]]; then
            echo "❌ Invalid range format. START and END must be numeric" >&2
            exit 1
        fi
        
        echo "Deleting VEN hosts from range: $VEN_PREFIX_SN$START to $VEN_PREFIX_SN$END"
        for ((i=START; i<=END; i++)); do
            local VEN_SERIAL="$VEN_PREFIX_SN$i"
            local HOST_ID
            if HOST_ID=$(orch-cli list host | grep "$VEN_SERIAL" | awk '{print $1}' | head -1); then
                if [ -n "$HOST_ID" ]; then
                    echo "Deleting host $VEN_SERIAL with ID: $HOST_ID"
                    if ! orch-cli delete host "$HOST_ID"; then
                        echo "❌ Failed to delete host $VEN_SERIAL" >&2
                        failed_deletions+=("$VEN_SERIAL")
                    fi
                else
                    echo "⚠️ Could not find host ID for serial $VEN_SERIAL" >&2
                fi
            else
                echo "⚠️ Could not find host for serial $VEN_SERIAL" >&2
            fi
        done
    else
        # Single or multiple individual serial numbers
        echo "Deleting hosts with provided serial numbers..."
        for i in $(seq 2 $#); do
            local EN_SN=${!i}
            local HOST_ID
            if HOST_ID=$(orch-cli list host | grep "$EN_SN" | awk '{print $1}' | head -1); then
                if [ -n "$HOST_ID" ]; then
                    echo "Deleting host $EN_SN with ID: $HOST_ID"
                    if ! orch-cli delete host "$HOST_ID"; then
                        echo "❌ Failed to delete host $EN_SN" >&2
                        failed_deletions+=("$EN_SN")
                    fi
                else
                    echo "⚠️ Could not find host ID for serial $EN_SN" >&2
                fi
            else
                echo "⚠️ Could not find host for serial $EN_SN" >&2
            fi
        done
    fi
    
    # List hosts after deletion to confirm
    echo ""
    echo "Hosts after deletion:"
    if ! orch-cli list host; then
        echo "⚠️ Failed to list hosts after deletion" >&2
    fi
    
    if [ ${#failed_deletions[@]} -eq 0 ]; then
        echo "Host deletion completed successfully"
    else
        echo "⚠️ Some hosts failed to delete: ${failed_deletions[*]}" >&2
        exit 1
    fi
}

# Function to create cluster(s)
create_cluster() {
    echo "=== Creating Cluster(s) ==="
    
    if ! check_orch_cli; then
        exit 1
    fi
    
    if [ $# -lt 2 ]; then
        echo "❌ Error: Cluster serial number required" >&2
        echo "Usage: create-cluster [cluster_serial] OR create-cluster [start-end]" >&2
        return 1
    fi
    
    local failed_creations=()
    local created_clusters=()
    
    # Check if this is a range operation (contains -)
    if [[ "${2}" == *-* ]]; then
        # Range format: start-end (uses VEN_PREFIX_SN)
        local RANGE="${2}"
        local START END
        START=$(echo "$RANGE" | cut -d'-' -f1)
        END=$(echo "$RANGE" | cut -d'-' -f2)
        
        # Validate range
        if ! [[ "$START" =~ ^[0-9]+$ ]] || ! [[ "$END" =~ ^[0-9]+$ ]]; then
            echo "❌ Invalid range format. START and END must be numeric" >&2
            exit 1
        fi
        
        if [ "$START" -gt "$END" ]; then
            echo "❌ Invalid range: START ($START) cannot be greater than END ($END)" >&2
            exit 1
        fi
        
        echo "Creating clusters from range: ${START} to ${END} (using VEN_PREFIX_SN: $VEN_PREFIX_SN)"
        for ((i=START; i<=END; i++)); do
            local VEN_SERIAL="$VEN_PREFIX_SN$i"  # Use prefix for range operations
            local CLUSTER_NAME="$VEN_SERIAL"
            echo "Creating cluster: $CLUSTER_NAME for VEN: $VEN_SERIAL"
            
            # Get VEN Host ID and UUID
            local VEN_HOST_ID
            if ! VEN_HOST_ID=$(orch-cli list host | grep "$VEN_SERIAL" | awk '{print $1}' | head -1) || [ -z "$VEN_HOST_ID" ]; then
                echo "❌ No host found for VEN serial: $VEN_SERIAL" >&2
                failed_creations+=("$CLUSTER_NAME")
                continue
            fi
            
            local VEN_UUID
            if ! VEN_UUID=$(orch-cli get host "$VEN_HOST_ID" | grep -E "^\s*-\s*UUID:" | awk '{print $3}') || [ -z "$VEN_UUID" ]; then
                echo "❌ No UUID found for host: $VEN_HOST_ID" >&2
                exit 1
            fi
            
            echo "Found VEN Host ID: $VEN_HOST_ID, UUID: $VEN_UUID"
            
            # Check if cluster already exists
            if orch-cli list cluster 2>/dev/null | grep -q "$CLUSTER_NAME"; then
                echo "Cluster $CLUSTER_NAME already exists, skipping creation..."
                local CLUSTER_ID=$(orch-cli list cluster | grep "$CLUSTER_NAME" | awk '{print $1}')
                created_clusters+=("$CLUSTER_NAME:$CLUSTER_ID")
            else
                # Create cluster using UUID
                if orch-cli create cluster "$CLUSTER_NAME" --nodes "${VEN_UUID}:all"; then
                    echo "✅ Successfully created cluster: $CLUSTER_NAME"
                    local CLUSTER_ID=$(orch-cli list cluster | grep "$CLUSTER_NAME" | awk '{print $1}' | head -1)
                    created_clusters+=("$CLUSTER_NAME:$CLUSTER_ID")
                else
                    echo "❌ Failed to create cluster: $CLUSTER_NAME" >&2
                    failed_creations+=("$CLUSTER_NAME")
                fi
            fi
        done
    else
        # Single or multiple individual serial numbers (use as-is, no prefix)
        echo "Creating clusters for individual serial numbers (no prefix):"
        for i in $(seq 2 $#); do
            local SERIAL_ARG=${!i}
            local VEN_SERIAL="$SERIAL_ARG"  # Use serial as-is, no prefix
            local CLUSTER_NAME="$SERIAL_ARG"
            
            echo "Creating cluster: $CLUSTER_NAME for VEN: $VEN_SERIAL"
            
            # Get VEN Host ID and UUID
            local VEN_HOST_ID
            if ! VEN_HOST_ID=$(orch-cli list host | grep "$VEN_SERIAL" | awk '{print $1}' | head -1) || [ -z "$VEN_HOST_ID" ]; then
                echo "❌ No host found for VEN serial: $VEN_SERIAL" >&2
                failed_creations+=("$CLUSTER_NAME")
                continue
            fi
            
            local VEN_UUID
            if ! VEN_UUID=$(orch-cli get host "$VEN_HOST_ID" | grep -E "^\s*-\s*UUID:" | awk '{print $3}') || [ -z "$VEN_UUID" ]; then
                echo "❌ No UUID found for host: $VEN_HOST_ID" >&2
                exit 1
            fi
            
            echo "Found VEN Host ID: $VEN_HOST_ID, UUID: $VEN_UUID"
            
            # Check if cluster already exists
            if orch-cli list cluster 2>/dev/null | grep -q "$CLUSTER_NAME"; then
                echo "Cluster $CLUSTER_NAME already exists, skipping creation..."
                local CLUSTER_ID=$(orch-cli list cluster | grep "$CLUSTER_NAME" | awk '{print $1}')
                created_clusters+=("$CLUSTER_NAME:$CLUSTER_ID")
            else
                # Create cluster using UUID
                if orch-cli create cluster "$CLUSTER_NAME" --nodes "${VEN_UUID}:all"; then
                    echo "✅ Successfully created cluster: $CLUSTER_NAME"
                    local CLUSTER_ID=$(orch-cli list cluster | grep "$CLUSTER_NAME" | awk '{print $1}' | head -1)
                    created_clusters+=("$CLUSTER_NAME:$CLUSTER_ID")
                    
                    # Save the first cluster ID to env file
                    if [ $i -eq 2 ] && ! save_to_env "CLUSTER_ID" "$CLUSTER_ID"; then
                        echo "⚠️ Failed to save cluster ID to environment file" >&2
                    fi
                else
                    echo "❌ Failed to create cluster: $CLUSTER_NAME" >&2
                    failed_creations+=("$CLUSTER_NAME")
                fi
            fi
        done
    fi
    
    # List all clusters after creation
    echo ""
    echo "Listing all clusters:"
    orch-cli list cluster || echo "⚠️ Failed to list clusters after creation"
    
    # Summary
    echo ""
    echo "=== Creation Summary ==="
    if [ ${#created_clusters[@]} -gt 0 ]; then
        echo "✅ Successfully created/found clusters:"
        for cluster_info in "${created_clusters[@]}"; do
            echo "  ${cluster_info}"
        done
    fi
    
    if [ ${#failed_creations[@]} -gt 0 ]; then
        echo "❌ Failed to create clusters:"
        printf '  %s\n' "${failed_creations[@]}"
        exit 1
    fi
    
    echo "Cluster creation completed successfully"
}

# Function to delete cluster(s)
delete_cluster() {
    echo "=== Deleting Cluster(s) ==="
    
    if ! check_orch_cli; then
        exit 1
    fi
    
    if [ $# -lt 2 ]; then
        echo "❌ Error: No cluster names provided for deletion" >&2
        echo "Usage: delete-cluster [cluster_names] OR delete-cluster [start-end]" >&2
        return 1
    fi
    
    # List current clusters first
    echo "Current clusters:"
    orch-cli list cluster || echo "❌ Failed to list current clusters"
    echo ""
    
    local failed_deletions=()
    local successful_deletions=()
    
    # Check if this is a range operation (contains -)
    if [[ "${2}" == *-* ]]; then
        # Range format: start-end (uses VEN_PREFIX_SN)
        local RANGE="${2}"
        local START END
        START=$(echo "$RANGE" | cut -d'-' -f1)
        END=$(echo "$RANGE" | cut -d'-' -f2)
        
        # Validate range
        if ! [[ "$START" =~ ^[0-9]+$ ]] || ! [[ "$END" =~ ^[0-9]+$ ]]; then
            echo "❌ Invalid range format. START and END must be numeric" >&2
            exit 1
        fi
        
        if [ "$START" -gt "$END" ]; then
            echo "❌ Invalid range: START ($START) cannot be greater than END ($END)" >&2
            exit 1
        fi
        
        echo "Deleting clusters from range: ${START} to ${END} (using VEN_PREFIX_SN: $VEN_PREFIX_SN)"
        for ((i=START; i<=END; i++)); do
            local CLUSTER_NAME="$VEN_PREFIX_SN$i"  # Use prefix for range operations
            local CLUSTER_ID
            if CLUSTER_ID=$(orch-cli list cluster | grep "$CLUSTER_NAME" | awk '{print $1}' | head -1) && [ -n "$CLUSTER_ID" ]; then
                echo "Deleting cluster $CLUSTER_NAME with ID: $CLUSTER_ID"
                if orch-cli delete cluster "$CLUSTER_NAME"; then
                    echo "✅ Successfully deleted cluster: $CLUSTER_NAME"
                    successful_deletions+=("$CLUSTER_NAME")
                else
                    echo "❌ Failed to delete cluster $CLUSTER_NAME" >&2
                    failed_deletions+=("$CLUSTER_NAME")
                fi
            else
                echo "⚠️ Could not find cluster $CLUSTER_NAME" >&2
            fi
        done
    else
        # Single or multiple individual cluster names (use as-is, no prefix)
        echo "Deleting clusters with provided names (no prefix):"
        for i in $(seq 2 $#); do
            local CLUSTER_NAME=${!i}  # Use name as-is, no prefix
            if [ -z "$CLUSTER_NAME" ]; then
                echo "❌ Cluster name cannot be empty" >&2
                exit 1
            fi
            
            local CLUSTER_ID
            if CLUSTER_ID=$(orch-cli list cluster | grep "$CLUSTER_NAME" | awk '{print $1}' | head -1) && [ -n "$CLUSTER_ID" ]; then
                echo "Deleting cluster $CLUSTER_NAME with ID: $CLUSTER_ID"
                if orch-cli delete cluster "$CLUSTER_NAME"; then
                    echo "✅ Successfully deleted cluster: $CLUSTER_NAME"
                    successful_deletions+=("$CLUSTER_NAME")
                else
                    echo "❌ Failed to delete cluster $CLUSTER_NAME" >&2
                    failed_deletions+=("$CLUSTER_NAME")
                fi
            else
                echo "⚠️ Could not find cluster $CLUSTER_NAME" >&2
            fi
        done
    fi
    
    # List clusters after deletion to confirm
    echo ""
    echo "Clusters after deletion:"
    orch-cli list cluster || echo "⚠️ Failed to list clusters after deletion"
    
    # Summary
    echo ""
    echo "=== Deletion Summary ==="
    if [ ${#successful_deletions[@]} -gt 0 ]; then
        echo "✅ Successfully deleted clusters:"
        printf '  %s\n' "${successful_deletions[@]}"
    fi
    
    if [ ${#failed_deletions[@]} -eq 0 ]; then
        echo "Cluster deletion completed successfully"
    else
        echo "❌ Some clusters failed to delete:"
        printf '  %s\n' "${failed_deletions[@]}"
        exit 1
    fi
}

# Function to show cluster status
cluster_status() {
    echo "=== Cluster Status ==="
    
    if ! check_orch_cli; then
        exit 1
    fi
    
    # If no arguments provided, just list all clusters
    if [ $# -lt 2 ]; then
        echo "Listing all clusters:"
        if ! orch-cli list cluster; then
            echo "❌ Failed to list clusters" >&2
            exit 1
        fi
        return 0
    fi
    
    # Arguments provided - show specific cluster(s) status and wait for them to be active
    local target_clusters=()
    
    if [[ "${2}" == *-* ]]; then
        # Range format: START-END (use VEN_PREFIX_SN for cluster names)
        local RANGE="${2}"
        local START END
        START=$(echo "$RANGE" | cut -d'-' -f1)
        END=$(echo "$RANGE" | cut -d'-' -f2)
        
        # Validate range
        if ! [[ "$START" =~ ^[0-9]+$ ]] || ! [[ "$END" =~ ^[0-9]+$ ]]; then
            echo "❌ Invalid range format. START and END must be numeric" >&2
            exit 1
        fi
        
        if [ "$START" -gt "$END" ]; then
            echo "❌ Invalid range: START ($START) cannot be greater than END ($END)" >&2
            exit 1
        fi
        
        # Build array of target clusters using VEN_PREFIX_SN
        for ((i=START; i<=END; i++)); do
            target_clusters+=("$VEN_PREFIX_SN$i")
        done
        
        echo "Monitoring clusters from range ${VEN_PREFIX_SN}${START} to ${VEN_PREFIX_SN}${END} until they reach active state"
    else
        # Multiple individual cluster names provided as arguments (use as-is, no prefix)
        for i in $(seq 2 $#); do
            local CLUSTER_NAME=${!i}  # Use name as-is, no prefix
            if [ -z "$CLUSTER_NAME" ]; then
                echo "❌ Cluster name cannot be empty" >&2
                exit 1
            fi
            target_clusters+=("$CLUSTER_NAME")
        done
        
        echo "Monitoring individual clusters until they reach active state"
    fi
    
    echo "Target clusters: ${target_clusters[*]}"
    
    local max_wait=1200  # 20 minutes
    local wait_interval=10
    local elapsed=0
    
    while [ $elapsed -lt $max_wait ]; do
        echo "=== Checking cluster status for specified clusters (elapsed: ${elapsed}s) ==="
        
        local all_active=true
        local clusters_status=()
        
        for cluster_name in "${target_clusters[@]}"; do
            # Simple check: orch-cli list cluster | grep clustername | grep active
            if orch-cli list cluster | grep "$cluster_name" | grep -q "active"; then
                echo "Cluster $cluster_name: Status=active"
                clusters_status+=("$cluster_name: ACTIVE")
            else
                echo "Cluster $cluster_name: Status=not_active"
                clusters_status+=("$cluster_name: NOT_ACTIVE")
                all_active=false
            fi
        done
        
        if $all_active; then
            echo "✅ All specified clusters are in active state!"
            echo "Final status:"
            printf '%s\n' "${clusters_status[@]}"
            return 0
        fi
        
        echo "⏳ Waiting for clusters to reach active state..."
        printf '%s\n' "${clusters_status[@]}"
        echo "Waiting ${wait_interval} seconds before next check..."
        sleep $wait_interval
        elapsed=$((elapsed + wait_interval))
    done
    
    echo "⚠️ Timeout reached (${max_wait}s). Some clusters may still not be active."
    printf '%s\n' "${clusters_status[@]}"
    return 1
}

# Main script logic - handle arguments
case "$1" in
    "login")
        login_to_orch
        ;;
    "get-profile")
        get_profile
        ;;
    "create-region-site")
        create_region_site "$@"
        ;;
    "list-region-site")
        list_region_site
        ;;
    "host-status")
        host_status "$@"
        ;;
    "host-nio-registration")
        host_nio_registration "$@"
        ;;
    "delete-hosts")
        delete_hosts "$@"
        ;;
    "create-cluster")
        create_cluster "$@"
        ;;
    "delete-cluster")
        delete_cluster "$@"
        ;;
    "cluster-status")
        cluster_status "$@"
        ;;
    *)
        echo "Usage: $0 {login|get-profile|create-region-site [region_name] [site_name]|list-region-site|host-status [serial_numbers]|host-nio-registration [serial_numbers]|delete-hosts [serial_numbers]|create-cluster [cluster_serial]|delete-cluster [cluster_names]|cluster-status [cluster_names]}"
        echo "  login                                    - Login to orchestrator"
        echo "  get-profile                              - Get latest toolkit profile"
        echo "  create-region-site [region_name] [site_name] - Create region and site (defaults: Bangalore, SRR3)"
        echo "  list-region-site                         - List all regions and sites"
        echo "  host-status [serials]                    - Monitor host status until running (all hosts if no args, specific hosts if args provided)"
        echo "    Examples: host-status                                  (monitor all hosts until running)"
        echo "             host-status ABC123 DEF456 GHI789              (monitor specific hosts until running)"
        echo "             host-status 100-105                           (monitor range ${VEN_PREFIX_SN}100 to ${VEN_PREFIX_SN}105 until running)"
        echo "  host-nio-registration [serials]          - Create host config and perform NIO registration"
        echo "    Examples: host-nio-registration ABC123 DEF456 GHI789  (list of strings)"
        echo "             host-nio-registration 123 456 789              (list of numbers)"
        echo "             host-nio-registration 100-105                  (range - creates ${VEN_PREFIX_SN}100 to ${VEN_PREFIX_SN}105)"
        echo "             host-nio-registration ABC123                   (single string)"
        echo "  delete-hosts [serials]                   - Delete hosts by serial numbers"
        echo "    Examples: delete-hosts ABC123 DEF456 GHI789  (list of strings)"
        echo "             delete-hosts 123 456 789              (list of numbers)"
        echo "             delete-hosts 100-105                  (range - deletes ${VEN_PREFIX_SN}100 to ${VEN_PREFIX_SN}105)"
        echo "             delete-hosts ABC123                   (single string)"
        echo "  create-cluster [cluster_serial]          - Create single cluster or range of clusters"
        echo "    Examples: create-cluster 001"
        echo "             create-cluster 1-3"
        echo "  delete-cluster [cluster_names]           - Delete clusters by name"
        echo "    Examples: delete-cluster 001"
        echo "             delete-cluster 1 2 3"
        echo "             delete-cluster cluster 1-3"
        echo "  cluster-status [cluster_names]           - Monitor cluster status until active (all clusters if no args, specific clusters if args provided)"
        echo "    Examples: cluster-status                                (monitor all clusters until active)"
        echo "             cluster-status suniltest01 mytest02            (monitor specific clusters until active)"
        echo "             cluster-status 1-3                             (monitor range 1 to 3 until active)"
        echo ""
        echo "Environment Variables:"
        echo "  VEN_PREFIX_SN - Prefix for VEN range operations (default: VIRTUALEN)"
        echo "               Usage: VEN_PREFIX_SN=MYPREFIX $0 host-nio-registration 1-3"
        exit 1
        ;;
esac
