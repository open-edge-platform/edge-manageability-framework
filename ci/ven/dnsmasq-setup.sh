#!/bin/bash
# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0
set -x

# Check if at least two arguments are provided
if [ -z "$1" ] || [ -z "$2" ]; then
    echo "Usage: $0 <cluster.fqdn> {setup|config}"
    exit 1
fi

CLUSTER_FQDN="$1"
ACTION="$2"

# Get interface name with 10 network IP
interface_name=$(ip -o -4 addr show | awk '$4 ~ /^10\./ {print $2}')

# Check if any interfaces were found
if [ -n "$interface_name" ]; then
    echo "Interfaces with IP addresses starting with 10.:"
    echo "$interface_name"
else
    echo "No interfaces found with IP addresses starting with 10."
    ip -o -4 addr show
    exit 1
fi

# Get the IP address of the specified interface
ip_address=$(ip -4 addr show "$interface_name" | grep -oP '(?<=inet\s)\d+(\.\d+){3}')
if [ -z "$ip_address" ]; then
    echo "No IP address found for $interface_name. Exiting."
    exit 1
fi

function setup_dns() {
    sudo apt update -y
    resolvectl status
    dns_server_ip=$(resolvectl status | awk '/Current DNS Server/ {print $4}')
    sudo apt install -y dnsmasq
    sudo systemctl disable systemd-resolved
    sudo systemctl stop systemd-resolved

    # Backup the original dnsmasq configuration file
    echo "Backing up the original dnsmasq configuration..."
    sudo cp /etc/dnsmasq.conf /etc/dnsmasq.conf.bak

    # Get the current hostname
    current_hostname=$(hostname)
    echo "Adding hostname '$current_hostname' to /etc/hosts..."
    echo "$ip_address $current_hostname" | sudo tee -a /etc/hosts > /dev/null

    # Unlink and recreate /etc/resolv.conf
    echo "Configuring /etc/resolv.conf..."
    sudo unlink /etc/resolv.conf
    cat <<EOL | sudo tee /etc/resolv.conf
nameserver 127.0.0.1
options trust-ad
EOL

    # Configure dnsmasq
    echo "Configuring dnsmasq..."
    cat <<EOL | sudo tee /etc/dnsmasq.conf
interface=$interface_name
bind-interfaces
dhcp-option=interface:$interface_name,option:dns-server,$ip_address
server=$ip_address
server=$dns_server_ip
server=8.8.8.8
EOL
}

function update_host_lb_ip() {
    # Get LoadBalancer IPs from Kubernetes services
    argocd_lb=$(kubectl get svc -n argocd | grep LoadBalancer | awk '{print $4}')
    tinkerbell_lb=$(kubectl get svc -n orch-boots | grep LoadBalancer | awk '{print $4}')
    cluster_lb=$(kubectl get svc -n orch-gateway | grep LoadBalancer | awk '{print $4}')
    
    # Check if LoadBalancer IPs were found
    if [ -z "$argocd_lb" ] || [ -z "$tinkerbell_lb" ] || [ -z "$cluster_lb" ]; then
        echo "One or more LoadBalancer IPs could not be retrieved. Exiting."
        exit 1
    fi
    
    # Uncomment these lines if you want to use the host IP instead
    # argocd_lb=$ip_address
    # tinkerbell_lb=$ip_address
    # cluster_lb=$ip_address
    
    cat <<EOL | sudo tee /etc/dnsmasq.d/cluster-hosts-dns.conf
address=/tinkerbell-nginx.$CLUSTER_FQDN/$tinkerbell_lb
address=/argo.$CLUSTER_FQDN/$argocd_lb
address=/$CLUSTER_FQDN/$cluster_lb
address=/alerting-monitor.$CLUSTER_FQDN/$cluster_lb
address=/api.$CLUSTER_FQDN/$cluster_lb
address=/app-orch.$CLUSTER_FQDN/$cluster_lb
address=/app-service-proxy.$CLUSTER_FQDN/$cluster_lb
address=/cluster-orch-edge-node.$CLUSTER_FQDN/$cluster_lb
address=/cluster-orch-node.$CLUSTER_FQDN/$cluster_lb
address=/cluster-orch.$CLUSTER_FQDN/$cluster_lb
address=/connect-gateway.$CLUSTER_FQDN/$cluster_lb
address=/fleet.$CLUSTER_FQDN/$cluster_lb
address=/infra-node.$CLUSTER_FQDN/$cluster_lb
address=/infra.$CLUSTER_FQDN/$cluster_lb
address=/keycloak.$CLUSTER_FQDN/$cluster_lb
address=/license-node.$CLUSTER_FQDN/$cluster_lb
address=/log-query.$CLUSTER_FQDN/$cluster_lb
address=/logs-node.$CLUSTER_FQDN/$cluster_lb
address=/metadata.$CLUSTER_FQDN/$cluster_lb
address=/metrics-node.$CLUSTER_FQDN/$cluster_lb
address=/observability-admin.$CLUSTER_FQDN/$cluster_lb
address=/observability-ui.$CLUSTER_FQDN/$cluster_lb
address=/onboarding-node.$CLUSTER_FQDN/$cluster_lb
address=/onboarding-stream.$CLUSTER_FQDN/$cluster_lb
address=/onboarding.$CLUSTER_FQDN/$cluster_lb
address=/orchestrator-license.$CLUSTER_FQDN/$cluster_lb
address=/rancher.$CLUSTER_FQDN/$cluster_lb
address=/registry-oci.$CLUSTER_FQDN/$cluster_lb
address=/registry.$CLUSTER_FQDN/$cluster_lb
address=/release.$CLUSTER_FQDN/$cluster_lb
address=/telemetry-node.$CLUSTER_FQDN/$cluster_lb
address=/telemetry.$CLUSTER_FQDN/$cluster_lb
address=/tinkerbell-server.$CLUSTER_FQDN/$cluster_lb
address=/update-node.$CLUSTER_FQDN/$cluster_lb
address=/update.$CLUSTER_FQDN/$cluster_lb
address=/vault.$CLUSTER_FQDN/$cluster_lb
address=/vnc.$CLUSTER_FQDN/$cluster_lb
address=/web-ui.$CLUSTER_FQDN/$cluster_lb
address=/ws-app-service-proxy.$CLUSTER_FQDN/$cluster_lb
EOL
}

# Main execution logic
if [ "$ACTION" == "setup" ]; then
    setup_dns
    # update_host_lb_ip  # Uncomment if you want to run this during setup
    sudo systemctl restart dnsmasq
    sudo systemctl enable dnsmasq
    cat /etc/resolv.conf
    cat /etc/dnsmasq.conf

elif [ "$ACTION" == "config" ]; then
    update_host_lb_ip
    sudo systemctl restart dnsmasq
    sudo systemctl enable dnsmasq
    echo "DNS config updated"
    sudo cat /etc/dnsmasq.d/cluster-hosts-dns.conf
else
    echo "Invalid action: $ACTION"
    echo "Usage: $0 <cluster.fqdn> {setup|config}"
    exit 1
fi
