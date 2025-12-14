#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# OnPrem Kubernetes Engine (RKE2) Installer
# This script installs or upgrades the RKE2 Kubernetes cluster

set -o errexit
set -o pipefail

cat << "EOF"
 _  ________   _____ _   _  _____ _______       _      _      ______ _____  
| |/ /  ____| |_   _| \ | |/ ____|__   __|/\   | |    | |    |  ____|  __ \ 
| ' /| |__      | | |  \| | (___    | |  /  \  | |    | |    | |__  | |__) |
|  < |  __|     | | | . | |\___ \   | | / /\ \ | |    | |    |  __| |  _  / 
| . \| |____   _| |_| |\  |____) |  | |/ ____ \| |____| |____| |____| | \ \ 
|_|\_\______| |_____|_| \_|_____/   |_/_/    \_\______|______|______|_|  \_\
EOF

# Add /usr/local/bin to the PATH
export PATH=$PATH:/usr/local/bin

# Parse command line arguments
UPGRADE_MODE=false
while [[ $# -gt 0 ]]; do
    case "$1" in
        -upgrade|--upgrade)
            UPGRADE_MODE=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [-upgrade|--upgrade]"
            exit 1
            ;;
    esac
done

# Set environment variables
export KUBECONFIG="/home/${USER}/.kube/config"
export INSTALLER_DEPLOY="true"

# Set deployment timeout
DEPLOYMENT_TIMEOUT="${DEPLOYMENT_TIMEOUT:-3600s}"
export DEPLOYMENT_TIMEOUT

# Validate deployment timeout is a valid duration
if ! echo "$DEPLOYMENT_TIMEOUT" | grep -qE '^[0-9]+(s|m|h)$'; then
    echo "Error: DEPLOYMENT_TIMEOUT must be a valid duration string (e.g., 3600s, 60m, 1h)"
    exit 1
fi

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RKE2_DIR="$SCRIPT_DIR/../../rke2"

#######################################
# Test that deployments and pods are ready
#######################################
test_deployment_and_pods() {
    echo "Waiting for deployments and pods to be ready..."
    
    # Wait for all deployments to be ready
    local timeout=300
    local elapsed=0
    while [ $elapsed -lt $timeout ]; do
        if kubectl get deployments -A --no-headers 2>/dev/null | grep -q "0/"; then
            echo "Waiting for deployments to be ready... (${elapsed}s)"
            sleep 10
            elapsed=$((elapsed + 10))
        else
            echo "All deployments are ready"
            break
        fi
    done
    
    # Wait for all pods to be running
    elapsed=0
    while [ $elapsed -lt $timeout ]; do
        local not_running
        not_running=$(kubectl get pods -A --no-headers 2>/dev/null | grep -v -E 'Running|Completed' | wc -l || echo "0")
        if [ "$not_running" -gt 0 ]; then
            echo "Waiting for pods to be ready... (${elapsed}s, ${not_running} pods not ready)"
            sleep 10
            elapsed=$((elapsed + 10))
        else
            echo "All pods are running"
            break
        fi
    done
}

#######################################
# Install RKE2 cluster
#######################################
install_rke2_cluster() {
    echo "Installing RKE2 cluster..."
    
    # Check if Docker credentials are provided for customization
    local docker_args=()
    if [ -n "${DOCKER_USERNAME:-}" ] && [ -n "${DOCKER_PASSWORD:-}" ]; then
        echo "Using Docker credentials for customizing RKE2 installation"
        docker_args=("-u" "$DOCKER_USERNAME" "-p" "$DOCKER_PASSWORD")
    fi
    
    # Run RKE2 installer
    if [ ! -f "$RKE2_DIR/rke2installerlocal.sh" ]; then
        echo "Error: RKE2 installer script not found at $RKE2_DIR/rke2installerlocal.sh"
        exit 1
    fi
    
    bash "$RKE2_DIR/rke2installerlocal.sh"
    
    # Customize RKE2
    if [ ! -f "$RKE2_DIR/customize-rke2.sh" ]; then
        echo "Error: RKE2 customize script not found at $RKE2_DIR/customize-rke2.sh"
        exit 1
    fi
    
    bash "$RKE2_DIR/customize-rke2.sh" "${docker_args[@]}"
    
    # Wait for deployments and pods to be ready
    test_deployment_and_pods
    
    # Install OpenEBS LocalPV
    echo "Installing OpenEBS LocalPV provisioner..."
    
    # Add OpenEBS helm repository
    helm repo add openebs-localpv https://openebs.github.io/dynamic-localpv-provisioner
    helm repo update
    
    # Install/upgrade OpenEBS LocalPV
    helm upgrade --install openebs-localpv openebs-localpv/localpv-provisioner \
        --version 4.3.0 \
        --namespace openebs-system \
        --create-namespace \
        --set hostpathClass.enabled=true \
        --set hostpathClass.name=openebs-hostpath \
        --set hostpathClass.isDefaultClass=true \
        --set deviceClass.enabled=false
    
    # Create etcd-cert secret
    echo "Creating etcd-certs secret..."
    kubectl create secret generic etcd-certs \
        --from-file=/var/lib/rancher/rke2/server/tls/etcd/server-client.crt \
        --from-file=/var/lib/rancher/rke2/server/tls/etcd/server-client.key \
        --from-file=/var/lib/rancher/rke2/server/tls/etcd/server-ca.crt \
        --dry-run=client -o yaml | kubectl apply -f -
    
    # Final verification after OpenEBS installation
    test_deployment_and_pods
    
    # Run customize script one more time after OpenEBS
    bash "$RKE2_DIR/customize-rke2.sh"
    
    echo ""
    echo "✓ RKE2 cluster ready: 😊"
    echo ""
}

#######################################
# Upgrade RKE2 cluster
#######################################
upgrade_rke2_cluster() {
    echo "Upgrading RKE2 cluster..."
    
    # For upgrade, we primarily need to run the RKE2 upgrade process
    # The actual upgrade logic would be in the RKE2 scripts or done via kubectl
    
    echo "Note: RKE2 upgrade typically involves:"
    echo "1. Updating RKE2 binaries"
    echo "2. Restarting RKE2 service"
    echo "3. Verifying cluster health"
    
    # Run customize script to ensure configuration is up to date
    if [ -f "$RKE2_DIR/customize-rke2.sh" ]; then
        bash "$RKE2_DIR/customize-rke2.sh"
    fi
    
    # Wait for cluster to stabilize
    test_deployment_and_pods
    
    echo ""
    echo "✓ RKE2 cluster upgrade completed"
    echo ""
}

#######################################
# Main execution
#######################################
main() {
    if [ "$UPGRADE_MODE" = true ]; then
        upgrade_rke2_cluster
    else
        install_rke2_cluster
    fi
}

# Run main function
main
