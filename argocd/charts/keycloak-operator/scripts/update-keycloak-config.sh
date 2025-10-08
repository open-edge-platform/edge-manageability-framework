#!/bin/bash

# Helper script to update Keycloak configuration
# This script can be used to easily manage Keycloak realm and client configurations

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="${SCRIPT_DIR}/../configs/keycloak-operator.yaml"

show_help() {
    cat << EOF
Keycloak Configuration Helper

Usage: $0 [COMMAND] [OPTIONS]

Commands:
    validate        Validate the current configuration syntax
    add-realm       Add a new realm configuration
    add-client      Add a new client to an existing realm
    list-realms     List all configured realms
    show-config     Show current configuration
    
Options:
    -h, --help      Show this help message

Examples:
    # Validate current configuration
    $0 validate
    
    # Show current configuration
    $0 show-config
    
    # List all realms
    $0 list-realms

Configuration Location: $CONFIG_FILE

The configuration is stored in YAML format under:
  keycloak.configCli.configuration

Each realm should be a separate JSON file entry:
  realm-<name>.json: |
    {
      "realm": "<name>",
      ...
    }
EOF
}

validate_config() {
    echo "Validating Keycloak configuration..."
    
    if [ ! -f "$CONFIG_FILE" ]; then
        echo "Error: Configuration file not found: $CONFIG_FILE"
        exit 1
    fi
    
    # Check YAML syntax
    if command -v yq >/dev/null 2>&1; then
        yq eval '.keycloak.configCli.configuration' "$CONFIG_FILE" >/dev/null
        echo "✓ YAML syntax is valid"
    else
        echo "Warning: yq not found, cannot validate YAML syntax"
    fi
    
    # Check JSON syntax in configuration files
    local temp_dir=$(mktemp -d)
    yq eval '.keycloak.configCli.configuration' "$CONFIG_FILE" -o=json > "$temp_dir/config.json"
    
    local error_count=0
    for key in $(jq -r 'keys[]' "$temp_dir/config.json"); do
        echo "Validating $key..."
        if ! jq '.' <<< "$(jq -r ".[\"$key\"]" "$temp_dir/config.json")" >/dev/null 2>&1; then
            echo "✗ Invalid JSON in $key"
            error_count=$((error_count + 1))
        else
            echo "✓ $key is valid JSON"
        fi
    done
    
    rm -rf "$temp_dir"
    
    if [ $error_count -eq 0 ]; then
        echo "✓ All configurations are valid"
    else
        echo "✗ Found $error_count configuration errors"
        exit 1
    fi
}

list_realms() {
    echo "Configured Keycloak Realms:"
    echo "=========================="
    
    if [ ! -f "$CONFIG_FILE" ]; then
        echo "Error: Configuration file not found: $CONFIG_FILE"
        exit 1
    fi
    
    local temp_dir=$(mktemp -d)
    yq eval '.keycloak.configCli.configuration' "$CONFIG_FILE" -o=json > "$temp_dir/config.json"
    
    for key in $(jq -r 'keys[]' "$temp_dir/config.json"); do
        local realm_name=$(jq -r ".[\"$key\"]" "$temp_dir/config.json" | jq -r '.realm // "unknown"')
        echo "  $key -> realm: $realm_name"
    done
    
    rm -rf "$temp_dir"
}

show_config() {
    echo "Current Keycloak Configuration:"
    echo "==============================="
    
    if [ ! -f "$CONFIG_FILE" ]; then
        echo "Error: Configuration file not found: $CONFIG_FILE"
        exit 1
    fi
    
    yq eval '.keycloak.configCli.configuration' "$CONFIG_FILE"
}

case "${1:-}" in
    validate)
        validate_config
        ;;
    list-realms)
        list_realms
        ;;
    show-config)
        show_config
        ;;
    -h|--help|help)
        show_help
        ;;
    *)
        echo "Error: Unknown command '${1:-}'"
        echo ""
        show_help
        exit 1
        ;;
esac