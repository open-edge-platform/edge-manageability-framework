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

# Source environment variables
ENV_FILE="$(dirname "$0")/config.env"
if [ -f "$ENV_FILE" ]; then
    echo "Sourcing environment file: $ENV_FILE"
    if ! source "$ENV_FILE"; then
        echo "❌ Failed to source environment file: $ENV_FILE" >&2
        exit 1
    fi
else
    echo "❌ Environment file not found: $ENV_FILE" >&2
    exit 1
fi

# Validate required environment variables
required_vars=(
    "CLUSTER_FQDN"
    "VEN_PICO_PATH"
    "VEN_DISK_SIZE"
    "VEN_MEMORY"
    "VEN_CPU_CORES"
)

for var in "${required_vars[@]}"; do
    if [ -z "${!var:-}" ]; then
        echo "❌ Required environment variable $var is not set" >&2
        exit 1
    fi
done

# Validate VEN_PICO_PATH exists
if [ ! -d "$VEN_PICO_PATH" ]; then
    echo "❌ VEN_PICO_PATH directory does not exist: $VEN_PICO_PATH" >&2
    exit 1
fi

# Apply defaults for variables that might not be set
VEN_PREFIX_SN="${VEN_PREFIX_SN:-picovensn}"
VEN_VM_MODE="${VEN_VM_MODE:-libvirt}"
BOOT_PXE_SERVER="${BOOT_PXE_SERVER:-false}"

# Set boot order based on PXE server setting and VM mode
if [ "$BOOT_PXE_SERVER" = "true" ]; then
    if [ "$VEN_VM_MODE" = "libvirt" ]; then
        export TF_VAR_boot_order='["hd","network"]'
    else
        # Proxmox mode
        export TF_VAR_boot_order='["scsi0","net0"]'
    fi
else
    unset TF_VAR_boot_order
fi

# Derived variables
MODULE_DIR="pico-vm-${VEN_VM_MODE}"
TINKERBELL_NGINX_DOMAIN="tinkerbell-nginx.${CLUSTER_FQDN}"

# Function to validate if argument is numeric
is_numeric() {
    if [[ $1 =~ ^[0-9]+$ ]]; then
        return 0
    else
        return 1
    fi
}

# Function to validate range format
is_range() {
    if [[ $1 =~ ^[0-9]+-[0-9]+$ ]]; then
        return 0
    else
        return 1
    fi
}

# Function to check if terraform is available
check_terraform() {
    if ! command -v terraform &> /dev/null; then
        echo "❌ Terraform is not installed or not in PATH" >&2
        return 1
    fi
    return 0
}

# Function to validate VM mode specific requirements
validate_vm_mode() {
    if [ "$VEN_VM_MODE" = "libvirt" ]; then
        # Check libvirt specific requirements
        if ! command -v virsh &> /dev/null; then
            echo "❌ virsh command not found. Libvirt tools are required for libvirt mode" >&2
            return 1
        fi
        
        # Validate libvirt specific variables
        local libvirt_vars=("LIBVIRT_POOL_NAME" "LIBVIRT_NETWORK_NAME" "LIBVIRT_VM_CONSOLE")
        for var in "${libvirt_vars[@]}"; do
            if [ -z "${!var:-}" ]; then
                echo "❌ Required libvirt variable $var is not set" >&2
                return 1
            fi
        done
        
    elif [ "$VEN_VM_MODE" = "proxmox" ]; then
        # Validate proxmox specific variables
        local proxmox_vars=("PROXMOX_ENDPOINT" "PROXMOX_USERNAME" "PROXMOX_PASSWORD" "PROXMOX_NODE_NAME" "PROXMOX_NETWORK_BRIDGE")
        for var in "${proxmox_vars[@]}"; do
            if [ -z "${!var:-}" ]; then
                echo "❌ Required proxmox variable $var is not set" >&2
                return 1
            fi
        done
    else
        echo "❌ Invalid VEN_VM_MODE: $VEN_VM_MODE. Must be 'libvirt' or 'proxmox'" >&2
        return 1
    fi
    
    return 0
}

# Function to validate serial number format
validate_serial() {
    local serial="$1"
    if [ -z "$serial" ]; then
        echo "❌ Serial number cannot be empty" >&2
        return 1
    fi
    
    # Check for invalid characters
    if [[ ! "$serial" =~ ^[a-zA-Z0-9_-]+$ ]]; then
        echo "❌ Invalid serial number format: $serial. Only alphanumeric, underscore, and dash allowed" >&2
        return 1
    fi
    
    return 0
}

# Function to cleanup on exit
cleanup() {
    local exit_code=$?
    if [ $exit_code -ne 0 ]; then
        echo "⚠️  Script exited with error code $exit_code. Some operations may be incomplete." >&2
    fi
}

trap cleanup EXIT

# Function to create a single VM
create_single_vm() {
    local VM_SERIAL="$1"
    
    if ! validate_serial "$VM_SERIAL"; then
        return 1
    fi
    
    (
        local VM_DIR="${VEN_PICO_PATH}/${VM_SERIAL}"

        echo "📁 Creating and copying files to $VM_DIR..."
        
        if ! mkdir -p "$VM_DIR"; then
            echo "❌ Failed to create directory: $VM_DIR" >&2
            return 1
        fi
        
        local MODULE_PATH="${VEN_PICO_PATH}/${MODULE_DIR}"
        if [ ! -d "$MODULE_PATH" ]; then
            echo "❌ Module directory not found: $MODULE_PATH" >&2
            return 1
        fi
        
        if ! cp -rf "$MODULE_PATH"/* "$VM_DIR/"; then
            echo "❌ Failed to copy module files to $VM_DIR" >&2
            return 1
        fi

        echo "⚙️ Running Terraform CREATE for $VM_SERIAL using ${VEN_VM_MODE} mode..."
        
        if ! cd "$VM_DIR"; then
            echo "❌ Failed to change to directory: $VM_DIR" >&2
            return 1
        fi
        
        if ! check_terraform; then
            return 1
        fi
        
        if ! terraform init -input=false; then
            echo "❌ Terraform init failed for $VM_SERIAL" >&2
            return 1
        fi
        
        local terraform_cmd_result=0
        
        if [ "$VEN_VM_MODE" = "libvirt" ]; then
            terraform apply \
                -var "vm_name=${VM_SERIAL}" \
                -var "smbios_serial=${VM_SERIAL}" \
                -var "tinkerbell_nginx_domain=${TINKERBELL_NGINX_DOMAIN}" \
                -var "libvirt_pool_name=${LIBVIRT_POOL_NAME}" \
                -var "libvirt_network_name=${LIBVIRT_NETWORK_NAME}" \
                -var "vm_console=${LIBVIRT_VM_CONSOLE}" \
                -var "disk_size=${VEN_DISK_SIZE}" \
                -var "memory=${VEN_MEMORY}" \
                -var "cpu_cores=${VEN_CPU_CORES}" \
                -auto-approve || terraform_cmd_result=$?
        else
            # Proxmox mode
            terraform apply \
                -var "vm_name=${VM_SERIAL}" \
                -var "smbios_serial=${VM_SERIAL}" \
                -var "tinkerbell_nginx_domain=${TINKERBELL_NGINX_DOMAIN}" \
                -var "proxmox_endpoint=${PROXMOX_ENDPOINT}" \
                -var "proxmox_username=${PROXMOX_USERNAME}" \
                -var "proxmox_password=${PROXMOX_PASSWORD}" \
                -var "proxmox_node_name=${PROXMOX_NODE_NAME}" \
                -var "network_bridge=${PROXMOX_NETWORK_BRIDGE}" \
                -var "memory_dedicated=${VEN_MEMORY}" \
                -var "memory_minimum=${VEN_MEMORY}" \
                -var "disk_size=${VEN_DISK_SIZE}" \
                -var "cpu_cores=${VEN_CPU_CORES}" \
                -var "proxmox_insecure=true" \
                -auto-approve || terraform_cmd_result=$?
        fi
        
        # Check if terraform command was successful
        if [ $terraform_cmd_result -eq 0 ]; then
            echo "✅ VM $VM_SERIAL created successfully"
        else
            echo "❌ Failed to create VM $VM_SERIAL" >&2
            return 1
        fi
        
    ) &
    sleep 10
}

# Create function
create() {
    echo "🚀 Creating VMs..."

    if ! validate_vm_mode; then
        exit 1
    fi

    if [ $# -lt 2 ]; then
        echo "❌ Error: No VM names or range specified!" >&2
        echo "Usage: $0 create [serial_numbers...] or [start-end]" >&2
        exit 1
    fi
    
    local failed_vms=()
    
    if is_range "$2"; then
        local RANGE="$2"
        local START END
        START=$(echo "$RANGE" | cut -d'-' -f1)
        END=$(echo "$RANGE" | cut -d'-' -f2)
        
        if ! is_numeric "$START" || ! is_numeric "$END"; then
            echo "❌ Invalid range format. START and END must be numeric" >&2
            exit 1
        fi
        
        if [ "$START" -gt "$END" ]; then
            echo "❌ Invalid range: START ($START) cannot be greater than END ($END)" >&2
            exit 1
        fi
        
        echo "Creating VMs from range: ${VEN_PREFIX_SN}${START} to ${VEN_PREFIX_SN}${END}"
        
        for I in $(seq "$START" "$END"); do
            local VM_SERIAL="${VEN_PREFIX_SN}${I}"
            if ! create_single_vm "$VM_SERIAL"; then
                failed_vms+=("$VM_SERIAL")
            fi
        done
    else
        echo "Creating VMs with provided serial numbers..."
        for I in $(seq 2 $#); do
            local VM_SERIAL=${!I}
            if ! create_single_vm "$VM_SERIAL"; then
                failed_vms+=("$VM_SERIAL")
            fi
        done
    fi

    wait
    
    if [ ${#failed_vms[@]} -eq 0 ]; then
        echo "✅ All VMs created successfully!"
    else
        echo "⚠️  Some VMs failed to create: ${failed_vms[*]}" >&2
        exit 1
    fi
}

# Function to ask for confirmation
confirm_action() {
    local action="$1"
    local targets="$2"
    
    echo "⚠️  WARNING: You are about to $action the following VMs:"
    echo "   $targets"
    echo ""
    read -p "Are you sure you want to proceed? (yes/no): " confirmation
    
    case "$confirmation" in
        yes|YES|y|Y)
            return 0
            ;;
        *)
            echo "Operation cancelled."
            exit 0
            ;;
    esac
}

# Function to destroy a single VM
destroy_single_vm() {
    local VM_SERIAL="$1"
    local VM_DIR="${VEN_PICO_PATH}/${VM_SERIAL}"

    if ! validate_serial "$VM_SERIAL"; then
        return 1
    fi

    if [[ -d "$VM_DIR" ]]; then
        echo "🧨 Terraform DESTROY for $VM_SERIAL..."
        
        if ! cd "$VM_DIR"; then
            echo "❌ Failed to change to directory: $VM_DIR" >&2
            return 1
        fi
        
        if ! check_terraform; then
            return 1
        fi
        
        local terraform_cmd_result=0
        
        if [ "$VEN_VM_MODE" = "libvirt" ]; then
            terraform destroy \
                -var "vm_name=${VM_SERIAL}" \
                -var "smbios_serial=${VM_SERIAL}" \
                -var "tinkerbell_nginx_domain=${TINKERBELL_NGINX_DOMAIN}" \
                -var "libvirt_pool_name=${LIBVIRT_POOL_NAME}" \
                -var "libvirt_network_name=${LIBVIRT_NETWORK_NAME}" \
                -var "vm_console=${LIBVIRT_VM_CONSOLE}" \
                -var "disk_size=${VEN_DISK_SIZE}" \
                -var "memory=${VEN_MEMORY}" \
                -var "cpu_cores=${VEN_CPU_CORES}" \
                -auto-approve || terraform_cmd_result=$?
        else
            terraform destroy \
                -var "vm_name=${VM_SERIAL}" \
                -var "smbios_serial=${VM_SERIAL}" \
                -var "tinkerbell_nginx_domain=${TINKERBELL_NGINX_DOMAIN}" \
                -var "proxmox_endpoint=${PROXMOX_ENDPOINT}" \
                -var "proxmox_username=${PROXMOX_USERNAME}" \
                -var "proxmox_password=${PROXMOX_PASSWORD}" \
                -var "proxmox_node_name=${PROXMOX_NODE_NAME}" \
                -var "network_bridge=${PROXMOX_NETWORK_BRIDGE}" \
                -var "disk_size=${VEN_DISK_SIZE}" \
                -var "memory_dedicated=${VEN_MEMORY}" \
                -var "memory_minimum=${VEN_MEMORY}" \
                -var "cpu_cores=${VEN_CPU_CORES}" \
                -auto-approve || terraform_cmd_result=$?
        fi
        
        if [ $terraform_cmd_result -eq 0 ]; then
            echo "✅ VM $VM_SERIAL destroyed successfully"
        else
            echo "❌ Failed to destroy VM $VM_SERIAL" >&2
            return 1
        fi
        
        cd - > /dev/null || true
    else
        echo "⚠️ Directory $VM_DIR not found, skipping..."
        return 1
    fi
}

# Destroy function
destroy() {
    echo "💣 Destroying VMs..."

    if ! validate_vm_mode; then
        exit 1
    fi

    if [ $# -lt 2 ]; then
        echo "❌ Error: No VM names or range specified!" >&2
        echo "Usage: $0 destroy [serial_numbers...] or [start-end]" >&2
        exit 1
    fi

    # Prepare confirmation message
    if is_range "$2"; then
        # Range format: START-END (use VEN_PREFIX_SN)
        RANGE="$2"
        START=$(echo "$RANGE" | cut -d'-' -f1)
        END=$(echo "$RANGE" | cut -d'-' -f2)
        TARGETS="VMs from range: ${VEN_PREFIX_SN}${START} to ${VEN_PREFIX_SN}${END}"
    else
        # Multiple individual serial numbers provided as arguments
        TARGETS="VMs: "
        for I in $(seq 2 $#); do
            VM_SERIAL=${!I}
            TARGETS="$TARGETS $VM_SERIAL"
        done
    fi

    # Ask for confirmation
    confirm_action "DESTROY" "$TARGETS"

    # Proceed with destruction
    local failed_vms=()
    
    if is_range "$2"; then
        local RANGE="$2"
        local START END
        START=$(echo "$RANGE" | cut -d'-' -f1)
        END=$(echo "$RANGE" | cut -d'-' -f2)
        
        if ! is_numeric "$START" || ! is_numeric "$END"; then
            echo "❌ Invalid range format. START and END must be numeric" >&2
            exit 1
        fi
        
        echo "Destroying VMs from range: ${VEN_PREFIX_SN}${START} to ${VEN_PREFIX_SN}${END}"
        
        for I in $(seq "$START" "$END"); do
            if ! destroy_single_vm "${VEN_PREFIX_SN}${I}"; then
                failed_vms+=("${VEN_PREFIX_SN}${I}")
            fi
        done
    else
        echo "Destroying VMs with provided serial numbers..."
        for I in $(seq 2 $#); do
            local VM_SERIAL=${!I}
            if ! destroy_single_vm "$VM_SERIAL"; then
                failed_vms+=("$VM_SERIAL")
            fi
        done
    fi

    if [ -d "${VEN_PICO_PATH}/common/mnt" ]; then
        echo "🔧 Unmounting common directory..."
        if ! sudo umount "${VEN_PICO_PATH}/common/mnt"; then
            echo "⚠️ Failed to unmount common directory, but continuing..."
        fi
    fi
    
    if [ ${#failed_vms[@]} -eq 0 ]; then
        echo "✅ All VMs destroyed successfully!"
    else
        echo "⚠️  Some VMs failed to destroy: ${failed_vms[*]}" >&2
        exit 1
    fi
}

# List function
list() {
    echo "📋 Listing all VMs..."
    # List all VMs that match our naming patterns
    echo "Active VMs:"
    virsh list --all
    
}

# Argument handler
case "$1" in
    create)
        create "$@"
        ;;
    destroy)
        destroy "$@"
        ;;
    list)
        list
        ;;
    *)
        echo "❌ Invalid usage."
        echo "Usage:"
        echo "  $0 create [serial_numbers...]            - Create VMs with serial numbers (vm_name = serial_no)"
        echo "  $0 create [start-end]                    - Create VMs in range (uses ${VEN_PREFIX_SN} prefix)"
        echo "  $0 destroy [serial_numbers...]           - Destroy VMs with serial numbers"
        echo "  $0 destroy [start-end]                   - Destroy VMs in range (uses ${VEN_PREFIX_SN} prefix)"
        echo "  $0 list                                  - List all VMs"
        echo ""
        echo "Examples:"
        echo "  $0 create SN001 SN002 SN003              - Create VMs with serial numbers SN001, SN002, SN003"
        echo "  $0 create 1-5                           - Create ${VEN_PREFIX_SN}1 to ${VEN_PREFIX_SN}5"
        echo "  VEN_PREFIX_SN=CUSTOM $0 create 1-3       - Create CUSTOM1 to CUSTOM3"
        echo "  VEN_VM_MODE=proxmox $0 create 1-3        - Create VMs using proxmox mode"
        echo ""
        echo "Environment Variables:"
        echo "  VEN_PREFIX_SN - Prefix for range operations (default: picovensn)"
        echo "  VEN_VM_MODE   - VM mode: libvirt or proxmox (default: libvirt)"
        echo "  VEN_DISK_SIZE - Disk size in GB (default: 110)"
        echo "  VEN_MEMORY    - Memory in MB (default: 8192)"
        echo "  VEN_CPU_CORES - Number of CPU cores (default: 4)"
        exit 1
        ;;
esac

